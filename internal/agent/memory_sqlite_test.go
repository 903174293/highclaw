package agent

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/highclaw/highclaw/internal/config"
)

type fakeEmbedder struct{}

func (fakeEmbedder) name() string    { return "fake" }
func (fakeEmbedder) dimensions() int { return 2 }
func (fakeEmbedder) embedOne(text string) ([]float32, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(text, "rust") {
		return []float32{1, 0}, nil
	}
	if strings.Contains(text, "python") {
		return []float32{0, 1}, nil
	}
	return []float32{0.5, 0.5}, nil
}
func (f fakeEmbedder) embedBatch(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		vec, err := f.embedOne(t)
		if err != nil {
			return nil, err
		}
		result[i] = vec
	}
	return result, nil
}

// queryScalar 测试辅助：执行 SQL 返回单个字符串值
func queryScalar(db *sql.DB, query string, args ...any) string {
	var v string
	_ = db.QueryRow(query, args...).Scan(&v)
	return v
}

func TestSQLiteMemoryStoreStoreRecallForget(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("k1", "rust memory engine", "core", memoryMeta{}); err != nil {
		t.Fatalf("store k1: %v", err)
	}
	if err := store.store("k2", "python tools", "daily", memoryMeta{SessionKey: "s1"}); err != nil {
		t.Fatalf("store k2: %v", err)
	}

	entries, err := store.recall("rust", "", "", 5)
	if err != nil {
		t.Fatalf("recall rust: %v", err)
	}
	if len(entries) == 0 || entries[0].Key != "k1" {
		t.Fatalf("expected k1 in recall, got %#v", entries)
	}

	entries, err = store.recall("", "k2", "s1", 5)
	if err != nil {
		t.Fatalf("recall key/session: %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "k2" {
		t.Fatalf("expected one k2 entry, got %#v", entries)
	}

	removed, err := store.forget("k2")
	if err != nil {
		t.Fatalf("forget k2: %v", err)
	}
	if !removed {
		t.Fatalf("expected removed=true")
	}
	removed, err = store.forget("k2")
	if err != nil {
		t.Fatalf("forget k2 again: %v", err)
	}
	if removed {
		t.Fatalf("expected removed=false on second forget")
	}
}

func TestSQLiteMemoryStorePathAndEmptyRecall(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if got, want := store.location(), filepath.Join(ws, "memory", "brain.db"); got != want {
		t.Fatalf("db path mismatch: got %s want %s", got, want)
	}
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	entries, err := store.recall("", "", "", 5)
	if err != nil {
		t.Fatalf("empty recall: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty recall results, got %d", len(entries))
	}
}

func TestSQLiteMemoryStoreSchemaHasEmbeddingCache(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	db := store.dbForHygiene()
	cnt := queryScalar(db, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='embedding_cache'")
	if cnt != "1" {
		t.Fatalf("expected embedding_cache table, got %q", cnt)
	}
	var hasEmb bool
	rows, _ := db.Query("PRAGMA table_info(memory_entries)")
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk)
		if name == "embedding" {
			hasEmb = true
		}
	}
	if !hasEmb {
		t.Fatalf("expected embedding column in memory_entries")
	}
}

func TestSQLiteMemoryStoreEmbeddingCacheEviction(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	store.embedder = fakeEmbedder{}
	store.embeddingCacheSize = 1
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("k1", "rust content", "core", memoryMeta{}); err != nil {
		t.Fatalf("store k1: %v", err)
	}
	if err := store.store("k2", "python content", "core", memoryMeta{}); err != nil {
		t.Fatalf("store k2: %v", err)
	}
	db := store.dbForHygiene()
	cnt := queryScalar(db, "SELECT COUNT(*) FROM embedding_cache")
	if cnt != "1" {
		t.Fatalf("expected cache size 1, got %q", cnt)
	}
}

func TestSQLiteMemoryStoreRecallWithEmbedding(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	store.embedder = fakeEmbedder{}
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("k1", "rust memory engine", "core", memoryMeta{}); err != nil {
		t.Fatalf("store k1: %v", err)
	}
	if err := store.store("k2", "python utility scripts", "core", memoryMeta{}); err != nil {
		t.Fatalf("store k2: %v", err)
	}
	entries, err := store.recall("rust", "", "", 5)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected entries")
	}
	if entries[0].Key != "k1" {
		t.Fatalf("expected k1 first, got %#v", entries)
	}
	db := store.dbForHygiene()
	cnt := queryScalar(db, "SELECT COUNT(*) FROM embedding_cache")
	if cnt == "0" {
		t.Fatalf("expected embedding cache to be populated")
	}
}

