// Package agent implements the AI agent runtime.
// The agent runs Pi agent sessions, manages model API calls,
// handles tool execution, and manages skills.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/highclaw/highclaw/internal/agent/providers"
	"github.com/highclaw/highclaw/internal/agent/tools"
	"github.com/highclaw/highclaw/internal/config"
)

const maxToolIterations = 10

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
		tools:  NewToolRegistry(cfg, logger),
	}
}

// RunRequest contains the inputs for an agent run.
type RunRequest struct {
	SessionKey   string
	Channel      string
	Message      string
	History      []ChatMessage
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
	r.logger.Debug("agent run",
		"session", req.SessionKey,
		"channel", req.Channel,
		"message_len", len(req.Message),
	)

	// 1. Build system prompt.
	systemPrompt := r.buildSystemPrompt(req)

	// 2. Run ZeroClaw-style tool loop.
	history := make([]ChatMessage, 0, len(req.History)+1)
	if len(req.History) > 0 {
		for _, msg := range req.History {
			role := strings.TrimSpace(msg.Role)
			if role == "" {
				continue
			}
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				continue
			}
			history = append(history, ChatMessage{Role: role, Content: content})
		}
	}
	if len(history) == 0 {
		history = append(history, ChatMessage{Role: "user", Content: req.Message})
	}
	var totalUsage TokenUsage

	for i := 0; i < maxToolIterations; i++ {
		modelStart := time.Now()
		modelResp, err := r.models.Chat(ctx, &ChatRequest{
			SystemPrompt: systemPrompt,
			Messages:     history,
			MaxTokens:    1200,
		})
		if err != nil {
			return nil, fmt.Errorf("model call failed: %w", err)
		}
		modelLatency := time.Since(modelStart)
		totalUsage.merge(modelResp.Usage)

		text, toolCalls := parseToolCalls(modelResp.Content)
		r.logger.Debug("model response",
			"iteration", i+1,
			"latency_ms", modelLatency.Milliseconds(),
			"resp_len", len(modelResp.Content),
			"tool_calls_total", len(toolCalls),
		)
		if len(toolCalls) == 0 {
			reply := strings.TrimSpace(text)
			if reply == "" {
				reply = strings.TrimSpace(modelResp.Content)
			}
			return &RunResult{
				Reply:      reply,
				TokensUsed: totalUsage,
			}, nil
		}

		// Match ZeroClaw interactive behavior: print text produced alongside tool calls.
		if strings.TrimSpace(text) != "" {
			fmt.Print(text)
		}

		var toolResults strings.Builder
		for _, call := range toolCalls {
			toolStart := time.Now()
			output := ""
			if r.tools.Has(call.Name) {
				out, err := r.tools.ExecuteJSON(ctx, call.Name, call.Arguments)
				if err != nil {
					output = "Error: " + err.Error()
				} else {
					output = out
				}
			} else {
				output = "Unknown tool: " + call.Name
			}
			r.logger.Debug("tool executed",
				"tool", call.Name,
				"latency_ms", time.Since(toolStart).Milliseconds(),
				"output_len", len(output),
			)
			fmt.Fprintf(&toolResults, "<tool_result>\n%s\n</tool_result>\n", output)
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
	b.WriteString("To use a tool, wrap a JSON object in <invoke></invoke> tags (preferred), or <tool_call></tool_call>:\n\n")
	b.WriteString("```\n<tool_call>\n{\"name\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}\n</tool_call>\n```\n\n")
	b.WriteString("```\n<invoke>\n{\"name\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}\n</invoke>\n```\n\n")
	b.WriteString("You may use multiple tool calls in a single response. ")
	b.WriteString("After tool execution, results appear in <tool_result> tags. ")
	b.WriteString("Continue reasoning with the results until you can give a final answer.\n\n")
	b.WriteString("### Available Tools\n\n")
	for _, spec := range r.tools.Specs() {
		fmt.Fprintf(&b, "**%s**: %s\nParameters: `%s`\n\n", spec.Name, spec.Description, spec.Parameters)
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
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1200
	}
	if req.Temperature == 0 {
		// Align with ZeroClaw default_temperature.
		req.Temperature = 0.7
	}

	// Determine provider from model string (e.g., "anthropic/claude-opus-4"),
	// aligned with ZeroClaw provider-factory style.
	provider, modelName := parseModelString(model)
	m.logger.Debug("calling model", "provider", provider, "model", modelName)

	p, err := m.factory.Create(provider, m.cfg)
	if err != nil {
		return nil, err
	}
	const maxAttempts = 3
	attemptErrors := make([]string, 0, maxAttempts)
	for i := 1; i <= maxAttempts; i++ {
		resp, err := p.Chat(ctx, req, modelName)
		if err == nil {
			return resp, nil
		}
		attemptErrors = append(attemptErrors, fmt.Sprintf(
			"%s attempt %d/%d: %s",
			provider, i, maxAttempts, formatProviderError(provider, err),
		))
		if isNonRetryableProviderError(err) {
			m.logger.Warn("Non-retryable error, switching provider", "provider", provider)
			break
		}
	}
	m.logger.Warn("Switching to fallback provider", "provider", provider)
	return nil, fmt.Errorf("All providers failed. Attempts:\n%s", strings.Join(attemptErrors, "\n"))
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
		return &openAIProvider{client: providers.NewOpenAIClientWithBaseURL(pcfg.APIKey, pcfg.BaseURL)}, nil
	})
	f.Register("openrouter", func(cfg *config.Config) (Provider, error) {
		pcfg, ok := cfg.Agent.Providers["openrouter"]
		if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
			return nil, fmt.Errorf("openrouter API key not configured")
		}
		baseURL := strings.TrimSpace(pcfg.BaseURL)
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		return &openAIProvider{client: providers.NewOpenAIClientWithBaseURL(pcfg.APIKey, baseURL)}, nil
	})
	registerOpenAICompatProviders(f,
		"venice", "deepseek", "mistral", "xai", "perplexity", "groq",
		"fireworks", "together", "cohere", "moonshot", "glm", "zhipu",
		"zai", "z.ai", "minimax", "qianfan", "vercel", "cloudflare",
		"opencode", "synthetic", "gemini",
	)
	return f
}

