package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/config"
)

type memoryEntry struct {
	Key        string
	Content    string
	Category   string
	Score      float64
	SessionKey string
	Channel    string
	Sender     string
	MessageID  string
	CreatedAt  string
	UpdatedAt  string
}

type sqliteMemoryStore struct {
	dbPath             string
	vectorWeight       float64
	keywordWeight      float64
	embeddingCacheSize int
	embedder           embeddingProvider
	mu                 sync.Mutex
}

func newSQLiteMemoryStore(cfg *config.Config) *sqliteMemoryStore {
	base := ""
	if cfg != nil {
		base = strings.TrimSpace(cfg.Agent.Workspace)
	}
	if base == "" {
		base = filepath.Join(os.TempDir(), "highclaw-workspace")
	}
	vectorWeight := 0.7
	keywordWeight := 0.3
	cacheSize := 10000
	if cfg != nil {
		if cfg.Memory.VectorWeight > 0 {
			vectorWeight = cfg.Memory.VectorWeight
		}
		if cfg.Memory.KeywordWeight > 0 {
			keywordWeight = cfg.Memory.KeywordWeight
		}
		if cfg.Memory.EmbeddingCacheSize > 0 {
			cacheSize = cfg.Memory.EmbeddingCacheSize
		}
	}
	return &sqliteMemoryStore{
		dbPath:             filepath.Join(base, "memory", "brain.db"),
		vectorWeight:       vectorWeight,
		keywordWeight:      keywordWeight,
		embeddingCacheSize: cacheSize,
		embedder:           createEmbeddingProvider(cfg),
	}
}

func (s *sqliteMemoryStore) location() string {
	return s.dbPath
}

func (s *sqliteMemoryStore) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		s.dbPath = filepath.Join(os.TempDir(), "highclaw-memory.db")
		if err2 := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err2 != nil {
			return fmt.Errorf("create memory state dir: %w", err)
		}
	}

	ddl := `
CREATE TABLE IF NOT EXISTS memory_entries (
  key TEXT PRIMARY KEY,
  content TEXT NOT NULL,
  category TEXT NOT NULL DEFAULT 'core',
  embedding BLOB,
  created_at TEXT NOT NULL DEFAULT '',
  session_key TEXT NOT NULL DEFAULT '',
  channel TEXT NOT NULL DEFAULT '',
  sender TEXT NOT NULL DEFAULT '',
  message_id TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS embedding_cache (
  content_hash TEXT PRIMARY KEY,
  embedding BLOB NOT NULL,
  created_at TEXT NOT NULL,
  accessed_at TEXT NOT NULL
);`
	_, err := s.exec(ddl)
	if err == nil {
		_, _ = s.exec("ALTER TABLE memory_entries ADD COLUMN category TEXT NOT NULL DEFAULT 'core';")
		_, _ = s.exec("ALTER TABLE memory_entries ADD COLUMN embedding BLOB;")
		_, _ = s.exec("ALTER TABLE memory_entries ADD COLUMN created_at TEXT NOT NULL DEFAULT '';")
		_, _ = s.exec("ALTER TABLE memory_entries ADD COLUMN session_key TEXT NOT NULL DEFAULT '';")
		_, _ = s.exec("ALTER TABLE memory_entries ADD COLUMN channel TEXT NOT NULL DEFAULT '';")
		_, _ = s.exec("ALTER TABLE memory_entries ADD COLUMN sender TEXT NOT NULL DEFAULT '';")
		_, _ = s.exec("ALTER TABLE memory_entries ADD COLUMN message_id TEXT NOT NULL DEFAULT '';")
		_, _ = s.exec("CREATE INDEX IF NOT EXISTS idx_memory_entries_updated_at ON memory_entries(updated_at DESC);")
		_, _ = s.exec("CREATE INDEX IF NOT EXISTS idx_memory_entries_category ON memory_entries(category);")
		_, _ = s.exec("CREATE INDEX IF NOT EXISTS idx_memory_entries_session_key ON memory_entries(session_key);")
		_, _ = s.exec("CREATE INDEX IF NOT EXISTS idx_memory_entries_channel_sender ON memory_entries(channel, sender);")
		_, _ = s.exec("CREATE INDEX IF NOT EXISTS idx_embedding_cache_accessed ON embedding_cache(accessed_at);")
		_, _ = s.exec("UPDATE memory_entries SET created_at = updated_at WHERE created_at = '';")
		s.ensureFTSContentMode()
	}
	return err
}