func TestSQLiteMemoryStoreRecallLimitZero(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("k1", "rust memory engine", "core", memoryMeta{}); err != nil {
		t.Fatalf("store: %v", err)
	}
	entries, err := store.recall("rust", "", "", 0)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 results when limit=0, got %d", len(entries))
	}
}

func TestSQLiteMemoryStoreRecallMatchesByKey(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("rust_preferences", "user likes systems programming", "core", memoryMeta{}); err != nil {
		t.Fatalf("store: %v", err)
	}
	entries, err := store.recall("rust_preferences", "", "", 10)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(entries) == 0 || entries[0].Key != "rust_preferences" {
		t.Fatalf("expected key-based match, got %#v", entries)
	}
}

func TestSQLiteMemoryStoreRecallSpecialQueriesDoNotCrash(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := store.store("q1", "He said hello world", "core", memoryMeta{}); err != nil {
		t.Fatalf("store q1: %v", err)
	}
	if err := store.store("q2", "function call test", "core", memoryMeta{}); err != nil {
		t.Fatalf("store q2: %v", err)
	}

	cases := []string{
		`"hello world"`,
		`*`,
		`(test)`,
		`'; DROP TABLE memory_entries; --`,
	}
	for _, q := range cases {
		entries, err := store.recall(q, "", "", 10)
		if err != nil {
			t.Fatalf("recall should not error for query %q: %v", q, err)
		}
		if len(entries) > 10 {
			t.Fatalf("recall should respect limit for query %q", q)
		}
	}

	db := store.dbForHygiene()
	cnt := queryScalar(db, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memory_entries'")
	if cnt != "1" {
		t.Fatalf("memory_entries table should exist, got %q", cnt)
	}
}

func TestSQLiteMemoryStoreGetListCountHealth(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !store.healthCheck() {
		t.Fatalf("expected healthCheck=true")
	}
	if err := store.store("a", "core memory", "core", memoryMeta{}); err != nil {
		t.Fatalf("store a: %v", err)
	}
	if err := store.store("b", "daily memory", "daily", memoryMeta{}); err != nil {
		t.Fatalf("store b: %v", err)
	}
	count, err := store.count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count=2 got %d", count)
	}
	got, err := store.get("a")
	if err != nil {
		t.Fatalf("get a: %v", err)
	}
	if got == nil || got.Key != "a" {
		t.Fatalf("expected get a")
	}
	all, err := store.list("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected list all=2 got %d", len(all))
	}
	core, err := store.list("core")
	if err != nil {
		t.Fatalf("list core: %v", err)
	}
	if len(core) != 1 || core[0].Key != "a" {
		t.Fatalf("expected core list only a, got %#v", core)
	}
}

func TestFTSContentModeTriggersExist(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	db := store.dbForHygiene()
	for _, name := range []string{"memory_entries_fts_ai", "memory_entries_fts_ad", "memory_entries_fts_au"} {
		cnt := queryScalar(db, "SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name=?", name)
		if cnt != "1" {
			t.Fatalf("trigger %s should exist, got %q", name, cnt)
		}
	}
}

func TestBM25ScoreNormalization(t *testing.T) {
	entries := []memoryEntry{
		{Key: "a", Score: 2.0},
		{Key: "b", Score: 1.0},
		{Key: "c", Score: 0.5},
	}
	normalized := normalizeBM25Scores(entries)
	if normalized[0].Score != 1.0 {
		t.Fatalf("expected max score normalized to 1.0, got %f", normalized[0].Score)
	}
	if normalized[1].Score != 0.5 {
		t.Fatalf("expected score 0.5, got %f", normalized[1].Score)
	}
	if normalized[2].Score != 0.25 {
		t.Fatalf("expected score 0.25, got %f", normalized[2].Score)
	}
}

func TestEmbedBatchFakeEmbedder(t *testing.T) {
	f := fakeEmbedder{}
	texts := []string{"rust code", "python script", "generic text"}
	vecs, err := f.embedBatch(texts)
	if err != nil {
		t.Fatalf("embedBatch: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("expected 3 results, got %d", len(vecs))
	}
	if vecs[0][0] != 1 || vecs[0][1] != 0 {
		t.Fatalf("expected rust=[1,0], got %v", vecs[0])
	}
	if vecs[1][0] != 0 || vecs[1][1] != 1 {
		t.Fatalf("expected python=[0,1], got %v", vecs[1])
	}
}

func TestInProcessSQLiteNoCLIDependency(t *testing.T) {
	ws := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.Workspace = ws
	store := newSQLiteMemoryStore(cfg)
	if err := store.init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if store.db == nil {
		t.Fatalf("expected in-process *sql.DB connection, got nil")
	}
	if err := store.store("test", "in-process sqlite works", "core", memoryMeta{}); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := store.get("test")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Content != "in-process sqlite works" {
		t.Fatalf("expected content match, got %#v", got)
	}
}
