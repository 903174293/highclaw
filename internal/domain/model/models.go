package model

// Note: AllProviders is now defined in providers_complete.go

// AnthropicModels returns all Anthropic Claude models.
var AnthropicModels = []Model{
	{ID: "claude-opus-4", Name: "Claude Opus 4", Provider: "anthropic", Description: "Most capable model", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-opus-4-6", Name: "Claude Opus 4.6", Provider: "anthropic", Description: "Latest Opus", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Provider: "anthropic", Description: "Balanced performance", MaxTokens: 200000, Capabilities: []string{"vision", "tools", "thinking"}},
	{ID: "claude-sonnet-3-5", Name: "Claude Sonnet 3.5", Provider: "anthropic", Description: "Previous generation", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-haiku-4", Name: "Claude Haiku 4", Provider: "anthropic", Description: "Fast and efficient", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "claude-haiku-3-5", Name: "Claude Haiku 3.5", Provider: "anthropic", Description: "Previous Haiku", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
}

// OpenAIModels returns all OpenAI models.
var OpenAIModels = []Model{
	{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Description: "Multimodal flagship", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", Description: "Affordable and fast", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", Description: "Previous flagship", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4", Name: "GPT-4", Provider: "openai", Description: "Original GPT-4", MaxTokens: 8192, Capabilities: []string{"tools"}},
	{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: "openai", Description: "Fast and affordable", MaxTokens: 16385, Capabilities: []string{"tools"}},
	{ID: "o1", Name: "o1", Provider: "openai", Description: "Reasoning model", MaxTokens: 200000, Capabilities: []string{"reasoning"}},
	{ID: "o1-mini", Name: "o1 Mini", Provider: "openai", Description: "Faster reasoning", MaxTokens: 128000, Capabilities: []string{"reasoning"}},
	{ID: "o3-mini", Name: "o3 Mini", Provider: "openai", Description: "Latest reasoning", MaxTokens: 200000, Capabilities: []string{"reasoning"}},
}

// GoogleModels returns all Google Gemini models.
var GoogleModels = []Model{
	{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash", Provider: "google", Description: "Latest experimental", MaxTokens: 1000000, Capabilities: []string{"vision", "tools"}},
	{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Provider: "google", Description: "Most capable", MaxTokens: 2000000, Capabilities: []string{"vision", "tools"}},
	{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Provider: "google", Description: "Fast and efficient", MaxTokens: 1000000, Capabilities: []string{"vision", "tools"}},
	{ID: "gemini-pro", Name: "Gemini Pro", Provider: "google", Description: "Previous generation", MaxTokens: 32000, Capabilities: []string{"tools"}},
}

// BedrockModels returns AWS Bedrock models.
var BedrockModels = []Model{
	{ID: "anthropic.claude-opus-4", Name: "Claude Opus 4 (Bedrock)", Provider: "bedrock", Description: "Via AWS Bedrock", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "anthropic.claude-sonnet-4", Name: "Claude Sonnet 4 (Bedrock)", Provider: "bedrock", Description: "Via AWS Bedrock", MaxTokens: 200000, Capabilities: []string{"vision", "tools"}},
	{ID: "meta.llama3-70b", Name: "Llama 3 70B", Provider: "bedrock", Description: "Meta's Llama", MaxTokens: 8192, Capabilities: []string{"tools"}},
	{ID: "mistral.mistral-large", Name: "Mistral Large", Provider: "bedrock", Description: "Mistral flagship", MaxTokens: 32000, Capabilities: []string{"tools"}},
}

// AzureModels returns Azure OpenAI models.
var AzureModels = []Model{
	{ID: "gpt-4o", Name: "GPT-4o (Azure)", Provider: "azure", Description: "Via Azure OpenAI", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-4-turbo", Name: "GPT-4 Turbo (Azure)", Provider: "azure", Description: "Via Azure OpenAI", MaxTokens: 128000, Capabilities: []string{"vision", "tools"}},
	{ID: "gpt-35-turbo", Name: "GPT-3.5 Turbo (Azure)", Provider: "azure", Description: "Via Azure OpenAI", MaxTokens: 16385, Capabilities: []string{"tools"}},
}

// OllamaModels returns common Ollama models.
var OllamaModels = []Model{
	{ID: "llama3.3:70b", Name: "Llama 3.3 70B", Provider: "ollama", Description: "Local Llama", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "qwen2.5:72b", Name: "Qwen 2.5 72B", Provider: "ollama", Description: "Alibaba's Qwen", MaxTokens: 32000, Capabilities: []string{"tools"}},
	{ID: "deepseek-r1:70b", Name: "DeepSeek R1 70B", Provider: "ollama", Description: "Reasoning model", MaxTokens: 64000, Capabilities: []string{"reasoning", "tools"}},
	{ID: "mistral:7b", Name: "Mistral 7B", Provider: "ollama", Description: "Efficient local model", MaxTokens: 32000, Capabilities: []string{"tools"}},
}

// GroqModels returns Groq models.
var GroqModels = []Model{
	{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Provider: "groq", Description: "Ultra-fast inference", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Provider: "groq", Description: "MoE model", MaxTokens: 32768, Capabilities: []string{"tools"}},
}

// TogetherModels returns Together AI models.
var TogetherModels = []Model{
	{ID: "meta-llama/Llama-3.3-70B-Instruct-Turbo", Name: "Llama 3.3 70B Turbo", Provider: "together", Description: "Fast Llama", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "Qwen/Qwen2.5-72B-Instruct-Turbo", Name: "Qwen 2.5 72B Turbo", Provider: "together", Description: "Fast Qwen", MaxTokens: 32000, Capabilities: []string{"tools"}},
}

// FireworksModels returns Fireworks AI models.
var FireworksModels = []Model{
	{ID: "accounts/fireworks/models/llama-v3p3-70b-instruct", Name: "Llama 3.3 70B", Provider: "fireworks", Description: "Via Fireworks", MaxTokens: 128000, Capabilities: []string{"tools"}},
}

// CohereModels returns Cohere models.
var CohereModels = []Model{
	{ID: "command-r-plus", Name: "Command R+", Provider: "cohere", Description: "Most capable", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "command-r", Name: "Command R", Provider: "cohere", Description: "Balanced", MaxTokens: 128000, Capabilities: []string{"tools"}},
}

// MistralModels returns Mistral AI models.
var MistralModels = []Model{
	{ID: "mistral-large-latest", Name: "Mistral Large", Provider: "mistral", Description: "Flagship model", MaxTokens: 128000, Capabilities: []string{"tools"}},
	{ID: "mistral-medium-latest", Name: "Mistral Medium", Provider: "mistral", Description: "Balanced", MaxTokens: 32000, Capabilities: []string{"tools"}},
}

// PerplexityModels returns Perplexity models.
var PerplexityModels = []Model{
	{ID: "llama-3.1-sonar-large-128k-online", Name: "Sonar Large Online", Provider: "perplexity", Description: "With web search", MaxTokens: 128000, Capabilities: []string{"tools", "search"}},
	{ID: "llama-3.1-sonar-small-128k-online", Name: "Sonar Small Online", Provider: "perplexity", Description: "Fast with search", MaxTokens: 128000, Capabilities: []string{"tools", "search"}},
}

// DeepSeekModels returns DeepSeek models.
var DeepSeekModels = []Model{
	{ID: "deepseek-chat", Name: "DeepSeek Chat", Provider: "deepseek", Description: "General purpose", MaxTokens: 64000, Capabilities: []string{"tools"}},
	{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Provider: "deepseek", Description: "Reasoning model", MaxTokens: 64000, Capabilities: []string{"reasoning", "tools"}},
}

// GetAllModels returns all models from all providers.
func GetAllModels() []Model {
	var all []Model
	all = append(all, AnthropicModels...)
	all = append(all, OpenAIModels...)
	all = append(all, GoogleModels...)
	all = append(all, BedrockModels...)
	all = append(all, AzureModels...)
	all = append(all, OllamaModels...)
	all = append(all, GroqModels...)
	all = append(all, TogetherModels...)
	all = append(all, FireworksModels...)
	all = append(all, CohereModels...)
	all = append(all, MistralModels...)
	all = append(all, PerplexityModels...)
	all = append(all, DeepSeekModels...)
	return all
}
