package model

// AllProviders returns all supported provider names (complete list from OpenClaw).
var AllProviders = []string{
	// Built-in providers (no models.providers config needed)
	"anthropic",
	"openai",
	"openai-codex",
	"google",
	"google-vertex",
	"google-antigravity",
	"google-gemini-cli",
	"amazon-bedrock",
	"openrouter",
	"vercel-ai-gateway",
	"xai",
	"groq",
	"cerebras",
	"mistral",
	"github-copilot",
	"huggingface",
	"zai",
	"opencode",
	"synthetic",

	// Custom providers (via models.providers config)
	"minimax",
	"minimax-cn",
	"qwen-portal",
	"venice",
	"moonshot",
	"together",
	"fireworks",
	"cohere",
	"perplexity",
	"deepseek",
	"qianfan",
	"glm",
	"xiaomi",
	"litellm",
}

// ProviderCategory represents the category of a provider.
type ProviderCategory string

const (
	CategoryBuiltIn ProviderCategory = "built-in" // No config needed, just API key
	CategoryCustom  ProviderCategory = "custom"   // Requires models.providers config
	CategoryOAuth   ProviderCategory = "oauth"    // OAuth-based authentication
	CategoryLocal   ProviderCategory = "local"    // Local models (Ollama, LM Studio)
)

// ProviderInfo contains metadata about a provider.
type ProviderInfo struct {
	ID          string
	Name        string
	Category    ProviderCategory
	AuthType    string // "api_key", "oauth", "token", "none"
	EnvVar      string // Primary environment variable for API key
	Description string
	DocsURL     string
}

