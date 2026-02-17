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

// OpenAIClient implements the OpenAI Chat Completions API client.
type OpenAIClient struct {
	APIKey  string
	BaseURL string
	client  *http.Client
}

// NewOpenAIClient creates a new OpenAI API client.
func NewOpenAIClient(apiKey string) *OpenAIClient {
	return NewOpenAIClientWithBaseURL(apiKey, "https://api.openai.com/v1")
}

// NewOpenAIClientWithBaseURL creates a new OpenAI-compatible API client.
func NewOpenAIClientWithBaseURL(apiKey, baseURL string) *OpenAIClient {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	base = strings.TrimRight(base, "/")
	return &OpenAIClient{
		APIKey:  apiKey,
		BaseURL: base,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// OpenAIChatRequest represents a request to the OpenAI Chat Completions API.
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// OpenAIMessage represents a message in the OpenAI format.
type OpenAIMessage struct {
	Role      string           `json:"role"` // "system", "user", "assistant"
	Content   any              `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIChatResponse represents the response from OpenAI Chat Completions API.
type OpenAIChatResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []OpenAIChoice   `json:"choices"`
	Usage   OpenAIUsageStats `json:"usage"`
}

// OpenAIChoice represents a single completion choice.
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIToolCall represents OpenAI-compatible function tool call metadata.
type OpenAIToolCall struct {
	Type     string         `json:"type,omitempty"`
	Function OpenAIFunction `json:"function,omitempty"`
}

// OpenAIFunction represents a function call payload.
type OpenAIFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// OpenAIUsageStats tracks OpenAI token consumption.
type OpenAIUsageStats struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// APIError represents a non-2xx provider HTTP response.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// Chat sends a chat request to the OpenAI API.
func (c *OpenAIClient) Chat(ctx context.Context, req *OpenAIChatRequest) (*OpenAIChatResponse, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 1200
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

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
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var chatResp OpenAIChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &chatResp, nil
}
