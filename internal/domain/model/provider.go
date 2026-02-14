// Package model defines the model provider domain.
package model

import "context"

// Provider represents an AI model provider.
type Provider interface {
	// Name returns the provider name (e.g., "anthropic", "openai").
	Name() string
	
	// Models returns the list of available models.
	Models() []Model
	
	// Chat sends a chat request and returns a response.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	
	// Stream sends a chat request and streams the response.
	Stream(ctx context.Context, req *ChatRequest, handler StreamHandler) error
}

// Model represents an AI model.
type Model struct {
	ID          string   // e.g., "claude-opus-4", "gpt-4o"
	Name        string   // Display name
	Provider    string   // Provider name
	Description string   // Model description
	MaxTokens   int      // Maximum context tokens
	Capabilities []string // e.g., "vision", "tools", "thinking"
}

// ChatRequest represents a chat request.
type ChatRequest struct {
	Model         string
	Messages      []Message
	SystemPrompt  string
	MaxTokens     int
	Temperature   float64
	Tools         []Tool
	ThinkingLevel string // "off", "low", "medium", "high", "max"
}

// Message represents a chat message.
type Message struct {
	Role    string        // "user", "assistant", "system"
	Content []ContentPart // Text, images, tool calls, etc.
}

// ContentPart represents a part of message content.
type ContentPart struct {
	Type string // "text", "image", "tool_use", "tool_result"
	
	// Text content
	Text string
	
	// Image content
	ImageURL string
	ImageData string
	
	// Tool use
	ToolCallID string
	ToolName   string
	ToolInput  map[string]any
	
	// Tool result
	ToolResult any
	IsError    bool
}

// Tool represents a tool definition.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ChatResponse represents a chat response.
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	StopReason   string
	Usage        TokenUsage
	ThinkingText string // Extended thinking output
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
}

// StreamHandler handles streaming responses.
type StreamHandler func(chunk StreamChunk) error

// StreamChunk represents a chunk of streaming response.
type StreamChunk struct {
	Type    string // "text", "tool_call", "thinking", "done"
	Content string
	ToolCall *ToolCall
	Usage    *TokenUsage
}

// Registry manages all model providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new model registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register registers a provider.
func (r *Registry) Register(provider Provider) {
	r.providers[provider.Name()] = provider
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// All returns all registered providers.
func (r *Registry) All() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// AllModels returns all available models from all providers.
func (r *Registry) AllModels() []Model {
	var models []Model
	for _, p := range r.providers {
		models = append(models, p.Models()...)
	}
	return models
}

// FindModel finds a model by ID across all providers.
func (r *Registry) FindModel(modelID string) (*Model, Provider, bool) {
	for _, p := range r.providers {
		for _, m := range p.Models() {
			if m.ID == modelID {
				return &m, p, true
			}
		}
	}
	return nil, nil, false
}

