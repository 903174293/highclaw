// Package agent implements the AI agent runtime.
// The agent runs Pi agent sessions, manages model API calls,
// handles tool execution, and manages skills.
package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/highclaw/highclaw/internal/agent/providers"
	"github.com/highclaw/highclaw/internal/agent/tools"
	"github.com/highclaw/highclaw/internal/config"
)

// Runner manages the agent execution loop.
type Runner struct {
	cfg    *config.Config
	logger *slog.Logger
	models *ModelManager
	tools  *ToolRegistry
}

// NewRunner creates a new agent runner.
func NewRunner(cfg *config.Config, logger *slog.Logger) *Runner {
	return &Runner{
		cfg:    cfg,
		logger: logger.With("component", "agent"),
		models: NewModelManager(cfg, logger),
		tools:  NewToolRegistry(),
	}
}

// RunRequest contains the inputs for an agent run.
type RunRequest struct {
	SessionKey   string
	Channel      string
	Message      string
	Images       [][]byte
	AgentID      string
	SystemPrompt string
}

// RunResult contains the outputs of an agent run.
type RunResult struct {
	Reply      string
	ToolCalls  []ToolCall
	TokensUsed TokenUsage
}

// ToolCall describes a tool invocation during an agent run.
type ToolCall struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Output string `json:"output"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	CacheRead    int `json:"cacheRead"`
	CacheWrite   int `json:"cacheWrite"`
}

// Run executes an agent session — send message, get response, execute tools.
func (r *Runner) Run(ctx context.Context, req *RunRequest) (*RunResult, error) {
	r.logger.Info("agent run",
		"session", req.SessionKey,
		"channel", req.Channel,
		"message_len", len(req.Message),
	)

	// 1. Build system prompt.
	systemPrompt := r.buildSystemPrompt(req)

	// 2. Call model.
	modelResp, err := r.models.Chat(ctx, &ChatRequest{
		SystemPrompt: systemPrompt,
		Messages: []ChatMessage{
			{Role: "user", Content: req.Message},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("model call failed: %w", err)
	}

	return &RunResult{
		Reply:      modelResp.Content,
		TokensUsed: modelResp.Usage,
	}, nil
}

// buildSystemPrompt constructs the system prompt from config, skills, and context.
func (r *Runner) buildSystemPrompt(req *RunRequest) string {
	// TODO: Load AGENTS.md, SOUL.md, skills, tool descriptions.
	prompt := "You are HighClaw, a personal AI assistant."
	if req.SystemPrompt != "" {
		prompt = req.SystemPrompt
	}
	return prompt
}

// ModelManager handles model provider selection and API calls.
type ModelManager struct {
	cfg    *config.Config
	logger *slog.Logger
}

// NewModelManager creates a new model manager.
func NewModelManager(cfg *config.Config, logger *slog.Logger) *ModelManager {
	return &ModelManager{
		cfg:    cfg,
		logger: logger.With("component", "models"),
	}
}

// ChatRequest represents a request to a model provider.
type ChatRequest struct {
	SystemPrompt  string
	Messages      []ChatMessage
	Model         string
	MaxTokens     int
	Temperature   float64
	ThinkingLevel string
}

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse contains the model's response.
type ChatResponse struct {
	Content string
	Usage   TokenUsage
}

// Chat sends a request to the configured model provider.
func (m *ModelManager) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = m.cfg.Agent.Model
	}

	m.logger.Info("calling model", "model", model)

	// Determine provider from model string (e.g., "anthropic/claude-opus-4").
	provider, modelName := parseModelString(model)

	switch provider {
	case "anthropic":
		return m.callAnthropic(ctx, req, modelName)
	case "openai":
		return m.callOpenAI(ctx, req, modelName)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// callAnthropic calls the Anthropic Claude API.
func (m *ModelManager) callAnthropic(ctx context.Context, req *ChatRequest, model string) (*ChatResponse, error) {
	// Get API key from config.
	providerCfg, ok := m.cfg.Agent.Providers["anthropic"]
	if !ok || providerCfg.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key not configured")
	}

	client := providers.NewAnthropicClient(providerCfg.APIKey)

	// Convert our ChatRequest to Anthropic format.
	messages := make([]providers.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, providers.Message{
			Role: msg.Role,
			Content: []providers.ContentBlock{
				{Type: "text", Text: msg.Content},
			},
		})
	}

	anthropicReq := &providers.ChatRequest{
		Model:         model,
		MaxTokens:     req.MaxTokens,
		Messages:      messages,
		System:        req.SystemPrompt,
		Temperature:   req.Temperature,
		ThinkingLevel: req.ThinkingLevel,
	}

	resp, err := client.Chat(ctx, anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}

	// Extract text content from response.
	content := providers.ExtractTextContent(resp.Content)

	return &ChatResponse{
		Content: content,
		Usage: TokenUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			CacheRead:    resp.Usage.CacheRead,
			CacheWrite:   resp.Usage.CacheWrite,
		},
	}, nil
}

// callOpenAI calls the OpenAI Chat Completions API.
func (m *ModelManager) callOpenAI(ctx context.Context, req *ChatRequest, model string) (*ChatResponse, error) {
	providerCfg, ok := m.cfg.Agent.Providers["openai"]
	if !ok || providerCfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	client := providers.NewOpenAIClient(providerCfg.APIKey)

	// Build messages: system prompt + conversation messages.
	messages := make([]providers.OpenAIMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, providers.OpenAIMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	for _, msg := range req.Messages {
		messages = append(messages, providers.OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	openaiReq := &providers.OpenAIChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	resp, err := client.Chat(ctx, openaiReq)
	if err != nil {
		return nil, fmt.Errorf("openai API call: %w", err)
	}

	// Extract content from the first choice.
	var content string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Content: content,
		Usage: TokenUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}, nil
}

// parseModelString splits "provider/model" into components.
// Examples: "anthropic/claude-opus-4" -> ("anthropic", "claude-opus-4")
func parseModelString(model string) (provider, modelName string) {
	for i, ch := range model {
		if ch == '/' {
			return model[:i], model[i+1:]
		}
	}
	// Default to anthropic if no provider specified.
	return "anthropic", model
}

// ToolRegistry manages available tools.
type ToolRegistry struct {
	tools map[string]ToolHandler
}

// ToolHandler is the function signature for tool implementations.
type ToolHandler func(ctx context.Context, input string) (string, error)

// NewToolRegistry creates a new tool registry with built-in tools.
func NewToolRegistry() *ToolRegistry {
	reg := &ToolRegistry{
		tools: make(map[string]ToolHandler),
	}

	// Register built-in tools.
	reg.Register("bash", tools.Bash)
	reg.Register("bash_process", tools.BashProcessTool)
	reg.Register("web_search", tools.WebSearch)

	return reg
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(name string, handler ToolHandler) {
	r.tools[name] = handler
}

// Execute runs a tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name, input string) (string, error) {
	handler, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return handler(ctx, input)
}
