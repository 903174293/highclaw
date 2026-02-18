package agent

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/highclaw/highclaw/internal/agent/providers"
	"github.com/highclaw/highclaw/internal/config"
)

func testModelManager() *ModelManager {
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		"openrouter": {APIKey: "sk-or"},
		"glm":        {APIKey: "sk-glm"},
		"groq":       {APIKey: "sk-groq"},
	}
	cfg.ModelRoutes = []config.ModelRouteConfig{
		{Hint: "fast", Provider: "groq", Model: "llama-3.3-70b-versatile"},
	}
	return NewModelManager(cfg, slog.Default())
}

func TestResolvePrimaryProviderPrefersModelPrefix(t *testing.T) {
	m := testModelManager()
	got := m.resolvePrimaryProvider("", "glm/glm-5")
	if got != "glm" {
		t.Fatalf("expected glm, got %s", got)
	}
}

func TestResolvePrimaryProviderPrefersModelPrefixEvenWhenNotConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		"openrouter": {APIKey: "sk-or"},
	}
	m := NewModelManager(cfg, slog.Default())
	got := m.resolvePrimaryProvider("", "glm/glm-5")
	if got != "glm" {
		t.Fatalf("expected explicit prefix provider glm, got %s", got)
	}
}

func TestResolvePrimaryProviderSupportsCustomURLPrefix(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		"openrouter": {APIKey: "sk-or"},
	}
	m := NewModelManager(cfg, slog.Default())
	model := "custom:https://api.example.com/v1/my-model"
	got := m.resolvePrimaryProvider("", model)
	if got != "custom:https://api.example.com/v1" {
		t.Fatalf("expected custom URL provider prefix, got %s", got)
	}
}

func TestSplitModelPrefixCustomURLs(t *testing.T) {
	cases := []struct {
		model      string
		wantPrefix string
		wantRest   string
	}{
		{
			model:      "custom:https://api.example.com/v1/my-model",
			wantPrefix: "custom:https://api.example.com/v1",
			wantRest:   "my-model",
		},
		{
			model:      "anthropic-custom:https://api.example.com/v1/claude-sonnet-4",
			wantPrefix: "anthropic-custom:https://api.example.com/v1",
			wantRest:   "claude-sonnet-4",
		},
	}
	for _, tc := range cases {
		prefix, rest, ok := splitModelPrefix(tc.model)
		if !ok {
			t.Fatalf("expected split to succeed for model=%s", tc.model)
		}
		if prefix != tc.wantPrefix || rest != tc.wantRest {
			t.Fatalf("unexpected split for model=%s, got (%s,%s), want (%s,%s)", tc.model, prefix, rest, tc.wantPrefix, tc.wantRest)
		}
	}
}

func TestNormalizeModelForProviderCustom(t *testing.T) {
	provider := "custom:https://api.example.com/v1"
	model := "custom:https://api.example.com/v1/my-model"
	got := normalizeModelForProvider(model, provider)
	if got != "my-model" {
		t.Fatalf("expected normalized custom model id, got %s", got)
	}
}

func TestNormalizeModelForProviderOpenRouterDoublePrefix(t *testing.T) {
	provider := "openrouter"
	model := "openrouter/anthropic/claude-sonnet-4"
	got := normalizeModelForProvider(model, provider)
	if got != "anthropic/claude-sonnet-4" {
		t.Fatalf("expected openrouter prefix stripped, got %s", got)
	}
}

func TestProviderCandidatesUseConfiguredFallbackOrder(t *testing.T) {
	m := testModelManager()
	m.cfg.Reliability.FallbackProviders = []string{"anthropic", "openai"}
	got := m.providerCandidates("openrouter")
	want := []string{"openrouter", "anthropic", "openai"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidates len: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected candidates order: got=%v want=%v", got, want)
		}
	}
}

func TestProviderCandidatesDeduplicate(t *testing.T) {
	m := testModelManager()
	m.cfg.Reliability.FallbackProviders = []string{"openrouter", "openai", "openai"}
	got := m.providerCandidates("openrouter")
	want := []string{"openrouter", "openai"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidates len: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected candidates order: got=%v want=%v", got, want)
		}
	}
}

