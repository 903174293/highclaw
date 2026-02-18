package agent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/config"
)

type memoryEntry struct {
	Key       string
	Content   string
	UpdatedAt string
}

type sqliteMemoryStore struct {
	dbPath string
	mu     sync.Mutex
}

func newSQLiteMemoryStore() *sqliteMemoryStore {
	return &sqliteMemoryStore{
		dbPath: filepath.Join(config.ConfigDir(), "state", "memory.db"),
	}
}

func (s *sqliteMemoryStore) location() string {
	return s.dbPath
}

func (s *sqliteMemoryStore) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		// Fallback for restricted environments.
		s.dbPath = filepath.Join(os.TempDir(), "highclaw-memory.db")
		if err2 := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err2 != nil {
			return fmt.Errorf("create memory state dir: %w", err)
		}
	}

	ddl := `
CREATE TABLE IF NOT EXISTS memory_entries (
  key TEXT PRIMARY KEY,
  content TEXT NOT NULL,
  updated_at TEXT NOT NULL
);`
	_, err := s.exec(ddl)
	return err
}

func (s *sqliteMemoryStore) store(key, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sql := fmt.Sprintf(
		"INSERT INTO memory_entries(key, content, updated_at) VALUES(%s, %s, %s) "+
			"ON CONFLICT(key) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at;",
		sqlQuote(key), sqlQuote(content), sqlQuote(time.Now().UTC().Format(time.RFC3339Nano)),
	)
	_, err := s.exec(sql)
	return err
}

func (s *sqliteMemoryStore) forget(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.exec(fmt.Sprintf("DELETE FROM memory_entries WHERE key=%s;", sqlQuote(key)))
	return err
}

func (s *sqliteMemoryStore) recall(query, key string, limit int) ([]memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	var sql string
	switch {
	case strings.TrimSpace(key) != "":
		sql = fmt.Sprintf(
			"SELECT key, content, updated_at FROM memory_entries WHERE key=%s ORDER BY updated_at DESC LIMIT %d;",
			sqlQuote(key), limit,
		)
	case strings.TrimSpace(query) != "":
		q := "%" + strings.TrimSpace(query) + "%"
		sql = fmt.Sprintf(
			"SELECT key, content, updated_at FROM memory_entries "+
				"WHERE key LIKE %s OR content LIKE %s ORDER BY updated_at DESC LIMIT %d;",
			sqlQuote(q), sqlQuote(q), limit,
		)
	default:
		sql = fmt.Sprintf(
			"SELECT key, content, updated_at FROM memory_entries ORDER BY updated_at DESC LIMIT %d;",
			limit,
		)
	}

	out, err := s.execTabs(sql)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	entries := make([]memoryEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		entry := memoryEntry{
			Key:     parts[0],
			Content: parts[1],
		}
		if len(parts) >= 3 {
			entry.UpdatedAt = parts[2]
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *sqliteMemoryStore) exec(sql string) (string, error) {
	cmd := exec.Command("sqlite3", s.dbPath, sql)
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
	cmd := exec.Command("sqlite3", "-tabs", "-noheader", s.dbPath, sql)
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
