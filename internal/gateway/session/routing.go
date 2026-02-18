package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/highclaw/highclaw/internal/config"
)

const DefaultExternalSessionKey = "agent:main:main"

type SessionBinding struct {
	Channel      string `json:"channel"`
	Conversation string `json:"conversation"`
	SessionKey   string `json:"sessionKey"`
}

type bindingStore struct {
	Bindings map[string]string `json:"bindings"`
}

func bindingsPath() string {
	return filepath.Join(config.ConfigDir(), "state", "session_bindings.json")
}

func bindingMapKey(channel, conversation string) string {
	return strings.ToLower(strings.TrimSpace(channel)) + "|" + strings.TrimSpace(conversation)
}

func loadBindings() (bindingStore, error) {
	path := bindingsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return bindingStore{Bindings: map[string]string{}}, nil
		}
		return bindingStore{}, err
	}
	var s bindingStore
	if err := json.Unmarshal(raw, &s); err != nil {
		return bindingStore{}, err
	}
	if s.Bindings == nil {
		s.Bindings = map[string]string{}
	}
	return s, nil
}

func saveBindings(s bindingStore) error {
	path := bindingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func ResolveSession(channel, conversation string) (string, error) {
	channel = strings.TrimSpace(channel)
	conversation = strings.TrimSpace(conversation)
	if channel == "" || conversation == "" {
		return DefaultExternalSessionKey, nil
	}
	s, err := loadBindings()
	if err != nil {
		return "", fmt.Errorf("load session bindings: %w", err)
	}
	if key, ok := s.Bindings[bindingMapKey(channel, conversation)]; ok && strings.TrimSpace(key) != "" {
		return strings.TrimSpace(key), nil
	}
	return DefaultExternalSessionKey, nil
}

func SetBinding(channel, conversation, sessionKey string) error {
	channel = strings.TrimSpace(channel)
	conversation = strings.TrimSpace(conversation)
	sessionKey = strings.TrimSpace(sessionKey)
	if channel == "" || conversation == "" || sessionKey == "" {
		return fmt.Errorf("channel, conversation, and sessionKey are required")
	}
	s, err := loadBindings()
	if err != nil {
		return fmt.Errorf("load session bindings: %w", err)
	}
	s.Bindings[bindingMapKey(channel, conversation)] = sessionKey
	return saveBindings(s)
}

func RemoveBinding(channel, conversation string) error {
	channel = strings.TrimSpace(channel)
	conversation = strings.TrimSpace(conversation)
	if channel == "" || conversation == "" {
		return fmt.Errorf("channel and conversation are required")
	}
	s, err := loadBindings()
	if err != nil {
		return fmt.Errorf("load session bindings: %w", err)
	}
	delete(s.Bindings, bindingMapKey(channel, conversation))
	return saveBindings(s)
}

func ListBindings() ([]SessionBinding, error) {
	s, err := loadBindings()
	if err != nil {
		return nil, fmt.Errorf("load session bindings: %w", err)
	}
	out := make([]SessionBinding, 0, len(s.Bindings))
	for k, v := range s.Bindings {
		parts := strings.SplitN(k, "|", 2)
		channel := ""
		conversation := ""
		if len(parts) > 0 {
			channel = parts[0]
		}
		if len(parts) > 1 {
			conversation = parts[1]
		}
		out = append(out, SessionBinding{
			Channel:      channel,
			Conversation: conversation,
			SessionKey:   v,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel == out[j].Channel {
			return out[i].Conversation < out[j].Conversation
		}
		return out[i].Channel < out[j].Channel
	})
	return out, nil
}