func registerOpenAICompatProviders(f *ProviderFactory, names ...string) {
	for _, name := range names {
		providerName := name
		f.Register(providerName, func(cfg *config.Config) (Provider, error) {
			pcfg, ok := cfg.Agent.Providers[providerName]
			if !ok {
				// Alias fallback (z.ai <-> zai, zhipu <-> glm)
				switch providerName {
				case "z.ai":
					pcfg, ok = cfg.Agent.Providers["zai"]
				case "zai":
					pcfg, ok = cfg.Agent.Providers["z.ai"]
				case "zhipu":
					pcfg, ok = cfg.Agent.Providers["glm"]
				case "glm":
					pcfg, ok = cfg.Agent.Providers["zhipu"]
				}
			}
			if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
				return nil, fmt.Errorf("%s API key not configured", providerName)
			}
			baseURL := strings.TrimSpace(pcfg.BaseURL)
			if baseURL == "" {
				baseURL = defaultBaseURLForProvider(providerName)
			}
			if strings.TrimSpace(baseURL) == "" {
				return nil, fmt.Errorf("%s base URL not configured", providerName)
			}
			return &openAIProvider{client: providers.NewOpenAIClientWithBaseURL(pcfg.APIKey, baseURL)}, nil
		})
	}
}

func defaultBaseURLForProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "mistral":
		return "https://api.mistral.ai/v1"
	case "together":
		return "https://api.together.xyz/v1"
	case "fireworks":
		return "https://api.fireworks.ai/inference/v1"
	case "cohere":
		return "https://api.cohere.com/compatibility/v1"
	case "moonshot":
		return "https://api.moonshot.cn/v1"
	case "glm", "zhipu", "zai", "z.ai":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "xai":
		return "https://api.x.ai/v1"
	case "perplexity":
		return "https://api.perplexity.ai"
	case "vercel":
		return "https://ai-gateway.vercel.sh/v1"
	case "cloudflare":
		return "https://api.cloudflare.com/client/v4/accounts"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	return ""
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
		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) > 0 {
			// Mirror ZeroClaw behavior: preserve full assistant message JSON so
			// parseToolCalls can decode OpenAI-style tool_calls reliably.
			b, err := json.Marshal(msg)
			if err == nil {
				content = string(b)
			} else {
				content = openAIMessageContentString(msg.Content)
			}
		} else {
			content = openAIMessageContentString(msg.Content)
		}
	}
	return &ChatResponse{
		Content: content,
		Usage: TokenUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}, nil
}

