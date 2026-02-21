// Package agent implements the AI agent runtime.
// The agent runs Pi agent sessions, manages model API calls,
// handles tool execution, and manages skills.
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/highclaw/highclaw/internal/agent/providers"
	"github.com/highclaw/highclaw/internal/agent/tools"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/skills"
)

const maxToolIterations = 10

const (
	maxHistoryMessages            = 50
	compactionKeepRecent          = 20
	compactionMaxSourceChars      = 12000
	compactionMaxSummaryChars     = 2000
	compactionSummarySystemPrompt = "You are a conversation compaction engine. Summarize older chat history into concise context for future turns. Preserve: user preferences, commitments, decisions, unresolved tasks, key facts. Omit: filler, repeated chit-chat, verbose tool logs. Output plain text bullet points only."
)

func autosaveMemoryKey(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func conversationMemoryKey(channel, sender, id string) string {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = "unknown"
	}
	sender = strings.TrimSpace(sender)
	if sender == "" {
		sender = "user"
	}
	id = strings.TrimSpace(id)
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%s_%s", channel, sender, id)
}

func buildMemoryContext(reg *ToolRegistry, userMsg, sessionKey string) string {
	_ = sessionKey
	if reg == nil || reg.memory == nil {
		return ""
	}
	query := strings.TrimSpace(userMsg)
	if query == "" {
		return ""
	}
	entries, err := reg.memory.recall(query, "", "", 5)
	if err != nil || len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[Memory context]\n")
	for _, e := range entries {
		if strings.TrimSpace(e.Content) == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s\n", e.Key, e.Content)
	}
	b.WriteString("\n")
	return b.String()
}

func truncateWithEllipsis(input string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= maxChars {
		return input
	}
	if maxChars <= 1 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-1]) + "…"
}

func trimHistory(history []ChatMessage) []ChatMessage {
	if len(history) == 0 {
		return history
	}
	hasSystem := strings.EqualFold(strings.TrimSpace(history[0].Role), "system")
	start := 0
	if hasSystem {
		start = 1
	}
	nonSystem := len(history) - start
	if nonSystem <= maxHistoryMessages {
		return history
	}
	toRemove := nonSystem - maxHistoryMessages
	trimmed := make([]ChatMessage, 0, len(history)-toRemove)
	trimmed = append(trimmed, history[:start]...)
	trimmed = append(trimmed, history[start+toRemove:]...)
	return trimmed
}