// ensureFTSContentMode 将 FTS5 迁移为 content= 关联表模式并创建自动同步 trigger
func (s *sqliteMemoryStore) ensureFTSContentMode() {
	out, _ := s.execTabs("SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name='memory_entries_fts_ai';")
	if strings.TrimSpace(out) == "1" {
		return
	}
	_, _ = s.exec("DROP TABLE IF EXISTS memory_entries_fts;")
	_, _ = s.exec("CREATE VIRTUAL TABLE memory_entries_fts USING fts5(key, content, content=memory_entries, content_rowid=rowid);")
	_, _ = s.exec("INSERT INTO memory_entries_fts(memory_entries_fts) VALUES('rebuild');")
	_, _ = s.exec("CREATE TRIGGER memory_entries_fts_ai AFTER INSERT ON memory_entries BEGIN INSERT INTO memory_entries_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content); END;")
	_, _ = s.exec("CREATE TRIGGER memory_entries_fts_ad AFTER DELETE ON memory_entries BEGIN INSERT INTO memory_entries_fts(memory_entries_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content); END;")
	_, _ = s.exec("CREATE TRIGGER memory_entries_fts_au AFTER UPDATE ON memory_entries BEGIN INSERT INTO memory_entries_fts(memory_entries_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content); INSERT INTO memory_entries_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content); END;")
}

func (s *sqliteMemoryStore) store(key, content, category string, meta memoryMeta) error {
	emb, _ := s.getOrComputeEmbedding(content)

	s.mu.Lock()
	defer s.mu.Unlock()
	category = strings.TrimSpace(category)
	if category == "" {
		category = "core"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	sql := fmt.Sprintf(
		"INSERT INTO memory_entries(key, content, category, embedding, created_at, session_key, channel, sender, message_id, updated_at) VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s, %s) "+
			"ON CONFLICT(key) DO UPDATE SET content=excluded.content, category=excluded.category, embedding=excluded.embedding, session_key=excluded.session_key, channel=excluded.channel, sender=excluded.sender, message_id=excluded.message_id, updated_at=excluded.updated_at;",
		sqlQuote(key), sqlQuote(content), sqlQuote(category),
		sqlBlobOrNull(emb),
		sqlQuote(now),
		sqlQuote(strings.TrimSpace(meta.SessionKey)),
		sqlQuote(strings.TrimSpace(meta.Channel)),
		sqlQuote(strings.TrimSpace(meta.Sender)),
		sqlQuote(strings.TrimSpace(meta.MessageID)),
		sqlQuote(now),
	)
	_, err := s.exec(sql)
	return err
}

func (s *sqliteMemoryStore) forget(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.execTabs(fmt.Sprintf("SELECT COUNT(*) FROM memory_entries WHERE key=%s;", sqlQuote(key)))
	if err != nil {
		return false, err
	}
	existed := strings.TrimSpace(out) == "1"
	_, err = s.exec(fmt.Sprintf("DELETE FROM memory_entries WHERE key=%s;", sqlQuote(key)))
	if err != nil {
		return false, err
	}
	return existed, nil
}

func (s *sqliteMemoryStore) get(key string) (*memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.execTabs(fmt.Sprintf(
		"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE key=%s ORDER BY updated_at DESC LIMIT 1;",
		sqlQuote(strings.TrimSpace(key)),
	))
	if err != nil {
		return nil, err
	}
	entries := parseMemoryEntries(out)
	if len(entries) == 0 {
		return nil, nil
	}
	entry := entries[0]
	return &entry, nil
}

func (s *sqliteMemoryStore) list(category string) ([]memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	category = strings.ToLower(strings.TrimSpace(category))
	sql := "SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries ORDER BY updated_at DESC;"
	if category != "" {
		sql = fmt.Sprintf(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE category=%s ORDER BY updated_at DESC;",
			sqlQuote(category),
		)
	}
	out, err := s.execTabs(sql)
	if err != nil {
		return nil, err
	}
	return parseMemoryEntries(out), nil
}

func (s *sqliteMemoryStore) count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := s.execTabs("SELECT COUNT(*) FROM memory_entries;")
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, nil
	}
	return v, nil
}

