package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/highclaw/highclaw/internal/config"
)

type embeddingProvider interface {
	name() string
	dimensions() int
	embedOne(text string) ([]float32, error)
	embedBatch(texts []string) ([][]float32, error)
}

type noopEmbedding struct{}

func (n noopEmbedding) name() string                       { return "none" }
func (n noopEmbedding) dimensions() int                    { return 0 }
func (n noopEmbedding) embedOne(string) ([]float32, error) { return nil, nil }
func (n noopEmbedding) embedBatch(texts []string) ([][]float32, error) {
	return make([][]float32, len(texts)), nil
}

type openAIEmbedding struct {
	baseURL string
	apiKey  string
	model   string
	dims    int
	client  *http.Client
}

func (o *openAIEmbedding) name() string    { return "openai" }
func (o *openAIEmbedding) dimensions() int { return o.dims }

func (o *openAIEmbedding) embeddingsURL() string {
	base := strings.TrimRight(strings.TrimSpace(o.baseURL), "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	if strings.HasSuffix(base, "/embeddings") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/embeddings"
	}
	if strings.Contains(base, "/api/") {
		return base + "/embeddings"
	}
	return base + "/v1/embeddings"
}

func (o *openAIEmbedding) embedOne(text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	if strings.TrimSpace(o.apiKey) == "" {
		return nil, nil
	}
	body := map[string]any{
		"model": o.model,
		"input": []string{text},
	}
	if o.dims > 0 {
		body["dimensions"] = o.dims
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, o.embeddingsURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error map[string]any `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding API error %d", resp.StatusCode)
	}
	if len(out.Data) == 0 {
		return nil, nil
	}
	return out.Data[0].Embedding, nil
}

// embedBatch 批量获取 embedding，一次 API 调用处理多个文本
func (o *openAIEmbedding) embedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(o.apiKey) == "" {
		return make([][]float32, len(texts)), nil
	}
	cleaned := make([]string, 0, len(texts))
	idxMap := make([]int, 0, len(texts))
	for i, t := range texts {
		if strings.TrimSpace(t) != "" {
			cleaned = append(cleaned, t)
			idxMap = append(idxMap, i)
		}
	}
	if len(cleaned) == 0 {
		return make([][]float32, len(texts)), nil
	}

	const batchSize = 100
	result := make([][]float32, len(texts))

	for start := 0; start < len(cleaned); start += batchSize {
		end := start + batchSize
		if end > len(cleaned) {
			end = len(cleaned)
		}
		chunk := cleaned[start:end]

		body := map[string]any{
			"model": o.model,
			"input": chunk,
		}
		if o.dims > 0 {
			body["dimensions"] = o.dims
		}
		payload, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, o.embeddingsURL(), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := o.client.Do(req)
		if err != nil {
			return nil, err
		}
		var out struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			} `json:"data"`
			Error map[string]any `json:"error"`
		}
		decErr := json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if decErr != nil {
			return nil, decErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("embedding batch API error %d", resp.StatusCode)
		}
		for _, d := range out.Data {
			absIdx := start + d.Index
			if absIdx >= 0 && absIdx < len(idxMap) {
				result[idxMap[absIdx]] = d.Embedding
			}
		}
	}
	return result, nil
}

func createEmbeddingProvider(cfg *config.Config) embeddingProvider {
	if cfg == nil {
		return noopEmbedding{}
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Memory.EmbeddingProvider))
	if provider == "" || provider == "none" {
		return noopEmbedding{}
	}

	model := strings.TrimSpace(cfg.Memory.EmbeddingModel)
	if model == "" {
		model = "text-embedding-3-small"
	}
	dims := cfg.Memory.EmbeddingDimensions

	switch {
	case provider == "openai":
		pcfg, ok := resolveProviderConfig(cfg, "openai")
		if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
			pcfg, ok = resolveProviderConfig(cfg, embeddingFallbackProvider(cfg))
		}
		if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
			return noopEmbedding{}
		}
		baseURL := strings.TrimSpace(pcfg.BaseURL)
		if baseURL == "" {
			baseURL = defaultBaseURLForProvider(embeddingFallbackProvider(cfg))
			if strings.TrimSpace(baseURL) == "" {
				baseURL = "https://api.openai.com"
			}
		}
		return &openAIEmbedding{
			baseURL: baseURL,
			apiKey:  strings.TrimSpace(pcfg.APIKey),
			model:   model,
			dims:    dims,
			client:  &http.Client{Timeout: 60 * time.Second},
		}
	case strings.HasPrefix(provider, "custom:http://"), strings.HasPrefix(provider, "custom:https://"):
		pcfg, ok := resolveProviderConfig(cfg, "openai")
		if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
			pcfg, ok = resolveProviderConfig(cfg, embeddingFallbackProvider(cfg))
		}
		if !ok || strings.TrimSpace(pcfg.APIKey) == "" {
			return noopEmbedding{}
		}
		base := strings.TrimPrefix(provider, "custom:")
		return &openAIEmbedding{
			baseURL: base,
			apiKey:  strings.TrimSpace(pcfg.APIKey),
			model:   model,
			dims:    dims,
			client:  &http.Client{Timeout: 60 * time.Second},
		}
	default:
		return noopEmbedding{}
	}
}

func embeddingFallbackProvider(cfg *config.Config) string {
	if cfg == nil {
		return "openai"
	}
	if prefix, _, ok := splitModelPrefix(strings.TrimSpace(cfg.Agent.Model)); ok {
		return prefix
	}
	if _, ok := resolveProviderConfig(cfg, "openrouter"); ok {
		return "openrouter"
	}
	keys := make([]string, 0, len(cfg.Agent.Providers))
	for k, pcfg := range cfg.Agent.Providers {
		if strings.TrimSpace(pcfg.APIKey) == "" {
			continue
		}
		keys = append(keys, strings.ToLower(strings.TrimSpace(k)))
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return keys[0]
	}
	return "openai"
}