func buildCompactionTranscript(messages []ChatMessage) string {
	var b strings.Builder
	for _, msg := range messages {
		role := strings.ToUpper(strings.TrimSpace(msg.Role))
		content := strings.TrimSpace(msg.Content)
		if role == "" || content == "" {
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", role, content)
	}
	return truncateWithEllipsis(b.String(), compactionMaxSourceChars)
}

func autoCompactHistory(
	ctx context.Context,
	history []ChatMessage,
	mgr *ModelManager,
	provider, model string,
) []ChatMessage {
	if len(history) == 0 {
		return history
	}
	hasSystem := strings.EqualFold(strings.TrimSpace(history[0].Role), "system")
	start := 0
	if hasSystem {
		start = 1
	}
	nonSystem := len(history) - start
	if nonSystem <= maxHistoryMessages {
		return history
	}
	keepRecent := compactionKeepRecent
	if keepRecent > nonSystem {
		keepRecent = nonSystem
	}
	compactCount := nonSystem - keepRecent
	if compactCount <= 0 {
		return history
	}
	compactStart := start
	compactEnd := start + compactCount
	toCompact := history[compactStart:compactEnd]
	recent := history[compactEnd:]
	transcript := buildCompactionTranscript(toCompact)
	if strings.TrimSpace(transcript) == "" {
		return history
	}
	summaryReq := &ChatRequest{
		SystemPrompt: compactionSummarySystemPrompt,
		Messages: []ChatMessage{{
			Role:    "user",
			Content: "Summarize the following conversation history for context preservation. Keep it short (max 12 bullet points).\n\n" + transcript,
		}},
		Provider:    strings.TrimSpace(provider),
		Model:       strings.TrimSpace(model),
		Temperature: 0.2,
	}
	resp, err := mgr.Chat(ctx, summaryReq)
	summary := transcript
	if err == nil && strings.TrimSpace(resp.Content) != "" {
		summary = strings.TrimSpace(resp.Content)
	}
	summary = truncateWithEllipsis(summary, compactionMaxSummaryChars)
	compactedNonSystem := []ChatMessage{{
		Role:    "assistant",
		Content: "[Compaction summary]\n" + summary,
	}}
	compactedNonSystem = append(compactedNonSystem, recent...)
	if hasSystem {
		return append([]ChatMessage{history[0]}, compactedNonSystem...)
	}
	return compactedNonSystem
}

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
	Sender       string
	MessageID    string
	Message      string
	History      []ChatMessage
	Images       [][]byte
	AgentID      string
	SystemPrompt string
	Provider     string
	Model        string
	Temperature  float64
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
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = "cli"
	}
	sender := strings.TrimSpace(req.Sender)
	if sender == "" {
		sender = "user"
	}
	userMessage := strings.TrimSpace(req.Message)
	if userMessage != "" && r.cfg.Memory.AutoSave && r.tools != nil && r.tools.memory != nil {
		meta := memoryMeta{
			SessionKey: strings.TrimSpace(req.SessionKey),
			Channel:    channel,
			Sender:     sender,
			MessageID:  strings.TrimSpace(req.MessageID),
		}
		if strings.TrimSpace(req.MessageID) != "" {
			_ = r.tools.memory.store(
				conversationMemoryKey(channel, sender, req.MessageID),
				userMessage,
				"conversation",
				meta,
			)
		}
		_ = r.tools.memory.store(autosaveMemoryKey("user_msg"), userMessage, "conversation", meta)
	}
	if ctxText := buildMemoryContext(r.tools, userMessage, req.SessionKey); ctxText != "" {
		req.Message = ctxText + req.Message
	}

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
	// 如果调用者已经传入了完整历史（包含最新消息），直接使用
	// 否则追加新消息
	if len(history) == 0 {
		history = append(history, ChatMessage{Role: "user", Content: req.Message})
	}
	// 注意：不再覆盖历史中的最后一条用户消息
	// commands.go 已经把新消息追加到 history 中了
	var totalUsage TokenUsage
	history = autoCompactHistory(ctx, history, r.models, strings.TrimSpace(req.Provider), strings.TrimSpace(req.Model))
	history = trimHistory(history)

	for i := 0; i < maxToolIterations; i++ {
		modelStart := time.Now()
		modelResp, err := r.models.Chat(ctx, &ChatRequest{
			SystemPrompt: systemPrompt,
			Messages:     history,
			Provider:     strings.TrimSpace(req.Provider),
			Model:        strings.TrimSpace(req.Model),
			MaxTokens:    0,
			Temperature:  req.Temperature,
		})
		if err != nil {
			return nil, err
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
			history = append(history, ChatMessage{Role: "assistant", Content: modelResp.Content})
			if strings.TrimSpace(reply) != "" && r.cfg.Memory.AutoSave && r.tools != nil && r.tools.memory != nil {
				_ = r.tools.memory.store(
					autosaveMemoryKey("assistant_resp"),
					truncateWithEllipsis(reply, 100),
					"daily",
					memoryMeta{
						SessionKey: strings.TrimSpace(req.SessionKey),
						Channel:    channel,
						Sender:     "assistant",
					},
				)
			}
			return &RunResult{
				Reply:      reply,
				TokensUsed: totalUsage,
			}, nil
		}

		// Match ZeroClaw interactive behavior: print text produced alongside tool calls.
		if strings.TrimSpace(text) != "" {
			fmt.Print(text)
			_ = os.Stdout.Sync()
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
			fmt.Fprintf(&toolResults, "<tool_result name=\"%s\">\n%s\n</tool_result>\n", call.Name, output)
		}

		history = append(history, ChatMessage{Role: "assistant", Content: modelResp.Content})
		history = append(history, ChatMessage{
			Role:    "user",
			Content: "[Tool results]\n" + toolResults.String(),
		})
	}

	return nil, fmt.Errorf("Agent exceeded maximum tool iterations (%d)", maxToolIterations)
}