func (s *sqliteMemoryStore) healthCheck() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.execTabs("SELECT 1;"); err != nil {
		return false
	}
	return true
}

func (s *sqliteMemoryStore) recall(query, key, sessionKey string, limit int) ([]memoryEntry, error) {
	queryEmbedding, _ := s.getOrComputeEmbedding(strings.TrimSpace(query))

	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		return []memoryEntry{}, nil
	}
	if strings.TrimSpace(key) == "" && strings.TrimSpace(query) == "" {
		return []memoryEntry{}, nil
	}

	var sql string
	sessionFilter := strings.TrimSpace(sessionKey)
	switch {
	case strings.TrimSpace(key) != "":
		if sessionFilter != "" {
			sql = fmt.Sprintf(
				"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE key=%s AND session_key=%s ORDER BY updated_at DESC LIMIT %d;",
				sqlQuote(key), sqlQuote(sessionFilter), limit,
			)
		} else {
			sql = fmt.Sprintf(
				"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE key=%s ORDER BY updated_at DESC LIMIT %d;",
				sqlQuote(key), limit,
			)
		}
	case strings.TrimSpace(query) != "":
		ftsExpr := ftsQuery(strings.TrimSpace(query))
		if sessionFilter != "" {
			sql = fmt.Sprintf(
				"SELECT m.key, m.content, m.category, m.session_key, m.channel, m.sender, m.message_id, m.created_at, m.updated_at, bm25(memory_entries_fts) "+
					"FROM memory_entries_fts JOIN memory_entries m ON m.rowid = memory_entries_fts.rowid "+
					"WHERE memory_entries_fts MATCH %s AND m.session_key=%s ORDER BY bm25(memory_entries_fts) ASC, m.updated_at DESC LIMIT %d;",
				sqlQuote(ftsExpr), sqlQuote(sessionFilter), limit,
			)
		} else {
			sql = fmt.Sprintf(
				"SELECT m.key, m.content, m.category, m.session_key, m.channel, m.sender, m.message_id, m.created_at, m.updated_at, bm25(memory_entries_fts) "+
					"FROM memory_entries_fts JOIN memory_entries m ON m.rowid = memory_entries_fts.rowid "+
					"WHERE memory_entries_fts MATCH %s ORDER BY bm25(memory_entries_fts) ASC, m.updated_at DESC LIMIT %d;",
				sqlQuote(ftsExpr), limit,
			)
		}
	default:
		if sessionFilter != "" {
			sql = fmt.Sprintf(
				"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE session_key=%s ORDER BY updated_at DESC LIMIT %d;",
				sqlQuote(sessionFilter), limit,
			)
		} else {
			sql = fmt.Sprintf(
				"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries ORDER BY updated_at DESC LIMIT %d;",
				limit,
			)
		}
	}

	out, err := s.execTabs(sql)
	if err != nil && strings.TrimSpace(query) != "" {
		out, err = s.execTabs(buildLikeRecallSQL(query, sessionFilter, limit))
	}
	if err != nil {
		return nil, err
	}
	keywordEntries := parseMemoryEntries(out)
	if len(keywordEntries) == 0 && strings.TrimSpace(query) != "" && strings.TrimSpace(key) == "" {
		likeOut, likeErr := s.execTabs(buildLikeRecallSQL(query, sessionFilter, limit))
		if likeErr == nil {
			keywordEntries = parseMemoryEntries(likeOut)
		}
	}
	keywordEntries = normalizeBM25Scores(keywordEntries)
	if strings.TrimSpace(query) == "" {
		return keywordEntries, nil
	}

	if len(queryEmbedding) == 0 {
		return keywordEntries, nil
	}
	vectorEntries, _ := s.vectorSearch(queryEmbedding, sessionFilter, limit*2)
	if len(vectorEntries) == 0 {
		return keywordEntries, nil
	}
	return s.hybridMerge(keywordEntries, vectorEntries, limit), nil
}

