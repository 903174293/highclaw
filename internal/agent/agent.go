// Package agent implements the AI agent runtime.
// The agent runs Pi agent sessions, manages model API calls,
// handles tool execution, and manages skills.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/highclaw/highclaw/internal/agent/providers"
	"github.com/highclaw/highclaw/internal/agent/tools"
	"github.com/highclaw/highclaw/internal/config"
)

const maxToolIterations = 8

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
		tools:  NewToolRegistry(cfg),
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

	// 2. Run ZeroClaw-style tool loop.
	history := []ChatMessage{{Role: "user", Content: req.Message}}
	var totalUsage TokenUsage

	for i := 0; i < maxToolIterations; i++ {
		modelResp, err := r.models.Chat(ctx, &ChatRequest{
			SystemPrompt: systemPrompt,
			Messages:     history,
		})
		if err != nil {
			return nil, fmt.Errorf("model call failed: %w", err)
		}
		totalUsage.merge(modelResp.Usage)

		text, calls := parseToolCalls(modelResp.Content)
		if len(calls) == 0 {
			reply := strings.TrimSpace(text)
			if reply == "" {
				reply = strings.TrimSpace(modelResp.Content)
			}
			return &RunResult{
				Reply:      reply,
				TokensUsed: totalUsage,
			}, nil
		}

		var toolResults strings.Builder
		for _, call := range calls {
			output, err := r.tools.ExecuteJSON(ctx, call.Name, call.Arguments)
			if err != nil {
				output = "Error: " + err.Error()
			}
			fmt.Fprintf(&toolResults, "<tool_result name=\"%s\">\n%s\n</tool_result>\n", call.Name, output)
		}

		history = append(history, ChatMessage{Role: "assistant", Content: modelResp.Content})
		history = append(history, ChatMessage{
			Role:    "user",
			Content: "[Tool results]\n" + toolResults.String(),
		})
	}

	return nil, fmt.Errorf("agent exceeded maximum tool iterations (%d)", maxToolIterations)
}

// buildSystemPrompt constructs the system prompt from config, skills, and context.
func (r *Runner) buildSystemPrompt(req *RunRequest) string {
	var b strings.Builder
	b.WriteString("You are HighClaw, a personal AI assistant.\n\n")
	b.WriteString("## Tool Use Protocol\n\n")
	b.WriteString("To use a tool, return:\n")
	b.WriteString("<tool_call>{\"name\":\"tool_name\",\"arguments\":{...}}</tool_call>\n\n")
	b.WriteString("Available tools:\n")
	for _, spec := range r.tools.Specs() {
		fmt.Fprintf(&b, "- %s: %s\n", spec.Name, spec.Description)
	}
	prompt := b.String()
	if req.SystemPrompt != "" {
		prompt = req.SystemPrompt
	}
	return prompt
}

// ModelManager handles model provider selection and API calls.
type ModelManager struct {
	cfg     *config.Config
	logger  *slog.Logger
	factory *ProviderFactory
}