// buildSystemPrompt constructs the system prompt from config, skills, and context.
func (r *Runner) buildSystemPrompt(req *RunRequest) string {
	if req.SystemPrompt != "" {
		return req.SystemPrompt
	}

	var b strings.Builder
	b.WriteString("You are HighClaw, a personal AI assistant.\n\n")
	b.WriteString("## Tools\n\n")
	b.WriteString("You have access to the following tools:\n\n")
	for _, spec := range r.tools.Specs() {
		fmt.Fprintf(&b, "- **%s**: %s\n", spec.Name, spec.Description)
	}
	b.WriteString("\n")
	b.WriteString("## Tool Use Protocol\n\n")
	b.WriteString("To use a tool, wrap a JSON object in <invoke> tags:\n\n")
	b.WriteString("```\n<invoke>\n{\"name\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}\n</invoke>\n```\n\n")
	b.WriteString("You may use multiple tool calls in a single response. ")
	b.WriteString("After tool execution, results appear in <tool_result> tags. ")
	b.WriteString("Continue reasoning with the results until you can give a final answer.\n\n")
	b.WriteString("### Available Tools\n\n")
	for _, spec := range r.tools.Specs() {
		fmt.Fprintf(&b, "**%s**: %s\nParameters: `%s`\n\n", spec.Name, spec.Description, spec.Parameters)
	}

	b.WriteString("## Safety\n\n")
	b.WriteString("- Do not exfiltrate private data.\n")
	b.WriteString("- Do not run destructive commands without asking.\n")
	b.WriteString("- Do not bypass oversight or approval mechanisms.\n")
	b.WriteString("- Prefer `trash` over `rm` (recoverable beats gone forever).\n")
	b.WriteString("- When in doubt, ask before acting externally.\n\n")

	workspace := strings.TrimSpace(r.cfg.Agent.Workspace)
	if workspace == "" {
		workspace = filepath.Join(config.ConfigDir(), "workspace")
	}

	fmt.Fprintf(&b, "## Workspace\n\nWorking directory: `%s`\n\n", workspace)
	b.WriteString("## Project Context\n\n")
	for _, name := range []string{
		"IDENTITY.md", "AGENTS.md", "HEARTBEAT.md", "SOUL.md",
		"USER.md", "TOOLS.md", "BOOTSTRAP.md", "MEMORY.md",
	} {
		path := filepath.Join(workspace, name)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", name, strings.TrimSpace(string(content)))
	}

	// 加载并注入 user-defined skills
	skillMgr := skills.NewManager(workspace)
	allSkills := skillMgr.LoadAll()
	if len(allSkills) > 0 {
		b.WriteString(skills.ToSystemPrompt(allSkills))
	}

	now := time.Now()
	fmt.Fprintf(&b, "## Current Date & Time\n\nTimezone: %s\n\n", now.Format("MST"))

	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown"
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(r.cfg.Agent.Model)
	}
	fmt.Fprintf(&b, "## Runtime\n\nHost: %s | OS: %s | Model: %s\n", host, runtime.GOOS, modelName)
	return b.String()
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
	Provider      string
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
	if req.Temperature == 0 {
		// Align with ZeroClaw default_temperature.
		req.Temperature = 0.7
	}

	// Resolve hint routes (hint:xxx) before primary provider selection.
	routeProvider, routeModel, routed := m.resolveHintRoute(model)
	effectiveModel := model
	if routed {
		effectiveModel = routeModel
	}

	// Determine provider/model with ZeroClaw-like priority:
	// hint route provider > provider override > configured primary > model prefix.
	provider := m.resolvePrimaryProvider(req.Provider, effectiveModel)
	if routeProvider != "" {
		provider = routeProvider
	}
	modelName := normalizeModelForProvider(effectiveModel, provider)

	candidates := m.providerCandidates(provider)
	maxAttempts := int(m.cfg.Reliability.ProviderRetries) + 1
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	baseBackoff := time.Duration(m.cfg.Reliability.ProviderBackoffMs) * time.Millisecond
	if baseBackoff <= 0 {
		baseBackoff = 500 * time.Millisecond
	}
	attemptErrors := make([]string, 0, len(candidates)*maxAttempts)

	for _, candidate := range candidates {
		p, err := m.factory.Create(candidate, m.cfg)
		if err != nil {
			msg := normalizeProviderCreateError(candidate, err)
			for i := 1; i <= maxAttempts; i++ {
				attemptErrors = append(attemptErrors, fmt.Sprintf(
					"%s attempt %d/%d: %s",
					candidate, i, maxAttempts, msg,
				))
			}
			continue
		}

		for i := 1; i <= maxAttempts; i++ {
			m.logger.Debug("calling model", "provider", candidate, "model", modelName)
			resp, err := p.Chat(ctx, req, modelName)
			if err == nil {
				if i > 1 {
					m.logger.Info("Provider recovered after retries", "provider", candidate, "attempt", i-1)
				}
				return resp, nil
			}
			attemptErrors = append(attemptErrors, fmt.Sprintf(
				"%s attempt %d/%d: %s",
				candidate, i, maxAttempts, formatProviderError(candidate, err),
			))
			if isNonRetryableProviderError(err) {
				m.logger.Warn("Non-retryable error, switching provider", "provider", candidate)
				break
			}

			if i < maxAttempts {
				m.logger.Warn("Provider call failed, retrying", "provider", candidate, "attempt", i, "max_retries", maxAttempts-1)
				backoff := baseBackoff * time.Duration(1<<(i-1))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
			}
		}
		// Match ZeroClaw reliable-provider logs: emit this after each provider cycle.
		m.logger.Warn("Switching to fallback provider", "provider", candidate)
	}
	return nil, fmt.Errorf("All providers failed. Attempts:\n%s", strings.Join(attemptErrors, "\n"))
}

func normalizeProviderCreateError(provider string, err error) string {
	msg := strings.TrimSpace(err.Error())
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "api key not configured") {
		upper := strings.ToUpper(strings.TrimSpace(provider))
		if upper == "" {
			upper = "PROVIDER"
		}
		return fmt.Sprintf("%s API key not set. Run `highclaw onboard` or set the appropriate env var.", upper)
	}
	return msg
}