func openAIMessageContentString(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

type ParsedToolCall struct {
	Name      string
	Arguments json.RawMessage
}

// parseToolCalls supports:
// 1) XML-style: <invoke>{"name":"shell","arguments":{...}}</invoke>
// 2) XML-style: <tool_call>{"name":"bash","arguments":{...}}</tool_call>
// 3) OpenAI-style JSON with tool_calls array.
func parseToolCalls(response string) (string, []ParsedToolCall) {
	textParts := make([]string, 0, 2)
	calls := make([]ParsedToolCall, 0)
	trimmed := strings.TrimSpace(response)

	// Try OpenAI-style JSON only when the whole response is JSON.
	var direct any
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &direct); err == nil {
			extracted := parseToolCallsFromAny(direct)
			if len(extracted) > 0 {
				calls = append(calls, extracted...)
				if root, ok := direct.(map[string]any); ok {
					if c, _ := root["content"].(string); strings.TrimSpace(c) != "" {
						textParts = append(textParts, strings.TrimSpace(c))
					}
				}
				return strings.Join(textParts, "\n"), calls
			}
		}
	}

	remaining := response
	// Parse <invoke> tags first (ZeroClaw-style).
	for {
		start := strings.Index(remaining, "<invoke>")
		if start == -1 {
			break
		}
		before := strings.TrimSpace(remaining[:start])
		if before != "" {
			textParts = append(textParts, before)
		}
		rest := remaining[start+len("<invoke>"):]
		end := strings.Index(rest, "</invoke>")
		if end == -1 {
			remaining = rest
			break
		}
		body := strings.TrimSpace(rest[:end])
		parsedAny := false
		for _, value := range extractJSONValues(body) {
			extracted := parseToolCallsFromAny(value)
			if len(extracted) > 0 {
				parsedAny = true
				calls = append(calls, extracted...)
			}
		}
		if !parsedAny {
			call := parseToolCallJSON(body)
			if call != nil {
				calls = append(calls, *call)
			}
		}
		remaining = rest[end+len("</invoke>"):]
	}

	// Parse <tool_call> tags as fallback/compatibility.
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
		parsedAny := false
		for _, value := range extractJSONValues(body) {
			extracted := parseToolCallsFromAny(value)
			if len(extracted) > 0 {
				parsedAny = true
				calls = append(calls, extracted...)
			}
		}
		if !parsedAny {
			call := parseToolCallJSON(body)
			if call != nil {
				calls = append(calls, *call)
			}
		}
		remaining = rest[end+len("</tool_call>"):]
	}

	if len(calls) == 0 {
		for _, value := range extractJSONValues(trimmed) {
			calls = append(calls, parseToolCallsFromAny(value)...)
		}
	}

	after := strings.TrimSpace(remaining)
	if after != "" {
		textParts = append(textParts, after)
	}

	return strings.Join(textParts, "\n"), calls
}

func parseToolCallsFromAny(v any) []ParsedToolCall {
	out := make([]ParsedToolCall, 0, 2)
	if call := parseToolCallFromAny(v); call != nil {
		out = append(out, *call)
		return out
	}
	if obj, ok := v.(map[string]any); ok {
		if tcRaw, ok := obj["tool_calls"].([]any); ok {
			for _, item := range tcRaw {
				if call := parseToolCallFromAny(item); call != nil {
					out = append(out, *call)
				}
			}
		}
	}
	if arr, ok := v.([]any); ok {
		for _, item := range arr {
			if call := parseToolCallFromAny(item); call != nil {
				out = append(out, *call)
			}
		}
	}
	return out
}

func extractJSONValues(input string) []any {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	values := make([]any, 0, 2)

	// direct parse first
	var direct any
	if err := json.Unmarshal([]byte(trimmed), &direct); err == nil {
		return append(values, direct)
	}

	dec := json.NewDecoder(strings.NewReader(trimmed))
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			break
		}
		values = append(values, v)
	}

	// fallback: scan candidate starts and decode from slice
	runes := []rune(trimmed)
	for i, ch := range runes {
		if ch != '{' && ch != '[' {
			continue
		}
		slice := string(runes[i:])
		dec2 := json.NewDecoder(strings.NewReader(slice))
		var v any
		if err := dec2.Decode(&v); err == nil {
			values = append(values, v)
		}
	}
	return values
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
	for _, value := range extractJSONValues(raw) {
		if call := parseToolCallFromAny(value); call != nil {
			return call
		}
		if obj, ok := value.(map[string]any); ok {
			if call := parseToolCallMap(obj); call != nil {
				return call
			}
		}
	}
	return nil
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
	logger *slog.Logger
	memory *sqliteMemoryStore
}

// ToolHandler is the function signature for tool implementations.
type ToolHandler func(ctx context.Context, input string) (string, error)

// ToolSpec contains metadata+handler for prompt construction and execution.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  string
	Handler     ToolHandler
}