// NewModelManager creates a new model manager.
func NewModelManager(cfg *config.Config, logger *slog.Logger) *ModelManager {
	return &ModelManager{
		cfg:     cfg,
		logger:  logger.With("component", "models"),
		factory: NewProviderFactory(),
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

	// Determine provider from model string (e.g., "anthropic/claude-opus-4"),
	// aligned with ZeroClaw provider-factory style.
	provider, modelName := parseModelString(model)
	m.logger.Info("calling model", "provider", provider, "model", modelName)

	p, err := m.factory.Create(provider, m.cfg)
	if err != nil {
		return nil, err
	}

	return p.Chat(ctx, req, modelName)
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

// Provider is the unified provider interface used by ModelManager.
type Provider interface {
	Chat(ctx context.Context, req *ChatRequest, model string) (*ChatResponse, error)
}

// ProviderBuilder constructs a provider instance from config.
type ProviderBuilder func(cfg *config.Config) (Provider, error)

// ProviderFactory is a provider registry+factory, mirroring ZeroClaw's pattern.
type ProviderFactory struct {
	builders map[string]ProviderBuilder
}

// NewProviderFactory creates a factory with built-in providers registered.
func NewProviderFactory() *ProviderFactory {
	f := &ProviderFactory{builders: map[string]ProviderBuilder{}}
	f.Register("anthropic", func(cfg *config.Config) (Provider, error) {
		pcfg, ok := cfg.Agent.Providers["anthropic"]
		if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
			return nil, fmt.Errorf("anthropic API key not configured")
		}
		return &anthropicProvider{client: providers.NewAnthropicClient(pcfg.APIKey)}, nil
	})
	f.Register("openai", func(cfg *config.Config) (Provider, error) {
		pcfg, ok := cfg.Agent.Providers["openai"]
		if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
			return nil, fmt.Errorf("openai API key not configured")
		}
		return &openAIProvider{client: providers.NewOpenAIClient(pcfg.APIKey)}, nil
	})
	return f
}

// Register registers a provider builder.
func (f *ProviderFactory) Register(name string, builder ProviderBuilder) {
	f.builders[name] = builder
}

// Create instantiates a provider by name.
func (f *ProviderFactory) Create(name string, cfg *config.Config) (Provider, error) {
	builder, ok := f.builders[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return builder(cfg)
}

type anthropicProvider struct {
	client *providers.AnthropicClient
}

func (p *anthropicProvider) Chat(ctx context.Context, req *ChatRequest, model string) (*ChatResponse, error) {
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
	resp, err := p.client.Chat(ctx, anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}
	return &ChatResponse{
		Content: providers.ExtractTextContent(resp.Content),
		Usage: TokenUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			CacheRead:    resp.Usage.CacheRead,
			CacheWrite:   resp.Usage.CacheWrite,
		},
	}, nil
}

type openAIProvider struct {
	client *providers.OpenAIClient
}

func (p *openAIProvider) Chat(ctx context.Context, req *ChatRequest, model string) (*ChatResponse, error) {
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
	openAIReq := &providers.OpenAIChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	resp, err := p.client.Chat(ctx, openAIReq)
	if err != nil {
		return nil, fmt.Errorf("openai API call: %w", err)
	}
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

type ParsedToolCall struct {
	Name      string
	Arguments json.RawMessage
}

// parseToolCalls supports:
// 1) XML-style: <tool_call>{"name":"bash","arguments":{...}}</tool_call>
// 2) OpenAI-style JSON with tool_calls array.
func parseToolCalls(response string) (string, []ParsedToolCall) {
	textParts := make([]string, 0, 2)
	calls := make([]ParsedToolCall, 0)
	trimmed := strings.TrimSpace(response)

	// Try OpenAI-style JSON first.
	var root map[string]any
	if err := json.Unmarshal([]byte(trimmed), &root); err == nil {
		if tcRaw, ok := root["tool_calls"].([]any); ok {
			for _, item := range tcRaw {
				call := parseToolCallFromAny(item)
				if call != nil {
					calls = append(calls, *call)
				}
			}
			if len(calls) > 0 {
				if c, _ := root["content"].(string); strings.TrimSpace(c) != "" {
					textParts = append(textParts, strings.TrimSpace(c))
				}
				return strings.Join(textParts, "\n"), calls
			}
		}
	}

	remaining := response
	for {
		start := strings.Index(remaining, "<tool_call>")
		if start == -1 {
			break
		}
		before := strings.TrimSpace(remaining[:start])
		if before != "" {
			textParts = append(textParts, before)
		}
		rest := remaining[start+len("<tool_call>"):]
		end := strings.Index(rest, "</tool_call>")
		if end == -1 {
			// malformed tag; keep remaining as text
			remaining = rest
			break
		}
		body := strings.TrimSpace(rest[:end])
		call := parseToolCallJSON(body)
		if call != nil {
			calls = append(calls, *call)
		}
		remaining = rest[end+len("</tool_call>"):]
	}

	after := strings.TrimSpace(remaining)
	if after != "" {
		textParts = append(textParts, after)
	}

	if len(calls) == 0 {
		// Fallback: whole response as single tool json.
		call := parseToolCallJSON(trimmed)
		if call != nil {
			calls = append(calls, *call)
			return "", calls
		}
	}

	return strings.Join(textParts, "\n"), calls
}

func parseToolCallFromAny(v any) *ParsedToolCall {
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	// OpenAI function format.
	if fn, ok := obj["function"].(map[string]any); ok {
		name, _ := fn["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil
		}
		args := fn["arguments"]
		switch t := args.(type) {
		case string:
			return &ParsedToolCall{Name: name, Arguments: json.RawMessage(t)}
		default:
			b, _ := json.Marshal(t)
			return &ParsedToolCall{Name: name, Arguments: b}
		}
	}
	return parseToolCallMap(obj)
}

func parseToolCallJSON(raw string) *ParsedToolCall {
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil
	}
	return parseToolCallMap(obj)
}