func (m *ModelManager) providerCandidates(primary string) []string {
	primary = normalizeProviderName(primary)
	if primary == "" {
		primary = "openrouter"
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	add := func(name string, validate bool) {
		name = normalizeProviderName(name)
		if name == "" {
			return
		}
		if validate && !m.factory.Has(name) {
			m.logger.Warn("Ignoring invalid fallback provider", "fallback_provider", name)
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	// Always keep primary candidate; validation happens at create-call stage.
	add(primary, false)
	for _, name := range m.cfg.Reliability.FallbackProviders {
		add(name, true)
	}
	if len(out) == 0 {
		out = append(out, "openrouter")
	}
	return out
}

func (m *ModelManager) resolvePrimaryProvider(override, model string) string {
	if p := strings.ToLower(strings.TrimSpace(override)); p != "" {
		return p
	}
	// Explicit provider/model syntax should always take precedence.
	// This avoids misrouting (e.g., glm/glm-5 sent to openrouter) when key resolution
	// happens via env/route-specific config later.
	if prefix, _, ok := splitModelPrefix(model); ok {
		return prefix
	}
	// ZeroClaw commonly defaults to openrouter when configured.
	if m.hasProviderConfigured("openrouter") {
		return "openrouter"
	}
	// Fallback to first configured provider deterministically.
	keys := make([]string, 0, len(m.cfg.Agent.Providers))
	for k, pcfg := range m.cfg.Agent.Providers {
		if strings.TrimSpace(pcfg.APIKey) != "" {
			keys = append(keys, strings.ToLower(strings.TrimSpace(k)))
		}
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return keys[0]
	}
	return "openrouter"
}

func (m *ModelManager) resolveHintRoute(model string) (provider, resolvedModel string, ok bool) {
	hint, hasHint := strings.CutPrefix(strings.TrimSpace(model), "hint:")
	if !hasHint {
		return "", "", false
	}
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return "", "", false
	}
	for _, route := range m.cfg.ModelRoutes {
		if strings.TrimSpace(route.Hint) != hint {
			continue
		}
		p := strings.ToLower(strings.TrimSpace(route.Provider))
		rm := strings.TrimSpace(route.Model)
		if p == "" || rm == "" {
			continue
		}
		return p, rm, true
	}
	m.logger.Warn("Unknown route hint, falling back to default provider", "hint", hint)
	return "", "", false
}

func normalizeModelForProvider(model, provider string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return model
	}
	prefix, rest, ok := splitModelPrefix(model)
	if !ok {
		return model
	}
	// Keep full "vendor/model" for gateway-style providers (notably openrouter).
	if provider == "openrouter" {
		// Backward compatibility: onboard may persist "openrouter/<vendor>/<model>".
		// OpenRouter expects "<vendor>/<model>", so strip only a leading openrouter prefix.
		if prefix == "openrouter" {
			return rest
		}
		return model
	}
	// If provider matches prefix, pass the raw model id.
	if provider == prefix {
		return rest
	}
	// Cross-provider explicit model path should be preserved.
	return model
}

func splitModelPrefix(model string) (prefix, rest string, ok bool) {
	model = strings.TrimSpace(model)
	lm := strings.ToLower(model)
	if strings.HasPrefix(lm, "custom:http://") || strings.HasPrefix(lm, "custom:https://") ||
		strings.HasPrefix(lm, "anthropic-custom:http://") || strings.HasPrefix(lm, "anthropic-custom:https://") {
		// custom provider identifiers contain URL slashes, so split at the last slash.
		// Format: custom:https://host/path/<model-id>
		idx := strings.LastIndex(model, "/")
		if idx <= 0 || idx >= len(model)-1 {
			return "", "", false
		}
		return strings.TrimSpace(model[:idx]), strings.TrimSpace(model[idx+1:]), true
	}
	for i, ch := range model {
		if ch == '/' {
			return strings.ToLower(strings.TrimSpace(model[:i])), strings.TrimSpace(model[i+1:]), true
		}
	}
	return "", "", false
}

func (m *ModelManager) hasProviderConfigured(provider string) bool {
	_, ok := resolveProviderConfig(m.cfg, provider)
	return ok
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
		pcfg, ok := resolveProviderConfig(cfg, "anthropic")
		if !ok {
			return nil, fmt.Errorf("anthropic API key not configured")
		}
		return &anthropicProvider{client: providers.NewAnthropicClient(pcfg.APIKey)}, nil
	})
	f.Register("openai", func(cfg *config.Config) (Provider, error) {
		pcfg, ok := resolveProviderConfig(cfg, "openai")
		if !ok {
			return nil, fmt.Errorf("openai API key not configured")
		}
		return &openAIProvider{client: providers.NewOpenAIClientWithBaseURL(pcfg.APIKey, pcfg.BaseURL)}, nil
	})
	f.Register("openrouter", func(cfg *config.Config) (Provider, error) {
		pcfg, ok := resolveProviderConfig(cfg, "openrouter")
		if !ok {
			return nil, fmt.Errorf("openrouter API key not configured")
		}
		baseURL := strings.TrimSpace(pcfg.BaseURL)
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		return &openAIProvider{client: providers.NewOpenAIClientWithBaseURLAndHeaders(
			pcfg.APIKey,
			baseURL,
			map[string]string{
				"HTTP-Referer": "https://github.com/highclaw/highclaw",
				"X-Title":      "HighClaw",
			},
		)}, nil
	})
	registerOpenAICompatProviders(f,
		"venice", "deepseek", "mistral", "xai", "grok", "perplexity", "groq",
		"fireworks", "fireworks-ai", "together", "together-ai", "cohere",
		"moonshot", "kimi", "glm", "zhipu", "zai", "z.ai", "minimax",
		"qianfan", "baidu", "vercel", "vercel-ai", "cloudflare", "cloudflare-ai",
		"opencode", "opencode-zen", "synthetic", "gemini", "google", "google-gemini",
		"bedrock", "aws-bedrock",
	)
	return f
}

