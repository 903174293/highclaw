package model

// Complete model definitions for all providers

// AnthropicModelsComplete returns all Anthropic Claude models.
var AnthropicModelsComplete = []Model{
	{ID: "claude-opus-4-6", Name: "Claude Opus 4.6", Provider: "anthropic", Description: "Latest Opus (Feb 2025)", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-opus-4-5", Name: "Claude Opus 4.5", Provider: "anthropic", Description: "Opus 4.5 (Nov 2024)", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-opus-4", Name: "Claude Opus 4", Provider: "anthropic", Description: "Opus 4 baseline", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Provider: "anthropic", Description: "Latest Sonnet", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Provider: "anthropic", Description: "Sonnet 4 baseline", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-3-7-sonnet-20250219", Name: "Claude 3.7 Sonnet", Provider: "anthropic", Description: "Claude 3.7 Sonnet", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet (Oct)", Provider: "anthropic", Description: "Claude 3.5 Sonnet v2", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-3-5-sonnet-20240620", Name: "Claude 3.5 Sonnet (Jun)", Provider: "anthropic", Description: "Claude 3.5 Sonnet v1", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Provider: "anthropic", Description: "Latest Haiku", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-haiku-4", Name: "Claude Haiku 4", Provider: "anthropic", Description: "Haiku 4 baseline", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Provider: "anthropic", Description: "Fast and efficient", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: "anthropic", Description: "Claude 3 Haiku", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic", Description: "Claude 3 Opus", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: "anthropic", Description: "Claude 3 Sonnet", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
}

// OpenAIModelsComplete returns all OpenAI models.
var OpenAIModelsComplete = []Model{
	// GPT-5 series
	{ID: "gpt-5.1-codex", Name: "GPT-5.1 Codex", Provider: "openai", Description: "Latest GPT-5 Codex", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "reasoning"}},
	{ID: "gpt-5.1", Name: "GPT-5.1", Provider: "openai", Description: "GPT-5.1", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Description: "GPT-5 baseline", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},

	// GPT-4 series
	{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Description: "Multimodal flagship", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", Description: "Affordable and fast", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", Description: "Previous flagship", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4", Name: "GPT-4", Provider: "openai", Description: "Original GPT-4", MaxTokens: 8192, Capabilities: []string{"tools"}},
	{ID: "gpt-4.1", Name: "GPT-4.1", Provider: "openai", Description: "GPT-4.1", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4.1-mini", Name: "GPT-4.1 Mini", Provider: "openai", Description: "GPT-4.1 Mini", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4.1-nano", Name: "GPT-4.1 Nano", Provider: "openai", Description: "GPT-4.1 Nano", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},

	// GPT-3.5
	{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: "openai", Description: "Fast and affordable", MaxTokens: 16385, Capabilities: []string{"tools"}},

	// o-series (reasoning)
	{ID: "o1", Name: "o1", Provider: "openai", Description: "Reasoning model", MaxTokens: 200000, Capabilities: []string{"reasoning"}},
	{ID: "o1-mini", Name: "o1 Mini", Provider: "openai", Description: "Faster reasoning", MaxTokens: 128000, Capabilities: []string{"reasoning"}},
	{ID: "o3-mini", Name: "o3 Mini", Provider: "openai", Description: "Latest reasoning", MaxTokens: 200000, Capabilities: []string{"reasoning"}},
}

// OpenAICodexModelsComplete returns OpenAI Codex models (via OAuth).
var OpenAICodexModelsComplete = []Model{
	{ID: "gpt-5.3-codex", Name: "GPT-5.3 Codex", Provider: "openai-codex", Description: "Latest Codex via OAuth", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "reasoning"}},
	{ID: "gpt-5.1-codex", Name: "GPT-5.1 Codex", Provider: "openai-codex", Description: "GPT-5.1 Codex", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4o", Name: "GPT-4o (Codex)", Provider: "openai-codex", Description: "GPT-4o via Codex", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
}

// GoogleModelsComplete returns all Google Gemini models.
var GoogleModelsComplete = []Model{
	{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash Exp", Provider: "google", Description: "Latest experimental", MaxTokens: 1000000, Capabilities: []string{"vision", "tools"}},
	{ID: "gemini-2.0-flash-thinking-exp-01-21", Name: "Gemini 2.0 Flash Thinking", Provider: "google", Description: "With thinking", MaxTokens: 1000000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Provider: "google", Description: "Most capable", MaxTokens: 2000000, Capabilities: []string{"vision", "tools"}},
	{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Provider: "google", Description: "Fast and efficient", MaxTokens: 1000000, Capabilities: []string{"vision", "tools"}},
	{ID: "gemini-pro", Name: "Gemini Pro", Provider: "google", Description: "Previous generation", MaxTokens: 32000, Capabilities: []string{"tools"}},
}

// BedrockModelsComplete returns AWS Bedrock models.
var BedrockModelsComplete = []Model{
	// Anthropic Claude on Bedrock
	{ID: "anthropic.claude-opus-4-5-20251101-v1:0", Name: "Claude Opus 4.5 (Bedrock)", Provider: "amazon-bedrock", Description: "Latest Opus on Bedrock", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-sonnet-4-5-20250929-v1:0", Name: "Claude Sonnet 4.5 (Bedrock)", Provider: "amazon-bedrock", Description: "Latest Sonnet on Bedrock", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-haiku-4-5-20251001-v1:0", Name: "Claude Haiku 4.5 (Bedrock)", Provider: "amazon-bedrock", Description: "Latest Haiku on Bedrock", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-3-5-sonnet-20241022-v2:0", Name: "Claude 3.5 Sonnet v2 (Bedrock)", Provider: "amazon-bedrock", Description: "Claude 3.5 Sonnet v2", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-3-5-sonnet-20240620-v1:0", Name: "Claude 3.5 Sonnet (Bedrock)", Provider: "amazon-bedrock", Description: "Claude 3.5 Sonnet", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-3-5-haiku-20241022-v1:0", Name: "Claude 3.5 Haiku (Bedrock)", Provider: "amazon-bedrock", Description: "Claude 3.5 Haiku", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-3-opus-20240229-v1:0", Name: "Claude 3 Opus (Bedrock)", Provider: "amazon-bedrock", Description: "Claude 3 Opus", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-3-sonnet-20240229-v1:0", Name: "Claude 3 Sonnet (Bedrock)", Provider: "amazon-bedrock", Description: "Claude 3 Sonnet", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-3-haiku-20240307-v1:0", Name: "Claude 3 Haiku (Bedrock)", Provider: "amazon-bedrock", Description: "Claude 3 Haiku", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},

	// Meta Llama on Bedrock
	{ID: "meta.llama3-70b-instruct-v1:0", Name: "Llama 3 70B (Bedrock)", Provider: "amazon-bedrock", Description: "Meta Llama 3 70B", MaxTokens: 8192, Capabilities: []string{"tools"}},
	{ID: "meta.llama3-8b-instruct-v1:0", Name: "Llama 3 8B (Bedrock)", Provider: "amazon-bedrock", Description: "Meta Llama 3 8B", MaxTokens: 8192, Capabilities: []string{"tools"}},

	// Mistral on Bedrock
	{ID: "mistral.mistral-large-2402-v1:0", Name: "Mistral Large (Bedrock)", Provider: "amazon-bedrock", Description: "Mistral Large", MaxTokens: 32000, Capabilities: []string{"tools"}},
	{ID: "mistral.mixtral-8x7b-instruct-v0:1", Name: "Mixtral 8x7B (Bedrock)", Provider: "amazon-bedrock", Description: "Mixtral 8x7B", MaxTokens: 32000, Capabilities: []string{"tools"}},

	// Cohere on Bedrock
	{ID: "cohere.command-r-v1:0", Name: "Command R (Bedrock)", Provider: "amazon-bedrock", Description: "Cohere Command R", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "cohere.command-r-plus-v1:0", Name: "Command R+ (Bedrock)", Provider: "amazon-bedrock", Description: "Cohere Command R+", MaxTokens: 128000, Capabilities: []string{"tools"}},
}

// XAIModelsComplete returns xAI Grok models.
var XAIModelsComplete = []Model{
	{ID: "grok-beta", Name: "Grok Beta", Provider: "xai", Description: "Grok conversational model", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "grok-vision-beta", Name: "Grok Vision Beta", Provider: "xai", Description: "Grok with vision", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
}

// GroqModelsComplete returns Groq models.
var GroqModelsComplete = []Model{
	{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Provider: "groq", Description: "Ultra-fast inference", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "llama-3.1-70b-versatile", Name: "Llama 3.1 70B", Provider: "groq", Description: "Llama 3.1 70B", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Provider: "groq", Description: "MoE model", MaxTokens: 32768, Capabilities: []string{"tools"}},
	{ID: "gemma-7b-it", Name: "Gemma 7B", Provider: "groq", Description: "Google Gemma", MaxTokens: 8192, Capabilities: []string{"tools"}},
}

// CerebrasModelsComplete returns Cerebras models.
var CerebrasModelsComplete = []Model{
	{ID: "llama3.1-70b", Name: "Llama 3.1 70B", Provider: "cerebras", Description: "Fast Llama on Cerebras", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "llama3.1-8b", Name: "Llama 3.1 8B", Provider: "cerebras", Description: "Fast Llama 8B", MaxTokens: 128000, Capabilities: []string{"tools"}},
}

// MistralModelsComplete returns Mistral AI models.
var MistralModelsComplete = []Model{
	{ID: "mistral-large-latest", Name: "Mistral Large", Provider: "mistral", Description: "Flagship model", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "mistral-medium-latest", Name: "Mistral Medium", Provider: "mistral", Description: "Balanced", MaxTokens: 32000, Capabilities: []string{"tools"}},
	{ID: "mistral-small-latest", Name: "Mistral Small", Provider: "mistral", Description: "Fast and efficient", MaxTokens: 32000, Capabilities: []string{"tools"}},
	{ID: "codestral-latest", Name: "Codestral", Provider: "mistral", Description: "Code-specialized", MaxTokens: 32000, Capabilities: []string{"tools"}},
}

// MiniMaxModelsComplete returns MiniMax models.
var MiniMaxModelsComplete = []Model{
	{ID: "MiniMax-M2.1", Name: "MiniMax M2.1", Provider: "minimax", Description: "Latest MiniMax model", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "MiniMax-Text-01", Name: "MiniMax Text 01", Provider: "minimax", Description: "Text-only model", MaxTokens: 200000, Capabilities: []string{"tools"}},
}

// DeepSeekModelsComplete returns DeepSeek models.
var DeepSeekModelsComplete = []Model{
	{ID: "deepseek-chat", Name: "DeepSeek Chat", Provider: "deepseek", Description: "General purpose", MaxTokens: 64000, Capabilities: []string{"tools"}},
	{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Provider: "deepseek", Description: "Reasoning model", MaxTokens: 64000, Capabilities: []string{"reasoning", "tools"}},
	{ID: "deepseek-coder", Name: "DeepSeek Coder", Provider: "deepseek", Description: "Code-specialized", MaxTokens: 64000, Capabilities: []string{"tools"}},
}

// GitHubCopilotModelsComplete returns GitHub Copilot models.
var GitHubCopilotModelsComplete = []Model{
	{ID: "gpt-4o", Name: "GPT-4o (Copilot)", Provider: "github-copilot", Description: "GPT-4o via Copilot", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4o-mini", Name: "GPT-4o Mini (Copilot)", Provider: "github-copilot", Description: "GPT-4o Mini via Copilot", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-3.5-sonnet", Name: "Claude 3.5 Sonnet (Copilot)", Provider: "github-copilot", Description: "Claude via Copilot", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "o1-preview", Name: "o1 Preview (Copilot)", Provider: "github-copilot", Description: "o1 via Copilot", MaxTokens: 128000, Capabilities: []string{"reasoning"}},
}

// OpenRouterModelsComplete returns OpenRouter models (unified gateway).
var OpenRouterModelsComplete = []Model{
	// Note: OpenRouter supports 100+ models, listing popular ones
	{ID: "anthropic/claude-opus-4", Name: "Claude Opus 4 (OR)", Provider: "openrouter", Description: "Via OpenRouter", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "openai/gpt-4o", Name: "GPT-4o (OR)", Provider: "openrouter", Description: "Via OpenRouter", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "google/gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash (OR)", Provider: "openrouter", Description: "Via OpenRouter", MaxTokens: 1000000, Capabilities: []string{"vision", "tools"}},
	{ID: "meta-llama/llama-3.3-70b-instruct", Name: "Llama 3.3 70B (OR)", Provider: "openrouter", Description: "Via OpenRouter", MaxTokens: 128000, Capabilities: []string{"tools"}},
}

// VercelAIGatewayModelsComplete returns Vercel AI Gateway models.
var VercelAIGatewayModelsComplete = []Model{
	// Vercel AI Gateway proxies to other providers
	{ID: "anthropic/claude-opus-4", Name: "Claude Opus 4 (Vercel)", Provider: "vercel-ai-gateway", Description: "Via Vercel", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "openai/gpt-4o", Name: "GPT-4o (Vercel)", Provider: "vercel-ai-gateway", Description: "Via Vercel", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
}

// ZAIModelsComplete returns Z.AI (GLM) models.
var ZAIModelsComplete = []Model{
	{ID: "glm-4-plus", Name: "GLM-4 Plus", Provider: "zai", Description: "Latest GLM model", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "glm-4", Name: "GLM-4", Provider: "zai", Description: "GLM-4 baseline", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "glm-3-turbo", Name: "GLM-3 Turbo", Provider: "zai", Description: "Fast GLM", MaxTokens: 128000, Capabilities: []string{"tools"}},
}

// OpenCodeModelsComplete returns OpenCode Zen models.
var OpenCodeModelsComplete = []Model{
	{ID: "opencode-zen-1", Name: "OpenCode Zen 1", Provider: "opencode", Description: "OpenCode Zen model", MaxTokens: 128000, Capabilities: []string{"tools"}},
}

// SyntheticModelsComplete returns Synthetic models (Anthropic-compatible).
var SyntheticModelsComplete = []Model{
	{ID: "claude-opus-4", Name: "Claude Opus 4 (Synthetic)", Provider: "synthetic", Description: "Anthropic-compatible", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-sonnet-4", Name: "Claude Sonnet 4 (Synthetic)", Provider: "synthetic", Description: "Anthropic-compatible", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
}

// GetAllModelsComplete returns all models from all providers.
func GetAllModelsComplete() []Model {
	var all []Model
	all = append(all, AnthropicModelsComplete...)
	all = append(all, OpenAIModelsComplete...)
	all = append(all, OpenAICodexModelsComplete...)
	all = append(all, GoogleModelsComplete...)
	all = append(all, BedrockModelsComplete...)
	all = append(all, XAIModelsComplete...)
	all = append(all, GroqModelsComplete...)
	all = append(all, CerebrasModelsComplete...)
	all = append(all, MistralModelsComplete...)
	all = append(all, MiniMaxModelsComplete...)
	all = append(all, DeepSeekModelsComplete...)
	all = append(all, GitHubCopilotModelsComplete...)
	all = append(all, OpenRouterModelsComplete...)
	all = append(all, VercelAIGatewayModelsComplete...)
	all = append(all, ZAIModelsComplete...)
	all = append(all, OpenCodeModelsComplete...)
	all = append(all, SyntheticModelsComplete...)
	return all
}

// GetModelsByProvider returns models for a specific provider.
func GetModelsByProvider(provider string) []Model {
	switch provider {
	case "anthropic":
		return AnthropicModelsComplete
	case "openai":
		return OpenAIModelsComplete
	case "openai-codex":
		return OpenAICodexModelsComplete
	case "google", "google-vertex", "google-antigravity", "google-gemini-cli":
		return GoogleModelsComplete
	case "amazon-bedrock":
		return BedrockModelsComplete
	case "xai":
		return XAIModelsComplete
	case "groq":
		return GroqModelsComplete
	case "cerebras":
		return CerebrasModelsComplete
	case "mistral":
		return MistralModelsComplete
	case "minimax", "minimax-cn":
		return MiniMaxModelsComplete
	case "deepseek":
		return DeepSeekModelsComplete
	case "github-copilot":
		return GitHubCopilotModelsComplete
	case "openrouter":
		return OpenRouterModelsComplete
	case "vercel-ai-gateway":
		return VercelAIGatewayModelsComplete
	case "zai":
		return ZAIModelsComplete
	case "opencode":
		return OpenCodeModelsComplete
	case "synthetic":
		return SyntheticModelsComplete
	default:
		return []Model{}
	}
}
