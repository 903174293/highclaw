package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type memoryStore interface {
	init() error
	store(key, content string) error
	forget(key string) error
	recall(query, key string, limit int) ([]memoryEntry, error)
	location() string
}

type disabledMemoryStore struct{}

func newDisabledMemoryStore() *disabledMemoryStore { return &disabledMemoryStore{} }

func (s *disabledMemoryStore) init() error { return nil }

func (s *disabledMemoryStore) store(key, content string) error {
	return fmt.Errorf("memory backend is disabled")
}

func (s *disabledMemoryStore) forget(key string) error {
	return fmt.Errorf("memory backend is disabled")
}

func (s *disabledMemoryStore) recall(query, key string, limit int) ([]memoryEntry, error) {
	return nil, nil
}

func (s *disabledMemoryStore) location() string { return "disabled" }

type markdownMemoryStore struct {
	filePath string
	mu       sync.Mutex
}

func newMarkdownMemoryStore(workspace string) *markdownMemoryStore {
	base := strings.TrimSpace(workspace)
	if base == "" {
		base = filepath.Join(os.TempDir(), "highclaw-workspace")
	}
	return &markdownMemoryStore{
		filePath: filepath.Join(base, "memory", "memory_store.json"),
	}
}

func (s *markdownMemoryStore) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return os.WriteFile(s.filePath, []byte("{}"), 0o644)
	}
	return nil
}

func (s *markdownMemoryStore) store(key, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return err
	}
	data[key] = memoryEntry{
		Key:       key,
		Content:   content,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	return s.saveLocked(data)
}

func (s *markdownMemoryStore) forget(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return err
	}
	delete(data, key)
	return s.saveLocked(data)
}

func (s *markdownMemoryStore) recall(query, key string, limit int) ([]memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	lq := strings.ToLower(strings.TrimSpace(query))
	lk := strings.TrimSpace(key)
	out := make([]memoryEntry, 0, len(data))
	for _, entry := range data {
		if lk != "" && entry.Key != lk {
			continue
		}
		if lq != "" {
			if !strings.Contains(strings.ToLower(entry.Key), lq) && !strings.Contains(strings.ToLower(entry.Content), lq) {
				continue
			}
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *markdownMemoryStore) location() string { return s.filePath }

func (s *markdownMemoryStore) loadLocked() (map[string]memoryEntry, error) {
	data := map[string]memoryEntry{}
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *markdownMemoryStore) saveLocked(data map[string]memoryEntry) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0o644)
}