const sqlitePragmaPrefix = "PRAGMA trusted_schema = ON;\n"

func (s *sqliteMemoryStore) exec(sql string) (string, error) {
	cmd := exec.Command("sqlite3", s.dbPath, sqlitePragmaPrefix+sql)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("sqlite exec failed: %s", msg)
	}
	return stdout.String(), nil
}

func (s *sqliteMemoryStore) execTabs(sql string) (string, error) {
	cmd := exec.Command("sqlite3", "-tabs", "-noheader", s.dbPath, sqlitePragmaPrefix+sql)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("sqlite query failed: %s", msg)
	}
	return stdout.String(), nil
}

func sqlQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func sqlBlobOrNull(v []byte) string {
	if len(v) == 0 {
		return "NULL"
	}
	return "X'" + strings.ToUpper(hex.EncodeToString(v)) + "'"
}

func ftsQuery(query string) string {
	words := strings.Fields(strings.TrimSpace(query))
	if len(words) == 0 {
		return `""`
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		w = strings.ReplaceAll(w, `"`, `""`)
		out = append(out, `"`+w+`"`)
	}
	if len(out) == 0 {
		return `""`
	}
	return strings.Join(out, " OR ")
}

func buildLikeRecallSQL(query, sessionFilter string, limit int) string {
	q := "%" + strings.TrimSpace(query) + "%"
	if sessionFilter != "" {
		return fmt.Sprintf(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries "+
				"WHERE session_key=%s AND (key LIKE %s OR content LIKE %s) ORDER BY updated_at DESC LIMIT %d;",
			sqlQuote(sessionFilter), sqlQuote(q), sqlQuote(q), limit,
		)
	}
	return fmt.Sprintf(
		"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries "+
			"WHERE key LIKE %s OR content LIKE %s ORDER BY updated_at DESC LIMIT %d;",
		sqlQuote(q), sqlQuote(q), limit,
	)
}

