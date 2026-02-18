package providers

import (
	"strings"
	"testing"
)

func TestScrubSecretPatterns(t *testing.T) {
	in := `{"error":"bad key sk-abc123xyz and slack xoxb-foo-bar and xoxp-hello"}`
	out := scrubSecretPatterns(in)
	if strings.Contains(out, "sk-abc123xyz") || strings.Contains(out, "xoxb-foo-bar") || strings.Contains(out, "xoxp-hello") {
		t.Fatalf("expected secret-like tokens to be redacted, got: %s", out)
	}
	if strings.Count(out, "[REDACTED]") < 3 {
		t.Fatalf("expected multiple redactions, got: %s", out)
	}
}

func TestSanitizeAPIErrorTruncates(t *testing.T) {
	in := strings.Repeat("x", maxAPIErrorChars+20)
	out := sanitizeAPIError(in)
	if len([]rune(out)) <= maxAPIErrorChars {
		t.Fatalf("expected ellipsis after truncation, got len=%d", len([]rune(out)))
	}
	if !strings.HasSuffix(out, "...") {
		t.Fatalf("expected ellipsis suffix, got: %s", out)
	}
}

func TestNewAPIErrorSanitizes(t *testing.T) {
	err := newAPIError(400, "token sk-secret should not leak")
	if !strings.Contains(err.Body, "[REDACTED]") {
		t.Fatalf("expected sanitized body, got: %s", err.Body)
	}
}
