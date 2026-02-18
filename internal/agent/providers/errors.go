package providers

import (
	"fmt"
	"strings"
)

const maxAPIErrorChars = 200

// APIError represents a non-2xx provider HTTP response.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

func newAPIError(statusCode int, body string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Body:       sanitizeAPIError(body),
	}
}

func sanitizeAPIError(input string) string {
	scrubbed := scrubSecretPatterns(input)
	runes := []rune(scrubbed)
	if len(runes) <= maxAPIErrorChars {
		return scrubbed
	}
	return string(runes[:maxAPIErrorChars]) + "..."
}

func scrubSecretPatterns(input string) string {
	out := input
	for _, prefix := range []string{"sk-", "xoxb-", "xoxp-"} {
		for {
			idx := strings.Index(out, prefix)
			if idx < 0 {
				break
			}
			start := idx
			end := idx + len(prefix)
			for end < len(out) {
				ch := out[end]
				if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
					ch == '-' || ch == '_' || ch == '.' || ch == ':' {
					end++
					continue
				}
				break
			}
			if end == idx+len(prefix) {
				break
			}
			out = out[:start] + "[REDACTED]" + out[end:]
		}
	}
	return out
}