func parseMemoryEntries(out string) []memoryEntry {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	entries := make([]memoryEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		entry := memoryEntry{Key: parts[0], Content: parts[1]}
		if len(parts) >= 3 {
			entry.Category = parts[2]
		}
		if len(parts) >= 4 {
			entry.SessionKey = parts[3]
		}
		if len(parts) >= 5 {
			entry.Channel = parts[4]
		}
		if len(parts) >= 6 {
			entry.Sender = parts[5]
		}
		if len(parts) >= 7 {
			entry.MessageID = parts[6]
		}
		if len(parts) >= 8 {
			entry.CreatedAt = parts[7]
		}
		if len(parts) >= 9 {
			entry.UpdatedAt = parts[8]
		}
		if len(parts) >= 10 {
			if score, e := strconv.ParseFloat(parts[9], 64); e == nil {
				entry.Score = math.Abs(score)
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

// normalizeBM25Scores 将 BM25 原始绝对值归一化到 [0,1]，最高分映射为 1.0
func normalizeBM25Scores(entries []memoryEntry) []memoryEntry {
	if len(entries) == 0 {
		return entries
	}
	maxScore := 0.0
	for _, e := range entries {
		if e.Score > maxScore {
			maxScore = e.Score
		}
	}
	if maxScore <= 0 {
		return entries
	}
	for i := range entries {
		entries[i].Score = entries[i].Score / maxScore
	}
	return entries
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x := float64(a[i])
		y := float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	s := dot / (math.Sqrt(na) * math.Sqrt(nb))
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return s
}

func vecToBytes(v []float32) []byte {
	if len(v) == 0 {
		return nil
	}
	out := make([]byte, 0, len(v)*4)
	for _, f := range v {
		bits := math.Float32bits(f)
		out = append(out, byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24))
	}
	return out
}

func bytesToVec(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	out := make([]float32, 0, len(b)/4)
	for i := 0; i+3 < len(b); i += 4 {
		bits := uint32(b[i]) | uint32(b[i+1])<<8 | uint32(b[i+2])<<16 | uint32(b[i+3])<<24
		out = append(out, math.Float32frombits(bits))
	}
	return out
}

func (s *sqliteMemoryStore) contentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func (s *sqliteMemoryStore) getOrComputeEmbedding(text string) ([]byte, error) {
	if s.embedder == nil || s.embedder.name() == "none" {
		return nil, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	hash := s.contentHash(text)
	row, err := s.execTabs(fmt.Sprintf("SELECT hex(embedding) FROM embedding_cache WHERE content_hash=%s LIMIT 1;", sqlQuote(hash)))
	if err == nil {
		hexBlob := strings.TrimSpace(row)
		if hexBlob != "" {
			_, _ = s.exec(fmt.Sprintf("UPDATE embedding_cache SET accessed_at=%s WHERE content_hash=%s;", sqlQuote(time.Now().UTC().Format(time.RFC3339Nano)), sqlQuote(hash)))
			if b, decErr := hex.DecodeString(hexBlob); decErr == nil {
				return b, nil
			}
		}
	}

	vec, err := s.embedder.embedOne(text)
	if err != nil || len(vec) == 0 {
		return nil, err
	}
	b := vecToBytes(vec)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = s.exec(fmt.Sprintf(
		"INSERT OR REPLACE INTO embedding_cache(content_hash, embedding, created_at, accessed_at) VALUES(%s, %s, %s, %s);",
		sqlQuote(hash), sqlBlobOrNull(b), sqlQuote(now), sqlQuote(now),
	))
	if s.embeddingCacheSize > 0 {
		_, _ = s.exec(fmt.Sprintf(
			"DELETE FROM embedding_cache WHERE content_hash IN (SELECT content_hash FROM embedding_cache ORDER BY accessed_at ASC LIMIT MAX(0, (SELECT COUNT(*) FROM embedding_cache)-%d));",
			s.embeddingCacheSize,
		))
	}
	return b, nil
}

func (s *sqliteMemoryStore) vectorSearch(queryEmbedding []byte, sessionFilter string, limit int) ([]memoryEntry, error) {
	if len(queryEmbedding) == 0 {
		return nil, nil
	}
	var sql string
	if sessionFilter != "" {
		sql = fmt.Sprintf("SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at, hex(embedding) FROM memory_entries WHERE embedding IS NOT NULL AND session_key=%s;", sqlQuote(sessionFilter))
	} else {
		sql = "SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at, hex(embedding) FROM memory_entries WHERE embedding IS NOT NULL;"
	}
	out, err := s.execTabs(sql)
	if err != nil {
		return nil, err
	}
	qv := bytesToVec(queryEmbedding)
	scored := make([]memoryEntry, 0)
	for _, row := range strings.Split(strings.TrimSpace(out), "\n") {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}
		parts := strings.Split(row, "\t")
		if len(parts) < 10 {
			continue
		}
		hexEmb := strings.TrimSpace(parts[9])
		if hexEmb == "" {
			continue
		}
		blob, decErr := hex.DecodeString(hexEmb)
		if decErr != nil {
			continue
		}
		score := cosineSimilarity(qv, bytesToVec(blob))
		if score <= 0 {
			continue
		}
		entry := memoryEntry{
			Key:        parts[0],
			Content:    parts[1],
			Category:   parts[2],
			SessionKey: parts[3],
			Channel:    parts[4],
			Sender:     parts[5],
			MessageID:  parts[6],
			CreatedAt:  parts[7],
			UpdatedAt:  parts[8],
			Score:      score,
		}
		scored = append(scored, entry)
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].UpdatedAt > scored[j].UpdatedAt
		}
		return scored[i].Score > scored[j].Score
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	if len(scored) == 0 {
		return nil, nil
	}
	return scored, nil
}

func (s *sqliteMemoryStore) hybridMerge(keywordEntries, vectorEntries []memoryEntry, limit int) []memoryEntry {
	type merged struct {
		entry        memoryEntry
		vectorScore  float64
		keywordScore float64
		finalScore   float64
	}
	m := map[string]*merged{}
	for _, e := range keywordEntries {
		ks := e.Score
		if ks == 0 {
			ks = 0.5
		}
		m[e.Key] = &merged{entry: e, keywordScore: ks}
	}
	for _, e := range vectorEntries {
		if existing, ok := m[e.Key]; ok {
			existing.vectorScore = e.Score
			continue
		}
		m[e.Key] = &merged{entry: e, vectorScore: e.Score}
	}
	items := make([]*merged, 0, len(m))
	for _, x := range m {
		x.finalScore = s.vectorWeight*x.vectorScore + s.keywordWeight*x.keywordScore
		if x.finalScore == 0 {
			x.finalScore = x.vectorScore
		}
		x.entry.Score = x.finalScore
		items = append(items, x)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].finalScore == items[j].finalScore {
			return items[i].entry.UpdatedAt > items[j].entry.UpdatedAt
		}
		return items[i].finalScore > items[j].finalScore
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]memoryEntry, 0, len(items))
	for _, it := range items {
		out = append(out, it.entry)
	}
	return out
}

