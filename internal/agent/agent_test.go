package agent

import "testing"

func TestParseToolCallsExtractsSingleCall(t *testing.T) {
	response := `Let me check that.
<tool_call>
{"name":"shell","arguments":{"command":"ls -la"}}
</tool_call>`

	text, calls := parseToolCalls(response)
	if text != "Let me check that." {
		t.Fatalf("unexpected text: %q", text)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "shell" {
		t.Fatalf("unexpected tool name: %s", calls[0].Name)
	}
	if string(calls[0].Arguments) != `{"command":"ls -la"}` {
		t.Fatalf("unexpected arguments: %s", string(calls[0].Arguments))
	}
}

func TestParseToolCallsOpenAIFormat(t *testing.T) {
	response := `{"content":"Let me check that for you.","tool_calls":[{"type":"function","function":{"name":"shell","arguments":"{\"command\":\"ls -la\"}"}}]}`

	text, calls := parseToolCalls(response)
	if text != "Let me check that for you." {
		t.Fatalf("unexpected text: %q", text)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "shell" {
		t.Fatalf("unexpected tool name: %s", calls[0].Name)
	}
	if string(calls[0].Arguments) != `{"command":"ls -la"}` {
		t.Fatalf("unexpected arguments: %s", string(calls[0].Arguments))
	}
}

func TestParseToolCallsOpenAIFormatMultipleCalls(t *testing.T) {
	response := `{"tool_calls":[{"type":"function","function":{"name":"file_read","arguments":"{\"path\":\"a.txt\"}"}},{"type":"function","function":{"name":"file_read","arguments":"{\"path\":\"b.txt\"}"}}]}`

	_, calls := parseToolCalls(response)
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Name != "file_read" || calls[1].Name != "file_read" {
		t.Fatalf("unexpected names: %s / %s", calls[0].Name, calls[1].Name)
	}
}

func TestParseToolCallsOpenAIFormatWithoutContent(t *testing.T) {
	response := `{"tool_calls":[{"type":"function","function":{"name":"memory_recall","arguments":"{}"}}]}`

	text, calls := parseToolCalls(response)
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "memory_recall" {
		t.Fatalf("unexpected name: %s", calls[0].Name)
	}
}

func TestParseToolCallsHandlesNoisyTagBody(t *testing.T) {
	response := `<tool_call>
I will now call the tool with this payload:
{"name":"shell","arguments":{"command":"pwd"}}
</tool_call>`

	text, calls := parseToolCalls(response)
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "shell" {
		t.Fatalf("unexpected tool name: %s", calls[0].Name)
	}
	if string(calls[0].Arguments) != `{"command":"pwd"}` {
		t.Fatalf("unexpected arguments: %s", string(calls[0].Arguments))
	}
}
