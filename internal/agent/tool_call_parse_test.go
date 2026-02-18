package agent

import "testing"

func TestExtractJSONValuesCapturesNonOverlappingValues(t *testing.T) {
	input := `prefix {"name":"shell","arguments":{"command":"echo hi"}} suffix {"name":"shell","arguments":{"command":"echo hi"}}`
	values := extractJSONValues(input)
	if len(values) != 2 {
		t.Fatalf("expected 2 non-overlapping JSON values, got %d", len(values))
	}
}

func TestParseToolCallsFromInvokeViaFallbackJSONScan(t *testing.T) {
	resp := `<invoke>{"name":"shell","arguments":{"command":"echo hi"}}</invoke>`
	_, calls := parseToolCalls(resp)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call from fallback JSON scan, got %d", len(calls))
	}
	if calls[0].Name != "shell" {
		t.Fatalf("expected shell tool call, got %s", calls[0].Name)
	}
}
