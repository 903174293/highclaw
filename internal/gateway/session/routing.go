// routing.go — 会话路由引擎，实现 OpenClaw 兼容的 DM Scope 多级隔离
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/highclaw/highclaw/internal/config"
)

const (
	DefaultAgentID  = "main"
	DefaultMainKey  = "main"
	DefaultAccountID = "default"

	// DMScope 枚举值
	DMScopeMain               = "main"
	DMScopePerPeer            = "per-peer"
	DMScopePerChannelPeer     = "per-channel-peer"
	DMScopePerAccountChPeer   = "per-account-channel-peer"
)

// DefaultExternalSessionKey 兜底会话 key
var DefaultExternalSessionKey = BuildMainSessionKey(DefaultAgentID, DefaultMainKey)

// PeerContext 描述一条入站消息的来源上下文
type PeerContext struct {
	Channel      string // 渠道名: telegram / discord / whatsapp / webhook / websocket / web
	AccountID    string // bot 账户 ID（多 bot 场景），可为空
	PeerID       string // 对端用户 ID
	PeerKind     string // "direct" | "group" | "channel"
	GroupID      string // 群组 ID（PeerKind != "direct" 时有效）
	Conversation string // 对话标识（兼容旧绑定查询）
}

var invalidCharsRe = regexp.MustCompile(`[^a-z0-9_-]+`)

// NormalizeID 将任意字符串规范化为合法 session key 段
func NormalizeID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	s = invalidCharsRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	s = strings.Trim(s, "-")
	return s
}

// BuildMainSessionKey 构建主会话 key
func BuildMainSessionKey(agentID, mainKey string) string {
	a := NormalizeID(agentID)
	if a == "" {
		a = DefaultAgentID
	}
	m := NormalizeID(mainKey)
	if m == "" {
		m = DefaultMainKey
	}
	return fmt.Sprintf("agent:%s:%s", a, m)
}

// BuildPeerSessionKey 根据 DM Scope 构建会话 key（核心路由逻辑）
func BuildPeerSessionKey(agentID string, mainKey string, peer PeerContext, dmScope string, identityLinks map[string][]string) string {
	a := NormalizeID(agentID)
	if a == "" {
		a = DefaultAgentID
	}
	peerKind := strings.TrimSpace(strings.ToLower(peer.PeerKind))
	if peerKind == "" {
		peerKind = "direct"
	}

	// 群组/频道消息 → 按 channel:group:groupId 路由
	if peerKind != "direct" {
		ch := NormalizeID(peer.Channel)
		if ch == "" {
			ch = "unknown"
		}
		gid := NormalizeID(peer.GroupID)
		if gid == "" {
			gid = "unknown"
		}
		return fmt.Sprintf("agent:%s:%s:%s:%s", a, ch, peerKind, gid)
	}

	// DM 消息：根据 dmScope 决定隔离级别
	if dmScope == "" {
		dmScope = DMScopeMain
	}

	peerID := resolveLinkedPeerID(peer.Channel, peer.PeerID, identityLinks)
	peerID = NormalizeID(peerID)

	switch dmScope {
	case DMScopePerAccountChPeer:
		if peerID != "" {
			ch := NormalizeID(peer.Channel)
			if ch == "" {
				ch = "unknown"
			}
			acct := NormalizeID(peer.AccountID)
			if acct == "" {
				acct = DefaultAccountID
			}
			return fmt.Sprintf("agent:%s:%s:%s:direct:%s", a, ch, acct, peerID)
		}
	case DMScopePerChannelPeer:
		if peerID != "" {
			ch := NormalizeID(peer.Channel)
			if ch == "" {
				ch = "unknown"
			}
			return fmt.Sprintf("agent:%s:%s:direct:%s", a, ch, peerID)
		}
	case DMScopePerPeer:
		if peerID != "" {
			return fmt.Sprintf("agent:%s:direct:%s", a, peerID)
		}
	}

	// 兜底：回到主会话
	return BuildMainSessionKey(agentID, mainKey)
}

// resolveLinkedPeerID 通过 identityLinks 做跨渠道身份合并
func resolveLinkedPeerID(channel, peerID string, links map[string][]string) string {
	peerID = strings.TrimSpace(peerID)
	if peerID == "" || links == nil {
		return peerID
	}
	needle := strings.ToLower(strings.TrimSpace(channel)) + ":" + strings.ToLower(peerID)
	for canonical, aliases := range links {
		for _, alias := range aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == needle {
				return canonical
			}
		}
	}
	return peerID
}

// ResolveSessionFromConfig 根据配置和入站消息上下文，自动路由到正确的会话 key
func ResolveSessionFromConfig(cfg *config.Config, peer PeerContext) string {
	sc := cfg.Session
	dmScope := strings.TrimSpace(strings.ToLower(sc.DMScope))
	if dmScope == "" {
		dmScope = DMScopePerChannelPeer
	}
	mainKey := strings.TrimSpace(sc.MainKey)
	if mainKey == "" {
		mainKey = DefaultMainKey
	}

	// 先检查显式绑定（兼容旧逻辑）
	conversation := strings.TrimSpace(peer.Conversation)
	channel := strings.TrimSpace(peer.Channel)
	if channel != "" && conversation != "" {
		if bound, err := lookupBinding(channel, conversation); err == nil && bound != "" {
			return bound
		}
	}

	return BuildPeerSessionKey(DefaultAgentID, mainKey, peer, dmScope, sc.IdentityLinks)
}

// ResolveSession 保持向后兼容的旧接口（无 peer 信息时回退到绑定查询 + 默认会话）
func ResolveSession(channel, conversation string) (string, error) {
	channel = strings.TrimSpace(channel)
	conversation = strings.TrimSpace(conversation)
	if channel == "" || conversation == "" {
		return DefaultExternalSessionKey, nil
	}
	if bound, err := lookupBinding(channel, conversation); err == nil && bound != "" {
		return bound, nil
	}
	return DefaultExternalSessionKey, nil
}

// --- 绑定存储（与旧实现兼容） ---

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

func lookupBinding(channel, conversation string) (string, error) {
	s, err := loadBindings()
	if err != nil {
		return "", err
	}
	key := s.Bindings[bindingMapKey(channel, conversation)]
	return strings.TrimSpace(key), nil
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

// SetBinding 创建 channel+conversation → sessionKey 的显式绑定
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

// RemoveBinding 删除绑定
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

// ListBindings 列出所有绑定
func ListBindings() ([]SessionBinding, error) {
	s, err := loadBindings()
	if err != nil {
		return nil, fmt.Errorf("load session bindings: %w", err)
	}
	out := make([]SessionBinding, 0, len(s.Bindings))
	for k, v := range s.Bindings {
		parts := strings.SplitN(k, "|", 2)
		ch := ""
		conv := ""
		if len(parts) > 0 {
			ch = parts[0]
		}
		if len(parts) > 1 {
			conv = parts[1]
		}
		out = append(out, SessionBinding{
			Channel:      ch,
			Conversation: conv,
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