func parseToolCallMap(obj map[string]any) *ParsedToolCall {
	name, _ := obj["name"].(string)
	if strings.TrimSpace(name) == "" {
		return nil
	}
	argsRaw, ok := obj["arguments"]
	if !ok {
		return &ParsedToolCall{Name: name, Arguments: json.RawMessage(`{}`)}
	}
	switch t := argsRaw.(type) {
	case string:
		return &ParsedToolCall{Name: name, Arguments: json.RawMessage(t)}
	default:
		b, _ := json.Marshal(t)
		return &ParsedToolCall{Name: name, Arguments: b}
	}
}

// ToolRegistry manages available tools.
type ToolRegistry struct {
	tools  map[string]ToolSpec
	policy *SecurityPolicy
}

// ToolHandler is the function signature for tool implementations.
type ToolHandler func(ctx context.Context, input string) (string, error)

// ToolSpec contains metadata+handler for prompt construction and execution.
type ToolSpec struct {
	Name        string
	Description string
	Handler     ToolHandler
}

// NewToolRegistry creates a new tool registry with built-in tools.
func NewToolRegistry(cfg *config.Config) *ToolRegistry {
	reg := &ToolRegistry{
		tools:  make(map[string]ToolSpec),
		policy: NewSecurityPolicy(cfg),
	}

	// Register built-in tools.
	reg.Register("bash", "Execute shell commands in the local runtime", reg.securedBashTool())
	reg.Register("bash_process", "Manage long-running shell processes", tools.BashProcessTool)
	reg.Register("web_search", "Search the web and return summarized results", tools.WebSearch)

	return reg
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(name, description string, handler ToolHandler) {
	r.tools[name] = ToolSpec{
		Name:        name,
		Description: description,
		Handler:     handler,
	}
}

// Specs returns all registered tool specs.
func (r *ToolRegistry) Specs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(r.tools))
	for _, s := range r.tools {
		specs = append(specs, s)
	}
	return specs
}

// Execute runs a tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name, input string) (string, error) {
	spec, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return spec.Handler(ctx, input)
}

// ExecuteJSON runs a tool with JSON arguments.
func (r *ToolRegistry) ExecuteJSON(ctx context.Context, name string, args json.RawMessage) (string, error) {
	input := strings.TrimSpace(string(args))
	if input == "" {
		input = "{}"
	}
	return r.Execute(ctx, name, input)
}

func (u *TokenUsage) merge(other TokenUsage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheRead += other.CacheRead
	u.CacheWrite += other.CacheWrite
}

func (r *ToolRegistry) securedBashTool() ToolHandler {
	return func(ctx context.Context, input string) (string, error) {
		if err := r.policy.ValidateBashInput(input); err != nil {
			return "", err
		}
		return tools.Bash(ctx, input)
	}
}