func registerOpenAICompatProviders(f *ProviderFactory, names ...string) {
	for _, name := range names {
		providerName := name
		f.Register(providerName, func(cfg *config.Config) (Provider, error) {
			pcfg, ok := resolveProviderConfig(cfg, providerName)
			if !ok {
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
		return "https://api.deepseek.com"
	case "groq":
		return "https://api.groq.com/openai"
	case "mistral":
		return "https://api.mistral.ai"
	case "together":
		return "https://api.together.xyz"
	case "together-ai":
		return "https://api.together.xyz"
	case "fireworks":
		return "https://api.fireworks.ai/inference"
	case "fireworks-ai":
		return "https://api.fireworks.ai/inference"
	case "cohere":
		return "https://api.cohere.com/compatibility"
	case "moonshot", "kimi":
		return "https://api.moonshot.cn"
	case "glm", "zhipu", "zai", "z.ai":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "xai":
		return "https://api.x.ai"
	case "perplexity":
		return "https://api.perplexity.ai"
	case "vercel", "vercel-ai":
		return "https://api.vercel.ai"
	case "cloudflare", "cloudflare-ai":
		return "https://gateway.ai.cloudflare.com/v1"
	case "bedrock", "aws-bedrock":
		return "https://bedrock-runtime.us-east-1.amazonaws.com"
	case "qianfan", "baidu":
		return "https://aip.baidubce.com"
	case "opencode", "opencode-zen":
		return "https://api.opencode.ai"
	case "minimax":
		return "https://api.minimax.chat/v1"
	case "grok":
		return "https://api.x.ai"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	case "google", "google-gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	return ""
}

func resolveProviderConfig(cfg *config.Config, provider string) (config.ProviderConfig, bool) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	aliases := providerAliases(provider)
	for _, key := range aliases {
		if pcfg, ok := cfg.Agent.Providers[key]; ok {
			pcfg.APIKey = strings.TrimSpace(pcfg.APIKey)
			if pcfg.APIKey != "" {
				return pcfg, true
			}
		}
	}
	// Route-scoped API key support (ZeroClaw parity): allow model route entries
	// to carry provider-specific keys even when Agent.Providers is not populated.
	for _, route := range cfg.ModelRoutes {
		rp := strings.ToLower(strings.TrimSpace(route.Provider))
		if rp == "" {
			continue
		}
		for _, alias := range aliases {
			if rp != alias {
				continue
			}
			if k := strings.TrimSpace(route.APIKey); k != "" {
				return config.ProviderConfig{
					APIKey:  k,
					BaseURL: defaultBaseURLForProvider(provider),
				}, true
			}
		}
	}

	// Environment override (matches ZeroClaw-style provider env precedence).
	for _, envKey := range providerEnvCandidates(provider) {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return config.ProviderConfig{
				APIKey:  v,
				BaseURL: defaultBaseURLForProvider(provider),
			}, true
		}
	}
	for _, envKey := range []string{"HIGHCLAW_API_KEY", "OPENCLAW_API_KEY", "ZEROCLAW_API_KEY", "API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return config.ProviderConfig{
				APIKey:  v,
				BaseURL: defaultBaseURLForProvider(provider),
			}, true
		}
	}
	return config.ProviderConfig{}, false
}

