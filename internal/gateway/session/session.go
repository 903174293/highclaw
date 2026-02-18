// Package session manages chat sessions on the gateway.
package session

import (
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/gateway/protocol"
)

// Session represents an active chat session.
type Session struct {
	Key            string    `json:"key"`
	Channel        string    `json:"channel"`
	AgentID        string    `json:"agentId,omitempty"`
	Model          string    `json:"model,omitempty"`
	ThinkingLevel  string    `json:"thinkingLevel,omitempty"`
	VerboseLevel   string    `json:"verboseLevel,omitempty"`
	MessageCount   int       `json:"messageCount"`
	CreatedAt      time.Time `json:"createdAt"`
	LastActivityAt time.Time `json:"lastActivityAt"`

	// GroupActivation controls how the bot activates in groups.
	// "mention" = only respond when mentioned; "always" = respond to all.
	GroupActivation string `json:"groupActivation,omitempty"`

	// History 存储消息历史，用于持久化
	History []protocol.ChatMessage `json:"history,omitempty"`

	mu       sync.Mutex             `json:"-"`
	messages []protocol.ChatMessage `json:"-"`
}

// Manager manages all active sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// GetOrCreate returns an existing session or creates a new one.
func (m *Manager) GetOrCreate(key, channel string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[key]; ok {
		return sess
	}

	sess := &Session{
		Key:            key,
		Channel:        channel,
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
		messages:       make([]protocol.ChatMessage, 0),
	}
	m.sessions[key] = sess
	return sess
}

// Get returns a session by key, if it exists.
func (m *Manager) Get(key string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[key]
	return sess, ok
}

// Delete removes a session.
func (m *Manager) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, key)
}

// List returns info about all active sessions.
func (m *Manager) List() []protocol.SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]protocol.SessionInfo, 0, len(m.sessions))
	for _, sess := range m.sessions {
		result = append(result, protocol.SessionInfo{
			Key:            sess.Key,
			Channel:        sess.Channel,
			AgentID:        sess.AgentID,
			Model:          sess.Model,
			ThinkingLevel:  sess.ThinkingLevel,
			MessageCount:   sess.MessageCount,
			LastActivityAt: sess.LastActivityAt.UnixMilli(),
		})
	}
	return result
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// AddMessage appends a message to a session's history.
func (s *Session) AddMessage(msg protocol.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg.Timestamp = time.Now().UnixMilli()
	s.messages = append(s.messages, msg)
	s.History = append(s.History, msg) // 同步更新 History 用于持久化
	s.MessageCount = len(s.messages)
	s.LastActivityAt = time.Now()
}

// Messages returns the session's message history.
func (s *Session) Messages() []protocol.ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 优先使用 messages，如果为空则使用 History（从磁盘加载的情况）
	src := s.messages
	if len(src) == 0 && len(s.History) > 0 {
		src = s.History
	}
	cp := make([]protocol.ChatMessage, len(src))
	copy(cp, src)
	return cp
}

// Reset clears the session's message history.
func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = s.messages[:0]
	s.History = s.History[:0]
	s.MessageCount = 0
}

// SyncHistory 在保存前同步 messages 到 History
func (s *Session) SyncHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) > 0 {
		s.History = make([]protocol.ChatMessage, len(s.messages))
		copy(s.History, s.messages)
	}
}

// RestoreMessages 从 History 恢复 messages（加载后调用）
func (s *Session) RestoreMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.History) > 0 && len(s.messages) == 0 {
		s.messages = make([]protocol.ChatMessage, len(s.History))
		copy(s.messages, s.History)
	}
}
