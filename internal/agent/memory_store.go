package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type memoryStore interface {
	init() error
	store(key, content, category string, meta memoryMeta) error
	forget(key string) (bool, error)
	recall(query, key, sessionKey string, limit int) ([]memoryEntry, error)
	get(key string) (*memoryEntry, error)
	list(category string) ([]memoryEntry, error)
	count() (int, error)
	healthCheck() bool
	location() string
}

type memoryMeta struct {
	SessionKey string
	Channel    string
	Sender     string
	MessageID  string
}

type disabledMemoryStore struct{}

func newDisabledMemoryStore() *disabledMemoryStore { return &disabledMemoryStore{} }

func (s *disabledMemoryStore) init() error { return nil }

func (s *disabledMemoryStore) store(key, content, category string, meta memoryMeta) error {
	return fmt.Errorf("memory backend is disabled")
}

func (s *disabledMemoryStore) forget(key string) (bool, error) {
	return false, fmt.Errorf("memory backend is disabled")
}

func (s *disabledMemoryStore) recall(query, key, sessionKey string, limit int) ([]memoryEntry, error) {
	return nil, nil
}

func (s *disabledMemoryStore) get(key string) (*memoryEntry, error) {
	_ = key
	return nil, nil
}

func (s *disabledMemoryStore) list(category string) ([]memoryEntry, error) {
	_ = category
	return []memoryEntry{}, nil
}

func (s *disabledMemoryStore) count() (int, error) {
	return 0, nil
}

func (s *disabledMemoryStore) healthCheck() bool {
	return true
}

func (s *disabledMemoryStore) location() string { return "disabled" }

type markdownMemoryStore struct {
	workspaceDir string
	mu           sync.Mutex
}

func newMarkdownMemoryStore(workspace string) *markdownMemoryStore {
	base := strings.TrimSpace(workspace)
	if base == "" {
		base = filepath.Join(os.TempDir(), "highclaw-workspace")
	}
	return &markdownMemoryStore{
		workspaceDir: base,
	}
}

func (s *markdownMemoryStore) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.MkdirAll(s.memoryDir(), 0o755)
}

func (s *markdownMemoryStore) store(key, content, category string, meta memoryMeta) error {
	_ = meta
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.dailyPath()
	if strings.EqualFold(strings.TrimSpace(category), "core") {
		path = s.corePath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	entry := "- **" + strings.TrimSpace(key) + "**: " + strings.TrimSpace(content)

	existing := ""
	if raw, err := os.ReadFile(path); err == nil {
		existing = string(raw)
	}

	var updated string
	if strings.TrimSpace(existing) == "" {
		if path == s.corePath() {
			updated = "# Long-Term Memory\n\n" + entry + "\n"
		} else {
			updated = "# Daily Log — " + time.Now().Format("2006-01-02") + "\n\n" + entry + "\n"
		}
	} else {
		updated = strings.TrimRight(existing, "\n") + "\n\n" + entry + "\n"
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

func (s *markdownMemoryStore) forget(key string) (bool, error) {
	_ = key
	// Append-only store, same behavior as zeroclaw markdown backend.
	return false, nil
}

func (s *markdownMemoryStore) recall(query, key, sessionKey string, limit int) ([]memoryEntry, error) {
	_ = sessionKey
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAllEntriesLocked()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []memoryEntry{}, nil
	}

	lq := strings.ToLower(strings.TrimSpace(query))
	if lq == "" {
		return []memoryEntry{}, nil
	}
	keywords := strings.Fields(lq)
	lk := strings.TrimSpace(key)

	type scoredEntry struct {
		entry memoryEntry
		score float64
	}
	scored := make([]scoredEntry, 0, len(all))
	for _, entry := range all {
		if lk != "" && entry.Key != lk {
			continue
		}
		contentLower := strings.ToLower(entry.Content)
		matched := 0
		for _, kw := range keywords {
			if strings.Contains(contentLower, kw) {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		scored = append(scored, scoredEntry{
			entry: entry,
			score: float64(matched) / float64(len(keywords)),
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].entry.Key < scored[j].entry.Key
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]memoryEntry, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.entry)
	}
	return out, nil
}

func (s *markdownMemoryStore) location() string {
	return s.workspaceDir
}

func (s *markdownMemoryStore) get(key string) (*memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAllEntriesLocked()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if strings.TrimSpace(all[i].Key) == strings.TrimSpace(key) {
			entry := all[i]
			return &entry, nil
		}
	}
	return nil, nil
}

func (s *markdownMemoryStore) list(category string) ([]memoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAllEntriesLocked()
	if err != nil {
		return nil, err
	}
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return all, nil
	}
	out := make([]memoryEntry, 0, len(all))
	for _, e := range all {
		if strings.ToLower(strings.TrimSpace(e.Category)) == category {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *markdownMemoryStore) count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAllEntriesLocked()
	if err != nil {
		return 0, err
	}
	return len(all), nil
}

func (s *markdownMemoryStore) healthCheck() bool {
	if err := s.init(); err != nil {
		return false
	}
	return true
}

func (s *markdownMemoryStore) memoryDir() string {
	return filepath.Join(s.workspaceDir, "memory")
}

func (s *markdownMemoryStore) corePath() string {
	return filepath.Join(s.workspaceDir, "MEMORY.md")
}

func (s *markdownMemoryStore) dailyPath() string {
	return filepath.Join(s.memoryDir(), time.Now().Format("2006-01-02")+".md")
}

var markdownKeyLine = regexp.MustCompile(`^\*\*(.+?)\*\*:\s*(.*)$`)

func parseMarkdownLine(line, fallbackKey string) (string, string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", ""
	}
	match := markdownKeyLine.FindStringSubmatch(trimmed)
	if len(match) == 3 {
		return strings.TrimSpace(match[1]), strings.TrimSpace(match[2])
	}
	return fallbackKey, trimmed
}

func (s *markdownMemoryStore) parseEntriesFromFile(path string, category string) ([]memoryEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fileStem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	lines := strings.Split(string(raw), "\n")
	entries := make([]memoryEntry, 0, len(lines))
	for i, line := range lines {
		fallbackKey := fileStem + ":" + strconv.Itoa(i)
		key, content := parseMarkdownLine(line, fallbackKey)
		if key == "" || content == "" {
			continue
		}
		entries = append(entries, memoryEntry{
			Key:       key,
			Content:   content,
			Category:  category,
			UpdatedAt: fileStem,
		})
	}
	return entries, nil
}

func (s *markdownMemoryStore) readAllEntriesLocked() ([]memoryEntry, error) {
	entries := make([]memoryEntry, 0, 128)

	if _, err := os.Stat(s.corePath()); err == nil {
		coreEntries, err := s.parseEntriesFromFile(s.corePath(), "core")
		if err != nil {
			return nil, err
		}
		entries = append(entries, coreEntries...)
	}

	memDir := s.memoryDir()
	if _, err := os.Stat(memDir); err == nil {
		dirEntries, err := os.ReadDir(memDir)
		if err != nil {
			return nil, err
		}
		for _, de := range dirEntries {
			if de.IsDir() || filepath.Ext(de.Name()) != ".md" {
				continue
			}
			path := filepath.Join(memDir, de.Name())
			dailyEntries, err := s.parseEntriesFromFile(path, "daily")
			if err != nil {
				return nil, err
			}
			entries = append(entries, dailyEntries...)
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].UpdatedAt > entries[j].UpdatedAt })
	return entries, nil
}