func providerAliases(provider string) []string {
	switch provider {
	case "z.ai":
		return []string{"z.ai", "zai"}
	case "zai":
		return []string{"zai", "z.ai"}
	case "zhipu":
		return []string{"zhipu", "glm"}
	case "glm":
		return []string{"glm", "zhipu"}
	case "google", "google-gemini":
		return []string{"google", "google-gemini", "gemini"}
	case "gemini":
		return []string{"gemini", "google", "google-gemini"}
	}
	return []string{provider}
}

func providerEnvCandidates(provider string) []string {
	switch provider {
	case "anthropic":
		return []string{"ANTHROPIC_OAUTH_TOKEN", "ANTHROPIC_API_KEY"}
	case "openrouter":
		return []string{"OPENROUTER_API_KEY"}
	case "openai":
		return []string{"OPENAI_API_KEY"}
	case "venice":
		return []string{"VENICE_API_KEY"}
	case "groq":
		return []string{"GROQ_API_KEY"}
	case "mistral":
		return []string{"MISTRAL_API_KEY"}
	case "deepseek":
		return []string{"DEEPSEEK_API_KEY"}
	case "xai":
		return []string{"XAI_API_KEY"}
	case "together", "together-ai":
		return []string{"TOGETHER_API_KEY"}
	case "fireworks", "fireworks-ai":
		return []string{"FIREWORKS_API_KEY"}
	case "perplexity":
		return []string{"PERPLEXITY_API_KEY"}
	case "cohere":
		return []string{"COHERE_API_KEY"}
	case "moonshot", "kimi":
		return []string{"MOONSHOT_API_KEY"}
	case "glm", "zhipu":
		return []string{"GLM_API_KEY"}
	case "minimax":
		return []string{"MINIMAX_API_KEY"}
	case "qianfan", "baidu":
		return []string{"QIANFAN_API_KEY"}
	case "zai", "z.ai":
		return []string{"ZAI_API_KEY"}
	case "synthetic":
		return []string{"SYNTHETIC_API_KEY"}
	case "opencode", "opencode-zen":
		return []string{"OPENCODE_API_KEY"}
	case "vercel", "vercel-ai":
		return []string{"VERCEL_API_KEY"}
	case "cloudflare", "cloudflare-ai":
		return []string{"CLOUDFLARE_API_KEY"}
	case "gemini", "google", "google-gemini":
		return []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}
	case "bedrock", "aws-bedrock":
		return []string{"AWS_BEDROCK_API_KEY", "BEDROCK_API_KEY"}
	}
	return nil
}

// Register registers a provider builder.
func (f *ProviderFactory) Register(name string, builder ProviderBuilder) {
	f.builders[name] = builder
}

// Has reports whether provider name can be resolved by this factory.
func (f *ProviderFactory) Has(name string) bool {
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "custom:") || strings.HasPrefix(lower, "anthropic-custom:") {
		return true
	}
	_, ok := f.builders[lower]
	return ok
}

func normalizeProviderName(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return ""
	}
	lower := strings.ToLower(n)
	if strings.HasPrefix(lower, "custom:") || strings.HasPrefix(lower, "anthropic-custom:") {
		return n
	}
	return lower
}

// Create instantiates a provider by name.
func (f *ProviderFactory) Create(name string, cfg *config.Config) (Provider, error) {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "custom:") {
		baseURL, err := parseCustomProviderURL(strings.TrimPrefix(name, "custom:"), "Custom provider", "custom:https://your-api.com")
		if err != nil {
			return nil, err
		}
		pcfg := config.ProviderConfig{}
		if byExact, ok := cfg.Agent.Providers[name]; ok {
			pcfg = byExact
		} else if byGeneric, ok := cfg.Agent.Providers["custom"]; ok {
			pcfg = byGeneric
		}
		if strings.TrimSpace(pcfg.APIKey) == "" {
			for _, k := range []string{"OPENAI_API_KEY", "HIGHCLAW_API_KEY", "OPENCLAW_API_KEY", "ZEROCLAW_API_KEY", "API_KEY"} {
				if v := strings.TrimSpace(os.Getenv(k)); v != "" {
					pcfg.APIKey = v
					break
				}
			}
		}
		if strings.TrimSpace(pcfg.APIKey) == "" {
			return nil, fmt.Errorf("custom API key not configured")
		}
		return &openAIProvider{client: providers.NewOpenAIClientWithBaseURL(pcfg.APIKey, baseURL)}, nil
	}
	if strings.HasPrefix(name, "anthropic-custom:") {
		baseURL, err := parseCustomProviderURL(strings.TrimPrefix(name, "anthropic-custom:"), "Anthropic-custom provider", "anthropic-custom:https://your-api.com")
		if err != nil {
			return nil, err
		}
		pcfg := config.ProviderConfig{}
		if byExact, ok := cfg.Agent.Providers[name]; ok {
			pcfg = byExact
		} else if byGeneric, ok := cfg.Agent.Providers["anthropic-custom"]; ok {
			pcfg = byGeneric
		}
		if strings.TrimSpace(pcfg.APIKey) == "" {
			pcfg, _ = resolveProviderConfig(cfg, "anthropic")
		}
		if strings.TrimSpace(pcfg.APIKey) == "" {
			for _, k := range []string{"ANTHROPIC_OAUTH_TOKEN", "ANTHROPIC_API_KEY", "HIGHCLAW_API_KEY", "OPENCLAW_API_KEY", "ZEROCLAW_API_KEY", "API_KEY"} {
				if v := strings.TrimSpace(os.Getenv(k)); v != "" {
					pcfg.APIKey = v
					break
				}
			}
		}
		if strings.TrimSpace(pcfg.APIKey) == "" {
			return nil, fmt.Errorf("anthropic-custom API key not configured")
		}
		return &anthropicProvider{client: providers.NewAnthropicClientWithBaseURL(pcfg.APIKey, baseURL)}, nil
	}
	builder, ok := f.builders[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s. check README for supported providers or run `highclaw onboard --interactive` to reconfigure.\nTip: use \"custom:https://your-api.com\" for OpenAI-compatible endpoints.\nTip: use \"anthropic-custom:https://your-api.com\" for Anthropic-compatible endpoints.", name)
	}
	return builder(cfg)
}