// GetProviderInfo returns metadata for all providers.
func GetProviderInfo() map[string]ProviderInfo {
	return map[string]ProviderInfo{
		"anthropic": {
			ID:          "anthropic",
			Name:        "Anthropic",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "ANTHROPIC_API_KEY",
			Description: "Claude models via Anthropic API",
			DocsURL:     "https://docs.openclaw.ai/providers/anthropic",
		},
		"openai": {
			ID:          "openai",
			Name:        "OpenAI",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "OPENAI_API_KEY",
			Description: "GPT models via OpenAI API",
			DocsURL:     "https://docs.openclaw.ai/providers/openai",
		},
		"openai-codex": {
			ID:          "openai-codex",
			Name:        "OpenAI Codex",
			Category:    CategoryOAuth,
			AuthType:    "oauth",
			EnvVar:      "",
			Description: "GPT Codex models via ChatGPT OAuth",
			DocsURL:     "https://docs.openclaw.ai/providers/openai",
		},
		"google": {
			ID:          "google",
			Name:        "Google Gemini",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "GOOGLE_API_KEY",
			Description: "Gemini models via Google AI Studio",
			DocsURL:     "https://docs.openclaw.ai/providers/google",
		},
		"google-vertex": {
			ID:          "google-vertex",
			Name:        "Google Vertex AI",
			Category:    CategoryBuiltIn,
			AuthType:    "oauth",
			EnvVar:      "GOOGLE_APPLICATION_CREDENTIALS",
			Description: "Gemini models via Vertex AI",
			DocsURL:     "https://docs.openclaw.ai/providers/google-vertex",
		},
		"google-antigravity": {
			ID:          "google-antigravity",
			Name:        "Google Antigravity",
			Category:    CategoryBuiltIn,
			AuthType:    "oauth",
			EnvVar:      "",
			Description: "Internal Google models",
			DocsURL:     "https://docs.openclaw.ai/providers/google-antigravity",
		},
		"google-gemini-cli": {
			ID:          "google-gemini-cli",
			Name:        "Google Gemini CLI",
			Category:    CategoryBuiltIn,
			AuthType:    "oauth",
			EnvVar:      "",
			Description: "Gemini via CLI OAuth",
			DocsURL:     "https://docs.openclaw.ai/providers/google",
		},
		"amazon-bedrock": {
			ID:          "amazon-bedrock",
			Name:        "Amazon Bedrock",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "AWS_ACCESS_KEY_ID",
			Description: "Claude, Llama, Mistral via AWS Bedrock",
			DocsURL:     "https://docs.openclaw.ai/providers/bedrock",
		},
		"openrouter": {
			ID:          "openrouter",
			Name:        "OpenRouter",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "OPENROUTER_API_KEY",
			Description: "Unified API for multiple providers",
			DocsURL:     "https://docs.openclaw.ai/providers/openrouter",
		},
		"vercel-ai-gateway": {
			ID:          "vercel-ai-gateway",
			Name:        "Vercel AI Gateway",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "AI_GATEWAY_API_KEY",
			Description: "Vercel's AI Gateway",
			DocsURL:     "https://docs.openclaw.ai/providers/vercel-ai-gateway",
		},
		"xai": {
			ID:          "xai",
			Name:        "xAI",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "XAI_API_KEY",
			Description: "Grok models from xAI",
			DocsURL:     "https://docs.openclaw.ai/providers/xai",
		},
		"groq": {
			ID:          "groq",
			Name:        "Groq",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "GROQ_API_KEY",
			Description: "Ultra-fast LLM inference",
			DocsURL:     "https://docs.openclaw.ai/providers/groq",
		},
		"cerebras": {
			ID:          "cerebras",
			Name:        "Cerebras",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "CEREBRAS_API_KEY",
			Description: "Fast inference on Cerebras hardware",
			DocsURL:     "https://docs.openclaw.ai/providers/cerebras",
		},
		"mistral": {
			ID:          "mistral",
			Name:        "Mistral AI",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "MISTRAL_API_KEY",
			Description: "Mistral models",
			DocsURL:     "https://docs.openclaw.ai/providers/mistral",
		},
		"github-copilot": {
			ID:          "github-copilot",
			Name:        "GitHub Copilot",
			Category:    CategoryBuiltIn,
			AuthType:    "token",
			EnvVar:      "COPILOT_GITHUB_TOKEN",
			Description: "GitHub Copilot models",
			DocsURL:     "https://docs.openclaw.ai/providers/github-copilot",
		},
		"huggingface": {
			ID:          "huggingface",
			Name:        "Hugging Face",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "HUGGINGFACE_HUB_TOKEN",
			Description: "Hugging Face Inference API",
			DocsURL:     "https://docs.openclaw.ai/providers/huggingface",
		},
		"zai": {
			ID:          "zai",
			Name:        "Z.AI (GLM)",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "ZAI_API_KEY",
			Description: "GLM models via Z.AI",
			DocsURL:     "https://docs.openclaw.ai/providers/zai",
		},
		"opencode": {
			ID:          "opencode",
			Name:        "OpenCode Zen",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "OPENCODE_API_KEY",
			Description: "OpenCode Zen models",
			DocsURL:     "https://docs.openclaw.ai/providers/opencode",
		},
		"synthetic": {
			ID:          "synthetic",
			Name:        "Synthetic",
			Category:    CategoryBuiltIn,
			AuthType:    "api_key",
			EnvVar:      "SYNTHETIC_API_KEY",
			Description: "Anthropic-compatible models",
			DocsURL:     "https://docs.openclaw.ai/providers/synthetic",
		},
		"minimax": {
			ID:          "minimax",
			Name:        "MiniMax",
			Category:    CategoryCustom,
			AuthType:    "api_key",
			EnvVar:      "MINIMAX_API_KEY",
			Description: "MiniMax models",
			DocsURL:     "https://docs.openclaw.ai/providers/minimax",
		},
		"minimax-cn": {
			ID:          "minimax-cn",
			Name:        "MiniMax CN",
			Category:    CategoryCustom,
			AuthType:    "api_key",
			EnvVar:      "MINIMAX_API_KEY",
			Description: "MiniMax China endpoint",
			DocsURL:     "https://docs.openclaw.ai/providers/minimax",
		},
		"together": {
			ID:          "together",
			Name:        "Together AI",
			Category:    CategoryCustom,
			AuthType:    "api_key",
			EnvVar:      "TOGETHER_API_KEY",
			Description: "Together AI models",
			DocsURL:     "https://docs.openclaw.ai/providers/together",
		},
		"deepseek": {
			ID:          "deepseek",
			Name:        "DeepSeek",
			Category:    CategoryCustom,
			AuthType:    "api_key",
			EnvVar:      "DEEPSEEK_API_KEY",
			Description: "DeepSeek reasoning models",
			DocsURL:     "https://docs.openclaw.ai/providers/deepseek",
		},
		"litellm": {
			ID:          "litellm",
			Name:        "LiteLLM",
			Category:    CategoryLocal,
			AuthType:    "none",
			EnvVar:      "",
			Description: "LiteLLM unified gateway",
			DocsURL:     "https://docs.openclaw.ai/providers/litellm",
		},
	}
}
