package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAnthropicNativeKey(t *testing.T) {
	// sk-ant- 前缀 = Anthropic 原生 key，使用 x-api-key header
	if !isAnthropicNativeKey("sk-ant-api03-abc") {
		t.Fatal("expected sk-ant- prefix to be native key")
	}
	// 非 sk-ant- 前缀 = 第三方兼容 provider，使用 Bearer
	if isAnthropicNativeKey("sk-cp-minimax-key") {
		t.Fatal("did not expect non-anthropic key to be detected as native")
	}
	if isAnthropicNativeKey("eyJhbGciOi...") {
		t.Fatal("did not expect JWT-like token to be native key")
	}
}

func TestAnthropicClientUsesCorrectAuthHeader(t *testing.T) {
	type seen struct {
		Authorization string
		APIKeyHeader  string
	}
	run := func(t *testing.T, token string, expectBearer bool) {
		t.Helper()
		var got seen
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got.Authorization = r.Header.Get("Authorization")
			got.APIKeyHeader = r.Header.Get("x-api-key")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"x","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
		}))
		defer srv.Close()

		c := NewAnthropicClientWithBaseURL(token, srv.URL)
		_, err := c.Chat(context.Background(), &ChatRequest{
			Model:     "claude-sonnet-4",
			MaxTokens: 16,
			Messages: []Message{
				{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
			},
		})
		if err != nil {
			t.Fatalf("chat failed: %v", err)
		}

		if expectBearer {
			if got.Authorization == "" || got.APIKeyHeader != "" {
				t.Fatalf("expected bearer auth only, got Authorization=%q x-api-key=%q", got.Authorization, got.APIKeyHeader)
			}
			return
		}
		if got.Authorization != "" || got.APIKeyHeader == "" {
			t.Fatalf("expected x-api-key only, got Authorization=%q x-api-key=%q", got.Authorization, got.APIKeyHeader)
		}
	}

	// 第三方兼容 provider（MiniMax 等）使用 Bearer
	run(t, "sk-cp-minimax-key", true)
	// Anthropic 原生 key 使用 x-api-key
	run(t, "sk-ant-api03-native-key", false)
}