// NewToolRegistry creates a new tool registry with built-in tools.
func NewToolRegistry(cfg *config.Config, logger *slog.Logger) *ToolRegistry {
	reg := &ToolRegistry{
		tools:  make(map[string]ToolSpec),
		policy: NewSecurityPolicy(cfg),
		logger: logger.With("component", "memory"),
		memory: newSQLiteMemoryStore(),
	}
	if err := reg.memory.init(); err != nil {
		reg.logger.Error("memory sqlite init failed", "error", err)
	} else {
		reg.logger.Info("memory initialized", "backend", "sqlite", "db", reg.memory.dbPath, "auto_save", true)
	}

	// Register built-in tools.
	reg.Register("shell", "Execute terminal commands. Use for local checks/build/tests/diagnostics.", `{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer"}},"required":["command"]}`, reg.securedBashTool())
	reg.Register("bash", "Alias of shell. Execute terminal commands.", `{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer"}},"required":["command"]}`, reg.securedBashTool())
	reg.Register("memory_store", "Save to memory. Persist durable preferences/decisions/context.", `{"type":"object","properties":{"key":{"type":"string"},"content":{"type":"string"},"value":{"type":"string"}}}`, reg.memoryStoreTool())
	reg.Register("memory_recall", "Search memory and return matching entries.", `{"type":"object","properties":{"query":{"type":"string"},"key":{"type":"string"},"limit":{"type":"integer"}}}`, reg.memoryRecallTool())
	reg.Register("memory_forget", "Delete a memory entry by key.", `{"type":"object","properties":{"key":{"type":"string"}},"required":["key"]}`, reg.memoryForgetTool())

	return reg
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(name, description, parameters string, handler ToolHandler) {
	r.tools[name] = ToolSpec{
		Name:        name,
		Description: description,
		Parameters:  parameters,
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

// Has reports whether a tool is registered.
func (r *ToolRegistry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
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

func (r *ToolRegistry) memoryStoreTool() ToolHandler {
	return func(ctx context.Context, input string) (string, error) {
		_ = ctx
		var payload map[string]any
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return "", fmt.Errorf("invalid memory_store input: %w", err)
		}
		key := strings.TrimSpace(stringValue(payload["key"]))
		content := strings.TrimSpace(stringValue(payload["content"]))
		if key == "" {
			key = fmt.Sprintf("mem_%d", time.Now().UnixNano())
		}
		if content == "" {
			content = strings.TrimSpace(stringValue(payload["value"]))
		}
		if content == "" {
			return "", fmt.Errorf("content is required")
		}
		if err := r.memory.store(key, content); err != nil {
			r.logger.Error("memory store failed", "key", key, "error", err)
			return "", err
		}
		r.logger.Info("memory stored", "key", key, "content_len", len(content))
		return "Memory stored successfully", nil
	}
}

func (r *ToolRegistry) memoryRecallTool() ToolHandler {
	return func(ctx context.Context, input string) (string, error) {
		_ = ctx
		var payload map[string]any
		if strings.TrimSpace(input) != "" {
			_ = json.Unmarshal([]byte(input), &payload)
		}
		query := strings.ToLower(strings.TrimSpace(stringValue(payload["query"])))
		key := strings.TrimSpace(stringValue(payload["key"]))
		if query == "" && key != "" {
			query = strings.ToLower(key)
		}
		limit := intValue(payload["limit"], 5)
		if limit <= 0 {
			limit = 5
		}
		entries, err := r.memory.recall(query, key, limit)
		if err != nil {
			r.logger.Error("memory recall failed", "query", query, "key", key, "error", err)
			return "", err
		}
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, fmt.Sprintf("%s: %s", e.Key, e.Content))
		}
		if len(out) == 0 {
			r.logger.Info("memory recall empty", "query", query, "key", key)
			return "No memory found", nil
		}
		r.logger.Info("memory recalled", "query", query, "key", key, "count", len(out))
		return strings.Join(out, "\n"), nil
	}
}

func (r *ToolRegistry) memoryForgetTool() ToolHandler {
	return func(ctx context.Context, input string) (string, error) {
		_ = ctx
		var payload map[string]any
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return "", fmt.Errorf("invalid memory_forget input: %w", err)
		}
		key := strings.TrimSpace(stringValue(payload["key"]))
		if key == "" {
			return "", fmt.Errorf("key is required")
		}
		if err := r.memory.forget(key); err != nil {
			r.logger.Error("memory forget failed", "key", key, "error", err)
			return "", err
		}
		r.logger.Info("memory forgotten", "key", key)
		return "Memory entry removed", nil
	}
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func intValue(v any, fallback int) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return i
		}
	}
	return fallback
}

func isNonRetryableProviderError(err error) bool {
	var apiErr *providers.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden, http.StatusNotFound, http.StatusBadRequest:
			return true
		}
	}
	return false
}

func formatProviderError(provider string, err error) string {
	var apiErr *providers.APIError
	if errors.As(err, &apiErr) {
		statusText := http.StatusText(apiErr.StatusCode)
		if statusText == "" {
			statusText = "Unknown Status"
		}
		return fmt.Sprintf("%s API error (%d %s): %s", providerDisplayName(provider), apiErr.StatusCode, statusText, strings.TrimSpace(apiErr.Body))
	}
	return err.Error()
}

func providerDisplayName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openrouter":
		return "OpenRouter"
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	default:
		if provider == "" {
			return "Provider"
		}
		return strings.ToUpper(provider[:1]) + provider[1:]
	}
}
