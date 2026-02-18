package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAnthropicSetupToken(t *testing.T) {
	if !isAnthropicSetupToken("sk-ant-oat01-abc") {
		t.Fatal("expected setup token to be detected")
	}
	if isAnthropicSetupToken("sk-ant-api-key") {
		t.Fatal("did not expect standard api key to be detected as setup token")
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

	run(t, "sk-ant-oat01-setup-token", true)
	run(t, "sk-ant-normal-api-key", false)
}