func TestProviderCandidatesIgnoreUnknownFallback(t *testing.T) {
	m := testModelManager()
	m.cfg.Reliability.FallbackProviders = []string{"unknown-provider", "openai"}
	got := m.providerCandidates("openrouter")
	want := []string{"openrouter", "openai"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidates len: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected candidates order: got=%v want=%v", got, want)
		}
	}
}

func TestProviderCandidatesKeepUnknownPrimary(t *testing.T) {
	m := testModelManager()
	m.cfg.Reliability.FallbackProviders = []string{"openai"}
	got := m.providerCandidates("unknown-primary")
	want := []string{"unknown-primary", "openai"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidates len: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected candidates order: got=%v want=%v", got, want)
		}
	}
}

func TestSplitModelPrefixCustomURLKeepsCase(t *testing.T) {
	model := "custom:https://API.Example.com/V1/My-Model"
	prefix, rest, ok := splitModelPrefix(model)
	if !ok {
		t.Fatal("expected custom split to succeed")
	}
	if prefix != "custom:https://API.Example.com/V1" || rest != "My-Model" {
		t.Fatalf("expected case-preserving split, got prefix=%s rest=%s", prefix, rest)
	}
}

func TestResolveHintRouteMatchesConfiguredRoute(t *testing.T) {
	m := testModelManager()
	p, model, ok := m.resolveHintRoute("hint:fast")
	if !ok {
		t.Fatal("expected hint route to resolve")
	}
	if p != "groq" {
		t.Fatalf("expected provider groq, got %s", p)
	}
	if model != "llama-3.3-70b-versatile" {
		t.Fatalf("unexpected resolved model: %s", model)
	}
}

func TestResolveHintRouteUnknownFallsBack(t *testing.T) {
	m := testModelManager()
	_, _, ok := m.resolveHintRoute("hint:reasoning")
	if ok {
		t.Fatal("expected unknown hint not to resolve")
	}
}

func TestResolveProviderConfigAlias(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		"gemini": {APIKey: "sk-gemini"},
	}
	pcfg, ok := resolveProviderConfig(cfg, "google")
	if !ok {
		t.Fatal("expected google alias to resolve gemini config")
	}
	if pcfg.APIKey != "sk-gemini" {
		t.Fatalf("unexpected api key from alias resolution: %s", pcfg.APIKey)
	}
}

func TestResolveProviderConfigFromEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-from-env")
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{}
	pcfg, ok := resolveProviderConfig(cfg, "openrouter")
	if !ok {
		t.Fatal("expected openrouter key from env to resolve")
	}
	if pcfg.APIKey != "sk-from-env" {
		t.Fatalf("unexpected env api key resolution: %s", pcfg.APIKey)
	}
}

func TestResolveProviderConfigFromRouteAPIKey(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{}
	cfg.ModelRoutes = []config.ModelRouteConfig{
		{Hint: "reasoning", Provider: "glm", Model: "glm-5", APIKey: "sk-route-glm"},
	}
	pcfg, ok := resolveProviderConfig(cfg, "glm")
	if !ok {
		t.Fatal("expected glm key from model route to resolve")
	}
	if pcfg.APIKey != "sk-route-glm" {
		t.Fatalf("unexpected route api key resolution: %s", pcfg.APIKey)
	}
}

func TestProviderFactoryCreateCustomProvider(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		"custom:https://example.com/v1": {APIKey: "sk-custom"},
	}
	f := NewProviderFactory()
	p, err := f.Create("custom:https://example.com/v1", cfg)
	if err != nil {
		t.Fatalf("expected custom provider to be created, got error: %v", err)
	}
	if p == nil {
		t.Fatal("expected custom provider instance")
	}
}

func TestProviderFactoryCreateAnthropicCustomProvider(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-anthropic-custom")
	cfg := config.Default()
	f := NewProviderFactory()
	p, err := f.Create("anthropic-custom:https://api.example.com", cfg)
	if err != nil {
		t.Fatalf("expected anthropic-custom provider to be created, got error: %v", err)
	}
	if p == nil {
		t.Fatal("expected anthropic-custom provider instance")
	}
}

