package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchInput represents the input to the web_search tool.
type WebSearchInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"maxResults,omitempty"`
}

// WebSearchResult represents a single search result.
type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WebSearchOutput represents the output from web_search.
type WebSearchOutput struct {
	Query   string            `json:"query"`
	Results []WebSearchResult `json:"results"`
}

// WebSearch performs a web search using DuckDuckGo Instant Answer API.
// This is a simple implementation; production should use Google Custom Search or similar.
func WebSearch(ctx context.Context, inputJSON string) (string, error) {
	var input WebSearchInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("invalid web_search input: %w", err)
	}

	if input.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	if input.MaxResults == 0 {
		input.MaxResults = 5
	}

	// Use DuckDuckGo HTML search (simple scraping approach)
	// For production, use Google Custom Search API or similar.
	results, err := searchDuckDuckGo(ctx, input.Query, input.MaxResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	output := WebSearchOutput{
		Query:   input.Query,
		Results: results,
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("marshal output: %w", err)
	}

	return string(result), nil
}

// searchDuckDuckGo performs a simple DuckDuckGo search.
// This is a placeholder implementation.
func searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	// Build DuckDuckGo search URL
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; HighClaw/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Parse HTML results (simplified - production should use proper HTML parser)
	results := parseSearchResults(string(body), maxResults)

	return results, nil
}

// parseSearchResults extracts search results from DuckDuckGo HTML.
// This is a very simplified parser - production should use goquery or similar.
func parseSearchResults(html string, maxResults int) []WebSearchResult {
	results := make([]WebSearchResult, 0, maxResults)
	marker := `class="result__a" href="`
	offset := 0

	for len(results) < maxResults {
		i := strings.Index(html[offset:], marker)
		if i < 0 {
			break
		}
		i += offset + len(marker)
		j := strings.Index(html[i:], `"`)
		if j < 0 {
			break
		}
		link := html[i : i+j]
		titleStart := strings.Index(html[i+j:], ">")
		if titleStart < 0 {
			offset = i + j
			continue
		}
		titleStart += i + j + 1
		titleEnd := strings.Index(html[titleStart:], "</a>")
		if titleEnd < 0 {
			offset = titleStart
			continue
		}
		title := stripHTML(html[titleStart : titleStart+titleEnd])
		if title == "" || link == "" {
			offset = titleStart + titleEnd
			continue
		}
		results = append(results, WebSearchResult{
			Title:   title,
			URL:     decodeDuckURL(link),
			Snippet: "",
		})
		offset = titleStart + titleEnd
	}

	return results
}

// BraveSearch uses Brave Search API (requires API key).
func BraveSearch(ctx context.Context, apiKey, query string, maxResults int) ([]WebSearchResult, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("api key is required")
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	u := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d", url.QueryEscape(query), maxResults)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave search returned status %d", resp.StatusCode)
	}
	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]WebSearchResult, 0, len(payload.Web.Results))
	for _, r := range payload.Web.Results {
		out = append(out, WebSearchResult{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return out, nil
}

// GoogleSearch uses Google Custom Search API (requires API key).
func GoogleSearch(ctx context.Context, apiKey, searchEngineID, query string, maxResults int) ([]WebSearchResult, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(searchEngineID) == "" {
		return nil, fmt.Errorf("api key and search engine id are required")
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	u := fmt.Sprintf(
		"https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=%d",
		url.QueryEscape(apiKey), url.QueryEscape(searchEngineID), url.QueryEscape(query), maxResults,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google search returned status %d", resp.StatusCode)
	}
	var payload struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]WebSearchResult, 0, len(payload.Items))
	for _, it := range payload.Items {
		out = append(out, WebSearchResult{Title: it.Title, URL: it.Link, Snippet: it.Snippet})
	}
	return out, nil
}

func stripHTML(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'", "&quot;", "\"")
	out := r.Replace(s)
	out = strings.ReplaceAll(out, "<b>", "")
	out = strings.ReplaceAll(out, "</b>", "")
	out = strings.TrimSpace(out)
	return out
}

func decodeDuckURL(u string) string {
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	if parsed.Path == "/l/" || parsed.Path == "/l" {
		target := parsed.Query().Get("uddg")
		if target != "" {
			if dec, err := url.QueryUnescape(target); err == nil {
				return dec
			}
			return target
		}
	}
	return u
}
