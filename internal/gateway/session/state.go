package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
)

type currentSessionState struct {
	Key       string `json:"key"`
	UpdatedAt int64  `json:"updatedAt"`
}

func stateDir() string {
	return filepath.Join(config.ConfigDir(), "state")
}

func currentSessionPath() string {
	return filepath.Join(stateDir(), "current_session.json")
}

// SetCurrent stores the active session key used by CLI/TUI defaults.
func SetCurrent(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		if err := os.Remove(currentSessionPath()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear current session: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	payload, err := json.MarshalIndent(currentSessionState{
		Key:       key,
		UpdatedAt: time.Now().UnixMilli(),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal current session: %w", err)
	}
	if err := os.WriteFile(currentSessionPath(), payload, 0o644); err != nil {
		return fmt.Errorf("write current session: %w", err)
	}
	return nil
}

// Current returns the active session key if present.
func Current() (string, error) {
	raw, err := os.ReadFile(currentSessionPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read current session: %w", err)
	}
	var st currentSessionState
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", fmt.Errorf("parse current session: %w", err)
	}
	return strings.TrimSpace(st.Key), nil
}

// SaveFromHistory writes a complete session snapshot and marks it current.
func SaveFromHistory(sessionKey, channel, agentID, model string, history []protocol.ChatMessage) error {
	now := time.Now()
	createdAt := now
	if old, err := Load(sessionKey); err == nil && !old.CreatedAt.IsZero() {
		createdAt = old.CreatedAt
	}

	sess := &Session{
		Key:            strings.TrimSpace(sessionKey),
		Channel:        strings.TrimSpace(channel),
		AgentID:        strings.TrimSpace(agentID),
		Model:          strings.TrimSpace(model),
		CreatedAt:      createdAt,
		LastActivityAt: now,
	}
	if sess.Channel == "" {
		sess.Channel = "cli"
	}
	if sess.AgentID == "" {
		sess.AgentID = "main"
	}

	for _, msg := range history {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if role == "" || content == "" {
			continue
		}
		ts := msg.Timestamp
		if ts <= 0 {
			ts = time.Now().UnixMilli()
		}
		sess.AddMessage(protocol.ChatMessage{
			Role:      role,
			Content:   content,
			Channel:   sess.Channel,
			Timestamp: ts,
		})
	}
	if err := sess.Save(); err != nil {
		return err
	}
	_ = SetCurrent(sess.Key)
	return nil
}
