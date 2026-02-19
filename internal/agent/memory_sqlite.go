package agent

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/config"
	_ "modernc.org/sqlite"
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
	db                 *sql.DB
	vectorWeight       float64
	keywordWeight      float64
	embeddingCacheSize int
	embedder           embeddingProvider
	mu                 sync.Mutex
}

// newSQLiteMemoryStore 根据配置创建 SQLite 内存存储实例
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

// openDB 打开或复用 database/sql 连接
func (s *sqliteMemoryStore) openDB() (*sql.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	db, err := sql.Open("sqlite", s.dbPath+"?_pragma=busy_timeout%3d5000&_pragma=journal_mode%3dwal")
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1)
	s.db = db
	return db, nil
}

// init 初始化数据库表结构、索引和 FTS5 trigger
func (s *sqliteMemoryStore) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		s.dbPath = filepath.Join(os.TempDir(), "highclaw-memory.db")
		if err2 := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err2 != nil {
			return fmt.Errorf("create memory state dir: %w", err)
		}
	}

	db, err := s.openDB()
	if err != nil {
		return err
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
	_, err = db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	migrations := []string{
		"ALTER TABLE memory_entries ADD COLUMN category TEXT NOT NULL DEFAULT 'core';",
		"ALTER TABLE memory_entries ADD COLUMN embedding BLOB;",
		"ALTER TABLE memory_entries ADD COLUMN created_at TEXT NOT NULL DEFAULT '';",
		"ALTER TABLE memory_entries ADD COLUMN session_key TEXT NOT NULL DEFAULT '';",
		"ALTER TABLE memory_entries ADD COLUMN channel TEXT NOT NULL DEFAULT '';",
		"ALTER TABLE memory_entries ADD COLUMN sender TEXT NOT NULL DEFAULT '';",
		"ALTER TABLE memory_entries ADD COLUMN message_id TEXT NOT NULL DEFAULT '';",
	}
	for _, m := range migrations {
		_, _ = db.Exec(m)
	}

	indices := []string{
		"CREATE INDEX IF NOT EXISTS idx_memory_entries_updated_at ON memory_entries(updated_at DESC);",
		"CREATE INDEX IF NOT EXISTS idx_memory_entries_category ON memory_entries(category);",
		"CREATE INDEX IF NOT EXISTS idx_memory_entries_session_key ON memory_entries(session_key);",
		"CREATE INDEX IF NOT EXISTS idx_memory_entries_channel_sender ON memory_entries(channel, sender);",
		"CREATE INDEX IF NOT EXISTS idx_embedding_cache_accessed ON embedding_cache(accessed_at);",
	}
	for _, idx := range indices {
		_, _ = db.Exec(idx)
	}
	_, _ = db.Exec("UPDATE memory_entries SET created_at = updated_at WHERE created_at = '';")
	s.ensureFTSContentMode(db)
	return nil
}

// ensureFTSContentMode 将 FTS5 迁移为 content= 关联表模式并创建自动同步 trigger
func (s *sqliteMemoryStore) ensureFTSContentMode(db *sql.DB) {
	var count int
	_ = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name='memory_entries_fts_ai'").Scan(&count)
	if count == 1 {
		return
	}
	db.Exec("DROP TABLE IF EXISTS memory_entries_fts;")
	db.Exec("CREATE VIRTUAL TABLE memory_entries_fts USING fts5(key, content, content=memory_entries, content_rowid=rowid);")
	db.Exec("INSERT INTO memory_entries_fts(memory_entries_fts) VALUES('rebuild');")
	db.Exec("CREATE TRIGGER memory_entries_fts_ai AFTER INSERT ON memory_entries BEGIN INSERT INTO memory_entries_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content); END;")
	db.Exec("CREATE TRIGGER memory_entries_fts_ad AFTER DELETE ON memory_entries BEGIN INSERT INTO memory_entries_fts(memory_entries_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content); END;")
	db.Exec("CREATE TRIGGER memory_entries_fts_au AFTER UPDATE ON memory_entries BEGIN INSERT INTO memory_entries_fts(memory_entries_fts, rowid, key, content) VALUES ('delete', old.rowid, old.key, old.content); INSERT INTO memory_entries_fts(rowid, key, content) VALUES (new.rowid, new.key, new.content); END;")
}