func parseCustomProviderURL(raw, label, example string) (string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", fmt.Errorf("%s URL is empty (example: %s)", label, example)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("%s URL is invalid: %w", label, err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("%s URL must use http or https", label)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("%s URL must include a host", label)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
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
		return nil, err
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
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096 // 默认 4096，与 Anthropic 保持一致
	}
	openAIReq := &providers.OpenAIChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}
	resp, err := p.client.Chat(ctx, openAIReq)
	if err != nil {
		return nil, err
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
// 1) XML-style: <tool_call>{"name":"bash","arguments":{...}}</tool_call>
// 2) OpenAI-style JSON with tool_calls array.
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
	// 解析 <tool_call> 和 <invoke> 标签（兼容 ZeroClaw 格式）
	tagPairs := []struct {
		open  string
		close string
	}{
		{"<tool_call>", "</tool_call>"},
		{"<invoke>", "</invoke>"},
	}
	for _, tag := range tagPairs {
		for {
			start := strings.Index(remaining, tag.open)
			if start == -1 {
				break
			}
			before := strings.TrimSpace(remaining[:start])
			if before != "" {
				textParts = append(textParts, before)
			}
			rest := remaining[start+len(tag.open):]
			end := strings.Index(rest, tag.close)
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
			remaining = rest[end+len(tag.close):]
		}
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

	// ZeroClaw parity: scan candidate JSON starts and decode non-overlapping values.
	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]
		if ch != '{' && ch != '[' {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(trimmed[i:]))
		var v any
		if err := dec.Decode(&v); err != nil {
			continue
		}
		consumed := int(dec.InputOffset())
		if consumed <= 0 {
			continue
		}
		values = append(values, v)
		// Move cursor to the end of this decoded JSON value.
		i += consumed - 1
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
	memory memoryStore
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
	runMemoryHygieneIfDue(cfg, logger.With("component", "memory"))

	backend := strings.ToLower(strings.TrimSpace(cfg.Memory.Backend))
	if backend == "" {
		backend = "sqlite"
	}
	var store memoryStore
	switch backend {
	case "none":
		backend = "markdown"
		store = newMarkdownMemoryStore(cfg.Agent.Workspace)
	case "markdown":
		store = newMarkdownMemoryStore(cfg.Agent.Workspace)
	case "sqlite":
		backend = "sqlite"
		store = newSQLiteMemoryStore(cfg)
	default:
		runtimeBackend := backend
		backend = "markdown"
		logger.Warn("unknown memory backend, falling back to markdown", "backend", runtimeBackend)
		store = newMarkdownMemoryStore(cfg.Agent.Workspace)
	}

	reg := &ToolRegistry{
		tools:  make(map[string]ToolSpec),
		policy: NewSecurityPolicy(cfg),
		logger: logger.With("component", "memory"),
		memory: store,
	}
	if err := store.init(); err != nil {
		reg.logger.Error("memory init failed", "backend", backend, "error", err)
	} else {
		reg.logger.Info("memory initialized", "backend", backend, "db", store.location(), "auto_save", cfg.Memory.AutoSave)
	}

	// Register built-in tools.
	reg.Register("shell", "Execute terminal commands. Use for local checks/build/tests/diagnostics.", `{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer"}},"required":["command"]}`, reg.securedBashTool())
	reg.Register("bash", "Alias of shell. Execute terminal commands.", `{"type":"object","properties":{"command":{"type":"string"},"timeout":{"type":"integer"}},"required":["command"]}`, reg.securedBashTool())
	reg.Register("memory_store", "Save to memory. Persist durable preferences/decisions/context.", `{"type":"object","properties":{"key":{"type":"string"},"content":{"type":"string"},"category":{"type":"string","enum":["core","daily","conversation"]}},"required":["key","content"]}`, reg.memoryStoreTool())
	reg.Register("memory_recall", "Search memory and return matching entries.", `{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`, reg.memoryRecallTool())
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
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
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
			return "", fmt.Errorf("Missing 'key' parameter")
		}
		if content == "" {
			return "", fmt.Errorf("Missing 'content' parameter")
		}
		category := strings.ToLower(strings.TrimSpace(stringValue(payload["category"])))
		if category == "" {
			category = "core"
		}
		switch category {
		case "core", "daily", "conversation":
		default:
			category = "core"
		}
		if err := r.memory.store(key, content, category, memoryMeta{}); err != nil {
			r.logger.Error("memory store failed", "key", key, "error", err)
			return "", err
		}
		r.logger.Info("memory stored", "key", key, "category", category, "content_len", len(content))
		return fmt.Sprintf("Stored memory: %s", key), nil
	}
}

