package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

	// 保存前同步 messages 到 History
	s.SyncHistory()

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

	// 从 History 恢复 messages
	sess.RestoreMessages()

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
			continue
		}
	}
}

// PruneResult 记录自动清理的结果
type PruneResult struct {
	Pruned int
	Capped int
}

// PruneStaleSessions 清理过期会话和超量会话
// maxAge: 会话最大空闲天数（0 表示不按时间清理）
// maxCount: 最大会话数量（0 表示不限制）
func PruneStaleSessions(maxAgeDays, maxCount int) (PruneResult, error) {
	all, err := LoadAll()
	if err != nil {
		return PruneResult{}, err
	}
	if len(all) == 0 {
		return PruneResult{}, nil
	}

	var result PruneResult
	now := time.Now()

	// 按 LastActivityAt 降序排列
	sort.Slice(all, func(i, j int) bool {
		return all[i].LastActivityAt.After(all[j].LastActivityAt)
	})

	keep := make(map[string]bool)
	for i, s := range all {
		// 按时间淘汰
		if maxAgeDays > 0 && now.Sub(s.LastActivityAt).Hours() > float64(maxAgeDays*24) {
			if err := Delete(s.Key); err == nil {
				result.Pruned++
			}
			continue
		}
		// 按数量淘汰
		if maxCount > 0 && i >= maxCount {
			if err := Delete(s.Key); err == nil {
				result.Capped++
			}
			continue
		}
		keep[s.Key] = true
	}

	return result, nil
}

// GetLastSessionKey 返回最近活跃的会话 key（按 LastActivityAt 排序）
func GetLastSessionKey() string {
	sessions, err := LoadAll()
	if err != nil || len(sessions) == 0 {
		return ""
	}
	// 按最近活动时间倒序
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActivityAt.After(sessions[j].LastActivityAt)
	})
	return sessions[0].Key
}