// store 写入或更新一条记忆
func (s *sqliteMemoryStore) store(key, content, category string, meta memoryMeta) error {
	emb, _ := s.getOrComputeEmbedding(content)

	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.openDB()
	if err != nil {
		return err
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "core"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(
		`INSERT INTO memory_entries(key, content, category, embedding, created_at, session_key, channel, sender, message_id, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(key) DO UPDATE SET content=excluded.content, category=excluded.category, embedding=excluded.embedding,
		   session_key=excluded.session_key, channel=excluded.channel, sender=excluded.sender, message_id=excluded.message_id, updated_at=excluded.updated_at`,
		key, content, category, emb, now,
		strings.TrimSpace(meta.SessionKey), strings.TrimSpace(meta.Channel),
		strings.TrimSpace(meta.Sender), strings.TrimSpace(meta.MessageID), now,
	)
	return err
}

// forget 删除指定 key 的记忆
func (s *sqliteMemoryStore) forget(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.openDB()
	if err != nil {
		return false, err
	}
	var cnt int
	_ = db.QueryRow("SELECT COUNT(*) FROM memory_entries WHERE key=?", key).Scan(&cnt)
	existed := cnt == 1
	_, err = db.Exec("DELETE FROM memory_entries WHERE key=?", key)
	if err != nil {
		return false, err
	}
	return existed, nil
}

// get 获取指定 key 的记忆条目
func (s *sqliteMemoryStore) get(key string) (*memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	row := db.QueryRow(
		"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE key=? LIMIT 1",
		strings.TrimSpace(key),
	)
	var e memoryEntry
	if err := row.Scan(&e.Key, &e.Content, &e.Category, &e.SessionKey, &e.Channel, &e.Sender, &e.MessageID, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

// list 列出指定分类（或全部）的记忆条目
func (s *sqliteMemoryStore) list(category string) ([]memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	category = strings.ToLower(strings.TrimSpace(category))
	var rows *sql.Rows
	if category != "" {
		rows, err = db.Query(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE category=? ORDER BY updated_at DESC",
			category,
		)
	} else {
		rows, err = db.Query("SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries ORDER BY updated_at DESC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemoryEntries(rows)
}

// count 返回记忆总条数
func (s *sqliteMemoryStore) count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) FROM memory_entries").Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}

// healthCheck 检查数据库连接是否正常
func (s *sqliteMemoryStore) healthCheck() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.openDB()
	if err != nil {
		return false
	}
	var v int
	return db.QueryRow("SELECT 1").Scan(&v) == nil
}

// recall 混合检索：FTS5 关键词 + 向量相似度
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

	db, err := s.openDB()
	if err != nil {
		return nil, err
	}

	sessionFilter := strings.TrimSpace(sessionKey)
	var keywordEntries []memoryEntry

	switch {
	case strings.TrimSpace(key) != "":
		keywordEntries, err = s.recallByKey(db, key, sessionFilter, limit)
	case strings.TrimSpace(query) != "":
		keywordEntries, err = s.recallByFTS(db, query, sessionFilter, limit)
		if err != nil {
			keywordEntries, err = s.recallByLike(db, query, sessionFilter, limit)
		}
		if len(keywordEntries) == 0 {
			if likeEntries, likeErr := s.recallByLike(db, query, sessionFilter, limit); likeErr == nil && len(likeEntries) > 0 {
				keywordEntries = likeEntries
			}
		}
	default:
		keywordEntries, err = s.recallDefault(db, sessionFilter, limit)
	}
	if err != nil {
		return nil, err
	}

	keywordEntries = normalizeBM25Scores(keywordEntries)
	if strings.TrimSpace(query) == "" || len(queryEmbedding) == 0 {
		return keywordEntries, nil
	}
	vectorEntries, _ := s.vectorSearch(db, queryEmbedding, sessionFilter, limit*2)
	if len(vectorEntries) == 0 {
		return keywordEntries, nil
	}
	return s.hybridMerge(keywordEntries, vectorEntries, limit), nil
}

// recallByKey 按 key 精确匹配检索
func (s *sqliteMemoryStore) recallByKey(db *sql.DB, key, sessionFilter string, limit int) ([]memoryEntry, error) {
	var rows *sql.Rows
	var err error
	if sessionFilter != "" {
		rows, err = db.Query(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE key=? AND session_key=? ORDER BY updated_at DESC LIMIT ?",
			key, sessionFilter, limit)
	} else {
		rows, err = db.Query(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE key=? ORDER BY updated_at DESC LIMIT ?",
			key, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemoryEntries(rows)
}

// recallByFTS 通过 FTS5 全文检索
func (s *sqliteMemoryStore) recallByFTS(db *sql.DB, query, sessionFilter string, limit int) ([]memoryEntry, error) {
	ftsExpr := ftsQuery(strings.TrimSpace(query))
	var rows *sql.Rows
	var err error
	if sessionFilter != "" {
		rows, err = db.Query(
			"SELECT m.key, m.content, m.category, m.session_key, m.channel, m.sender, m.message_id, m.created_at, m.updated_at, bm25(memory_entries_fts) "+
				"FROM memory_entries_fts JOIN memory_entries m ON m.rowid = memory_entries_fts.rowid "+
				"WHERE memory_entries_fts MATCH ? AND m.session_key=? ORDER BY bm25(memory_entries_fts) ASC, m.updated_at DESC LIMIT ?",
			ftsExpr, sessionFilter, limit)
	} else {
		rows, err = db.Query(
			"SELECT m.key, m.content, m.category, m.session_key, m.channel, m.sender, m.message_id, m.created_at, m.updated_at, bm25(memory_entries_fts) "+
				"FROM memory_entries_fts JOIN memory_entries m ON m.rowid = memory_entries_fts.rowid "+
				"WHERE memory_entries_fts MATCH ? ORDER BY bm25(memory_entries_fts) ASC, m.updated_at DESC LIMIT ?",
			ftsExpr, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemoryEntriesWithScore(rows)
}

// recallByLike 使用 LIKE 模糊匹配作为 FTS 的回退方案
func (s *sqliteMemoryStore) recallByLike(db *sql.DB, query, sessionFilter string, limit int) ([]memoryEntry, error) {
	q := "%" + strings.TrimSpace(query) + "%"
	var rows *sql.Rows
	var err error
	if sessionFilter != "" {
		rows, err = db.Query(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE session_key=? AND (key LIKE ? OR content LIKE ?) ORDER BY updated_at DESC LIMIT ?",
			sessionFilter, q, q, limit)
	} else {
		rows, err = db.Query(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE key LIKE ? OR content LIKE ? ORDER BY updated_at DESC LIMIT ?",
			q, q, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemoryEntries(rows)
}

// recallDefault 无条件按时间排序检索
func (s *sqliteMemoryStore) recallDefault(db *sql.DB, sessionFilter string, limit int) ([]memoryEntry, error) {
	var rows *sql.Rows
	var err error
	if sessionFilter != "" {
		rows, err = db.Query(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries WHERE session_key=? ORDER BY updated_at DESC LIMIT ?",
			sessionFilter, limit)
	} else {
		rows, err = db.Query(
			"SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at FROM memory_entries ORDER BY updated_at DESC LIMIT ?",
			limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemoryEntries(rows)
}

// scanMemoryEntries 扫描标准 9 列结果集
func scanMemoryEntries(rows *sql.Rows) ([]memoryEntry, error) {
	var entries []memoryEntry
	for rows.Next() {
		var e memoryEntry
		if err := rows.Scan(&e.Key, &e.Content, &e.Category, &e.SessionKey, &e.Channel, &e.Sender, &e.MessageID, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// scanMemoryEntriesWithScore 扫描 9 列 + BM25 分数
func scanMemoryEntriesWithScore(rows *sql.Rows) ([]memoryEntry, error) {
	var entries []memoryEntry
	for rows.Next() {
		var e memoryEntry
		var score float64
		if err := rows.Scan(&e.Key, &e.Content, &e.Category, &e.SessionKey, &e.Channel, &e.Sender, &e.MessageID, &e.CreatedAt, &e.UpdatedAt, &score); err != nil {
			return nil, err
		}
		e.Score = math.Abs(score)
		entries = append(entries, e)
	}
	return entries, rows.Err()
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

// getOrComputeEmbedding 获取或计算文本的 embedding，带 LRU 缓存
func (s *sqliteMemoryStore) getOrComputeEmbedding(text string) ([]byte, error) {
	if s.embedder == nil || s.embedder.name() == "none" {
		return nil, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}

	s.mu.Lock()
	db, err := s.openDB()
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	hash := s.contentHash(text)

	var cachedBlob []byte
	if err := db.QueryRow("SELECT embedding FROM embedding_cache WHERE content_hash=?", hash).Scan(&cachedBlob); err == nil && len(cachedBlob) > 0 {
		db.Exec("UPDATE embedding_cache SET accessed_at=? WHERE content_hash=?", time.Now().UTC().Format(time.RFC3339Nano), hash)
		s.mu.Unlock()
		return cachedBlob, nil
	}
	s.mu.Unlock()

	vec, err := s.embedder.embedOne(text)
	if err != nil || len(vec) == 0 {
		return nil, err
	}
	b := vecToBytes(vec)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	s.mu.Lock()
	db, _ = s.openDB()
	db.Exec("INSERT OR REPLACE INTO embedding_cache(content_hash, embedding, created_at, accessed_at) VALUES(?,?,?,?)", hash, b, now, now)
	if s.embeddingCacheSize > 0 {
		db.Exec("DELETE FROM embedding_cache WHERE content_hash IN (SELECT content_hash FROM embedding_cache ORDER BY accessed_at ASC LIMIT MAX(0, (SELECT COUNT(*) FROM embedding_cache)-?))", s.embeddingCacheSize)
	}
	s.mu.Unlock()
	return b, nil
}

// vectorSearch 暴力向量搜索，计算 cosine similarity
func (s *sqliteMemoryStore) vectorSearch(db *sql.DB, queryEmbedding []byte, sessionFilter string, limit int) ([]memoryEntry, error) {
	if len(queryEmbedding) == 0 {
		return nil, nil
	}
	var rows *sql.Rows
	var err error
	if sessionFilter != "" {
		rows, err = db.Query("SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at, embedding FROM memory_entries WHERE embedding IS NOT NULL AND session_key=?", sessionFilter)
	} else {
		rows, err = db.Query("SELECT key, content, category, session_key, channel, sender, message_id, created_at, updated_at, embedding FROM memory_entries WHERE embedding IS NOT NULL")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	qv := bytesToVec(queryEmbedding)
	var scored []memoryEntry
	for rows.Next() {
		var e memoryEntry
		var embBlob []byte
		if err := rows.Scan(&e.Key, &e.Content, &e.Category, &e.SessionKey, &e.Channel, &e.Sender, &e.MessageID, &e.CreatedAt, &e.UpdatedAt, &embBlob); err != nil {
			continue
		}
		score := cosineSimilarity(qv, bytesToVec(embBlob))
		if score <= 0 {
			continue
		}
		e.Score = score
		scored = append(scored, e)
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

// hybridMerge 合并关键词搜索和向量搜索结果
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

	db, err := s.openDB()
	if err != nil {
		return 0, err
	}
	db.Exec("INSERT INTO memory_entries_fts(memory_entries_fts) VALUES('rebuild')")

	rows, err := db.Query("SELECT key, content FROM memory_entries WHERE embedding IS NULL OR length(embedding) = 0")
	if err != nil {
		return 0, err
	}
	type kc struct{ key, content string }
	var items []kc
	for rows.Next() {
		var k, c string
		if err := rows.Scan(&k, &c); err != nil {
			continue
		}
		if strings.TrimSpace(k) != "" && strings.TrimSpace(c) != "" {
			items = append(items, kc{k, c})
		}
	}
	rows.Close()
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
			db.Exec("UPDATE memory_entries SET embedding=? WHERE key=?", b, items[i].key)
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
			db.Exec("UPDATE memory_entries SET embedding=? WHERE key=?", emb, it.key)
			reEmbedded++
		}
	}
	return reEmbedded, nil
}

// dbForHygiene 供 memory_hygiene 使用的内部连接获取方法
func (s *sqliteMemoryStore) dbForHygiene() *sql.DB {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, _ := s.openDB()
	return db
}