func (r *ToolRegistry) memoryRecallTool() ToolHandler {
	return func(ctx context.Context, input string) (string, error) {
		_ = ctx
		var payload map[string]any
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return "", fmt.Errorf("invalid memory_recall input: %w", err)
		}
		query := strings.TrimSpace(stringValue(payload["query"]))
		if query == "" {
			return "", fmt.Errorf("Missing 'query' parameter")
		}
		limit := 5
		if rawLimit, ok := payload["limit"]; ok {
			limit = intValue(rawLimit, 0)
		}
		entries, err := r.memory.recall(query, "", "", limit)
		if err != nil {
			r.logger.Error("memory recall failed", "query", query, "error", err)
			return "", err
		}
		if len(entries) == 0 {
			r.logger.Info("memory recall empty", "query", query)
			return "No memories found matching that query.", nil
		}
		out := make([]string, 0, len(entries)+1)
		out = append(out, fmt.Sprintf("Found %d memories:", len(entries)))
		for _, e := range entries {
			scoreText := ""
			if e.Score > 0 {
				scoreText = fmt.Sprintf(" [%.0f%%]", e.Score*100)
			}
			out = append(out, fmt.Sprintf("- [%s] %s: %s%s", e.Category, e.Key, e.Content, scoreText))
		}
		r.logger.Info("memory recalled", "query", query, "count", len(entries))
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
			return "", fmt.Errorf("Missing 'key' parameter")
		}
		removed, err := r.memory.forget(key)
		if err != nil {
			r.logger.Error("memory forget failed", "key", key, "error", err)
			return "", err
		}
		if removed {
			r.logger.Info("memory forgotten", "key", key)
			return fmt.Sprintf("Forgot memory: %s", key), nil
		}
		r.logger.Info("memory forget no-op", "key", key)
		return fmt.Sprintf("No memory found with key: %s", key), nil
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
		code := apiErr.StatusCode
		if code >= 400 && code < 500 {
			// 429 / 408 are transient and should remain retryable.
			return code != http.StatusTooManyRequests && code != http.StatusRequestTimeout
		}
		return false
	}
	// ZeroClaw parity: fallback string scan for 4xx codes when error wrappers hide typed status.
	msg := err.Error()
	current := ""
	for _, r := range msg {
		if r >= '0' && r <= '9' {
			current += string(r)
			continue
		}
		if current != "" {
			if code, convErr := strconv.Atoi(current); convErr == nil && code >= 400 && code < 500 {
				return code != http.StatusTooManyRequests && code != http.StatusRequestTimeout
			}
			current = ""
		}
	}
	if current != "" {
		if code, convErr := strconv.Atoi(current); convErr == nil && code >= 400 && code < 500 {
			return code != http.StatusTooManyRequests && code != http.StatusRequestTimeout
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
	case "glm", "zhipu":
		return "GLM"
	case "zai", "z.ai":
		return "Z.AI"
	case "xai", "grok":
		return "xAI"
	case "minimax":
		return "MiniMax"
	case "qianfan", "baidu":
		return "Qianfan"
	case "opencode", "opencode-zen":
		return "OpenCode Zen"
	case "together", "together-ai":
		return "Together AI"
	case "fireworks", "fireworks-ai":
		return "Fireworks AI"
	case "cloudflare", "cloudflare-ai":
		return "Cloudflare AI"
	case "vercel", "vercel-ai":
		return "Vercel AI Gateway"
	case "moonshot", "kimi":
		return "Moonshot"
	case "bedrock", "aws-bedrock":
		return "Amazon Bedrock"
	default:
		if provider == "" {
			return "Provider"
		}
		return strings.ToUpper(provider[:1]) + provider[1:]
	}
}
