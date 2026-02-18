package agent

import (
	"testing"

	"github.com/highclaw/highclaw/internal/config"
)

func TestEmbeddingFallbackProviderFromModelPrefix(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.Model = "glm/glm-5"
	got := embeddingFallbackProvider(cfg)
	if got != "glm" {
		t.Fatalf("expected glm, got %s", got)
	}
}

func TestEmbeddingProviderFallsBackToModelProviderKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.EmbeddingProvider = "openai"
	cfg.Agent.Model = "openrouter/anthropic/claude-sonnet-4"
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		"openrouter": {APIKey: "sk-openrouter", BaseURL: "https://openrouter.ai/api/v1"},
	}
	ep := createEmbeddingProvider(cfg)
	o, ok := ep.(*openAIEmbedding)
	if !ok {
		t.Fatalf("expected openAIEmbedding, got %T", ep)
	}
	if o.apiKey != "sk-openrouter" {
		t.Fatalf("expected fallback api key from openrouter, got %q", o.apiKey)
	}
	if o.baseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("expected fallback baseURL from openrouter, got %q", o.baseURL)
	}
}

func TestEmbeddingProviderNoKeyReturnsNoop(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.EmbeddingProvider = "openai"
	cfg.Agent.Providers = map[string]config.ProviderConfig{}
	ep := createEmbeddingProvider(cfg)
	if ep.name() != "none" {
		t.Fatalf("expected noop embedder, got %s", ep.name())
	}
}
