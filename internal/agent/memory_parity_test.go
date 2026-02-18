package agent

import (
	"strings"
	"testing"
)

func TestAutosaveMemoryKeyPrefixAndUniqueness(t *testing.T) {
	k1 := autosaveMemoryKey("user_msg")
	k2 := autosaveMemoryKey("user_msg")
	if !strings.HasPrefix(k1, "user_msg_") {
		t.Fatalf("key1 prefix mismatch: %s", k1)
	}
	if !strings.HasPrefix(k2, "user_msg_") {
		t.Fatalf("key2 prefix mismatch: %s", k2)
	}
	if k1 == k2 {
		t.Fatalf("autosave keys should be unique")
	}
}

func TestConversationMemoryKeyDefaults(t *testing.T) {
	key := conversationMemoryKey("", "", "")
	parts := strings.Split(key, "_")
	if len(parts) < 3 {
		t.Fatalf("conversation key should have 3 parts, got %q", key)
	}
	if parts[0] != "unknown" {
		t.Fatalf("expected default channel unknown, got %s", parts[0])
	}
	if parts[1] != "user" {
		t.Fatalf("expected default sender user, got %s", parts[1])
	}
}