func TestProviderFactoryUnknownProviderHint(t *testing.T) {
	cfg := config.Default()
	f := NewProviderFactory()
	_, err := f.Create("does-not-exist", cfg)
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "run `highclaw onboard --interactive`") {
		t.Fatalf("expected onboarding hint in error, got: %s", msg)
	}
}

func TestProviderFactoryCreateProviderAliasFromWizardList(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "sk-kimi")
	cfg := config.Default()
	f := NewProviderFactory()
	p, err := f.Create("kimi", cfg)
	if err != nil {
		t.Fatalf("expected kimi alias provider to be created, got error: %v", err)
	}
	if p == nil {
		t.Fatal("expected kimi provider instance")
	}
}

func TestProviderFactoryOpenRouterAddsHeaders(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		"openrouter": {APIKey: "sk-or"},
	}
	f := NewProviderFactory()
	p, err := f.Create("openrouter", cfg)
	if err != nil {
		t.Fatalf("expected openrouter provider to be created, got error: %v", err)
	}
	op, ok := p.(*openAIProvider)
	if !ok {
		t.Fatalf("expected *openAIProvider, got %T", p)
	}
	if op.client.Headers["HTTP-Referer"] == "" || op.client.Headers["X-Title"] == "" {
		t.Fatalf("expected openrouter extra headers to be set, got: %#v", op.client.Headers)
	}
}

func TestIsNonRetryableProviderErrorFromAPIStatus(t *testing.T) {
	if !isNonRetryableProviderError(&providers.APIError{StatusCode: 400, Body: "bad request"}) {
		t.Fatal("expected 400 to be non-retryable")
	}
	if isNonRetryableProviderError(&providers.APIError{StatusCode: 429, Body: "rate limit"}) {
		t.Fatal("expected 429 to be retryable")
	}
	if isNonRetryableProviderError(&providers.APIError{StatusCode: 408, Body: "timeout"}) {
		t.Fatal("expected 408 to be retryable")
	}
}

func TestIsNonRetryableProviderErrorFromWrappedString(t *testing.T) {
	err := errors.New("openai API call: API error 402: payment required")
	if !isNonRetryableProviderError(err) {
		t.Fatal("expected wrapped 402 error to be non-retryable")
	}
	err429 := errors.New("transient failure 429 from provider")
	if isNonRetryableProviderError(err429) {
		t.Fatal("expected wrapped 429 to be retryable")
	}
}

func TestProviderDisplayNameMappings(t *testing.T) {
	cases := map[string]string{
		"glm":           "GLM",
		"z.ai":          "Z.AI",
		"minimax":       "MiniMax",
		"cloudflare-ai": "Cloudflare AI",
		"openrouter":    "OpenRouter",
	}
	for in, want := range cases {
		if got := providerDisplayName(in); got != want {
			t.Fatalf("unexpected provider display for %s: got=%s want=%s", in, got, want)
		}
	}
}

func TestDefaultBaseURLParityWithZeroClaw(t *testing.T) {
	cases := map[string]string{
		"groq":         "https://api.groq.com/openai",
		"fireworks-ai": "https://api.fireworks.ai/inference",
		"vercel-ai":    "https://api.vercel.ai",
		"cloudflare":   "https://gateway.ai.cloudflare.com/v1",
		"moonshot":     "https://api.moonshot.cn",
	}
	for provider, want := range cases {
		if got := defaultBaseURLForProvider(provider); got != want {
			t.Fatalf("unexpected default base url for %s: got=%s want=%s", provider, got, want)
		}
	}
}

func TestFormatProviderErrorIncludesStatusText(t *testing.T) {
	err := &providers.APIError{
		StatusCode: 402,
		Body:       `{"error":{"message":"payment required"}}`,
	}
	got := formatProviderError("openrouter", err)
	if !strings.Contains(got, "OpenRouter API error (402 Payment Required)") {
		t.Fatalf("unexpected formatted provider error: %s", got)
	}
}

func TestFormatProviderErrorPassthrough(t *testing.T) {
	err := errors.New("plain provider failure")
	got := formatProviderError("openrouter", err)
	if got != "plain provider failure" {
		t.Fatalf("expected passthrough non-API error, got: %s", got)
	}
}
