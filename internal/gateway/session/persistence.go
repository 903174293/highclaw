package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SessionsDir returns the path to the sessions directory.
func SessionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".highclaw/sessions"
	}
	return filepath.Join(home, ".highclaw", "sessions")
}

// Save persists a session to disk.
func (s *Session) Save() error {
	dir := SessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	// Sanitize session key for filename
	filename := sanitizeFilename(s.Key) + ".json"
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load loads a session from disk.
func Load(key string) (*Session, error) {
	dir := SessionsDir()
	filename := sanitizeFilename(key) + ".json"
	path := filepath.Join(dir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &sess, nil
}

// LoadAll loads all sessions from disk.
func LoadAll() ([]*Session, error) {
	dir := SessionsDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue // Skip invalid files
		}

		sessions = append(sessions, &sess)
	}

	return sessions, nil
}

// Delete removes a session file from disk.
func Delete(key string) error {
	dir := SessionsDir()
	filename := sanitizeFilename(key) + ".json"
	path := filepath.Join(dir, filename)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete session file: %w", err)
	}

	return nil
}

// sanitizeFilename converts a session key to a safe filename.
func sanitizeFilename(key string) string {
	// Replace unsafe characters with underscores
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, key)

	// Limit length
	if len(safe) > 200 {
		safe = safe[:200]
	}

	return safe
}

// AutoSave enables automatic session saving after each message.
func (m *Manager) AutoSave(enabled bool) {
	if !enabled {
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sess := range m.sessions {
		if err := sess.Save(); err != nil {
			// Best-effort persistence; individual failures should not panic runtime.
			continue
		}
	}
}
