package session

import (
	"testing"

	"github.com/highclaw/highclaw/internal/config"
)

// TestNormalizeID 测试 ID 规范化
func TestNormalizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  ", ""},
		{"Hello", "hello"},
		{"User@123!#$", "user-123"},
		{"very_long_name_that_exceeds_sixty_four_characters_and_should_be_truncated_properly", "very_long_name_that_exceeds_sixty_four_characters_and_should_be_"},
	}
	for _, tc := range tests {
		got := NormalizeID(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeID(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// TestBuildMainSessionKey 测试主会话 key 生成
func TestBuildMainSessionKey(t *testing.T) {
	tests := []struct {
		agentID string
		mainKey string
		want    string
	}{
		{"", "", "agent:main:main"},
		{"agent1", "prod", "agent:agent1:prod"},
		{"  ", "  ", "agent:main:main"},
	}
	for _, tc := range tests {
		got := BuildMainSessionKey(tc.agentID, tc.mainKey)
		if got != tc.want {
			t.Errorf("BuildMainSessionKey(%q, %q) = %q; want %q", tc.agentID, tc.mainKey, got, tc.want)
		}
	}
}

// TestBuildPeerSessionKey 测试 DM Scope 路由
func TestBuildPeerSessionKey(t *testing.T) {
	tests := []struct {
		name    string
		peer    PeerContext
		dmScope string
		want    string
	}{
		{
			name: "group_message",
			peer: PeerContext{
				Channel:  "telegram",
				PeerID:   "user123",
				PeerKind: "group",
				GroupID:  "group999",
			},
			dmScope: DMScopePerChannelPeer,
			want:    "agent:main:telegram:group:group999",
		},
		{
			name: "dm_main_scope",
			peer: PeerContext{
				Channel:  "whatsapp",
				PeerID:   "123456789",
				PeerKind: "direct",
			},
			dmScope: DMScopeMain,
			want:    "agent:main:main",
		},
		{
			name: "dm_per_peer",
			peer: PeerContext{
				Channel:  "whatsapp",
				PeerID:   "user456",
				PeerKind: "direct",
			},
			dmScope: DMScopePerPeer,
			want:    "agent:main:direct:user456",
		},
		{
			name: "dm_per_channel_peer",
			peer: PeerContext{
				Channel:  "telegram",
				PeerID:   "user789",
				PeerKind: "direct",
			},
			dmScope: DMScopePerChannelPeer,
			want:    "agent:main:telegram:direct:user789",
		},
		{
			name: "dm_per_account_channel_peer",
			peer: PeerContext{
				Channel:   "telegram",
				AccountID: "bot1",
				PeerID:    "user999",
				PeerKind:  "direct",
			},
			dmScope: DMScopePerAccountChPeer,
			want:    "agent:main:telegram:bot1:direct:user999",
		},
		{
			name: "empty_peer_id_fallback_main",
			peer: PeerContext{
				Channel:  "discord",
				PeerID:   "",
				PeerKind: "direct",
			},
			dmScope: DMScopePerChannelPeer,
			want:    "agent:main:main",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildPeerSessionKey(DefaultAgentID, DefaultMainKey, tc.peer, tc.dmScope, nil)
			if got != tc.want {
				t.Errorf("BuildPeerSessionKey() = %q; want %q", got, tc.want)
			}
		})
	}
}

// TestResolveSessionFromConfig 测试配置驱动的路由
func TestResolveSessionFromConfig(t *testing.T) {
	cfg := &config.Config{
		Session: config.SessionConfig{
			DMScope: DMScopePerChannelPeer,
			MainKey: "home",
		},
	}

	peer := PeerContext{
		Channel:  "whatsapp",
		PeerID:   "user123",
		PeerKind: "direct",
	}

	got := ResolveSessionFromConfig(cfg, peer)
	want := "agent:main:whatsapp:direct:user123"
	if got != want {
		t.Errorf("ResolveSessionFromConfig() = %q; want %q", got, want)
	}
}

// TestIdentityLinks 测试跨渠道身份合并
func TestIdentityLinks(t *testing.T) {
	links := map[string][]string{
		"alice": {"telegram:alice_tg", "whatsapp:alice_wa"},
	}

	peer := PeerContext{
		Channel:  "whatsapp",
		PeerID:   "alice_wa",
		PeerKind: "direct",
	}

	got := BuildPeerSessionKey(DefaultAgentID, DefaultMainKey, peer, DMScopePerPeer, links)
	want := "agent:main:direct:alice"
	if got != want {
		t.Errorf("IdentityLinks merge failed: got %q; want %q", got, want)
	}
}