// reindex 重建 FTS5 索引并为缺失 embedding 的条目批量补全向量
func (s *sqliteMemoryStore) reindex() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.exec("INSERT INTO memory_entries_fts(memory_entries_fts) VALUES('rebuild');")

	out, err := s.execTabs("SELECT key, content FROM memory_entries WHERE embedding IS NULL OR length(embedding) = 0;")
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(out) == "" {
		return 0, nil
	}

	type kc struct{ key, content string }
	var items []kc
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		content := strings.TrimSpace(parts[1])
		if key != "" && content != "" {
			items = append(items, kc{key, content})
		}
	}
	if len(items) == 0 {
		return 0, nil
	}

	texts := make([]string, len(items))
	for i, it := range items {
		texts[i] = it.content
	}

	s.mu.Unlock()
	batchVecs, batchErr := s.embedder.embedBatch(texts)
	s.mu.Lock()

	reEmbedded := 0
	if batchErr == nil && len(batchVecs) == len(items) {
		for i, vec := range batchVecs {
			if len(vec) == 0 {
				continue
			}
			b := vecToBytes(vec)
			_, _ = s.exec(fmt.Sprintf(
				"UPDATE memory_entries SET embedding=%s WHERE key=%s;",
				sqlBlobOrNull(b), sqlQuote(items[i].key),
			))
			reEmbedded++
		}
	} else {
		for _, it := range items {
			s.mu.Unlock()
			emb, embErr := s.getOrComputeEmbedding(it.content)
			s.mu.Lock()
			if embErr != nil || len(emb) == 0 {
				continue
			}
			_, _ = s.exec(fmt.Sprintf(
				"UPDATE memory_entries SET embedding=%s WHERE key=%s;",
				sqlBlobOrNull(emb), sqlQuote(it.key),
			))
			reEmbedded++
		}
	}
	return reEmbedded, nil
}
