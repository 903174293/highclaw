// Package providers implements AI model provider clients.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicClient implements the Anthropic Claude API client.
type AnthropicClient struct {
	APIKey  string
	BaseURL string
	client  *http.Client
}

// NewAnthropicClient creates a new Anthropic API client.
func NewAnthropicClient(apiKey string) *AnthropicClient {
	return NewAnthropicClientWithBaseURL(apiKey, "https://api.anthropic.com/v1")
}

// NewAnthropicClientWithBaseURL creates a new Anthropic-compatible API client.
func NewAnthropicClientWithBaseURL(apiKey, baseURL string) *AnthropicClient {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	base = strings.TrimRight(base, "/")
	return &AnthropicClient{
		APIKey:  apiKey,
		BaseURL: base,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ChatRequest represents a request to the Anthropic Messages API.
type ChatRequest struct {
	Model         string    `json:"model"`
	MaxTokens     int       `json:"max_tokens"`
	Messages      []Message `json:"messages"`
	System        string    `json:"system,omitempty"`
	Temperature   float64   `json:"temperature,omitempty"`
	Tools         []Tool    `json:"tools,omitempty"`
	ThinkingLevel string    `json:"-"` // Not sent to API, used for extended thinking
	Stream        bool      `json:"stream,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string         `json:"role"` // "user" or "assistant"
	Content []ContentBlock `json:"content"`
}

// ContentBlock can be text, image, or tool use/result.
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "tool_use", "tool_result"

	// For text blocks
	Text string `json:"text,omitempty"`

	// For image blocks
	Source *ImageSource `json:"source,omitempty"`

	// For tool_use blocks
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result blocks
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ImageSource represents an image in base64 format.
type ImageSource struct {
	Type      string `json:"type"` // "base64"
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Tool represents a tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ChatResponse represents the response from Anthropic API.
type ChatResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read_input_tokens,omitempty"`
	CacheWrite   int `json:"cache_creation_input_tokens,omitempty"`
}

// Chat sends a chat request to the Anthropic API.
func (c *AnthropicClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Apply extended thinking if requested.
	if req.ThinkingLevel != "" && req.ThinkingLevel != "off" {
		req.Model = c.applyExtendedThinking(req.Model, req.ThinkingLevel)
	}

	// Default max_tokens if not set.
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/messages", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if isAnthropicSetupToken(c.APIKey) {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	} else {
		httpReq.Header.Set("x-api-key", c.APIKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, newAPIError(resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &chatResp, nil
}

func isAnthropicSetupToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), "sk-ant-oat01-")
}

// applyExtendedThinking applies extended thinking mode to the model name.
func (c *AnthropicClient) applyExtendedThinking(model, level string) string {
	// Extended thinking is enabled by appending thinking budget to model name.
	// Levels: low (10k tokens), medium (20k), high (40k), max (100k)
	switch level {
	case "low":
		return model + ":thinking:10000"
	case "medium":
		return model + ":thinking:20000"
	case "high":
		return model + ":thinking:40000"
	case "max":
		return model + ":thinking:100000"
	default:
		return model
	}
}

// ExtractTextContent extracts all text from content blocks.
func ExtractTextContent(blocks []ContentBlock) string {
	var text string
	for _, block := range blocks {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}

// ExtractToolUses extracts all tool_use blocks from content.
func ExtractToolUses(blocks []ContentBlock) []ContentBlock {
	var tools []ContentBlock
	for _, block := range blocks {
		if block.Type == "tool_use" {
			tools = append(tools, block)
		}
	}
	return tools
}
