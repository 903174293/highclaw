package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/highclaw/highclaw/internal/config"
)

type SecurityPolicy struct {
	allowed map[string]struct{}
}

func NewSecurityPolicy(cfg *config.Config) *SecurityPolicy {
	allowed := map[string]struct{}{}
	// Baseline allowlist. Keep network fetch commands enabled for parity with
	// weather/tool workflows commonly used in ZeroClaw-style prompts.
	list := []string{"git", "npm", "cargo", "ls", "cat", "grep", "find", "echo", "pwd", "wc", "head", "tail", "curl", "wget"}
	// Merge user-defined entries instead of replacing baseline, because some
	// existing configs store tool ids here (e.g. "bash"), not OS command names.
	list = append(list, cfg.Agent.Sandbox.Allow...)
	for _, cmd := range list {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" {
			allowed[cmd] = struct{}{}
		}
	}
	return &SecurityPolicy{allowed: allowed}
}

func (p *SecurityPolicy) ValidateBashInput(inputJSON string) error {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &in); err != nil {
		return fmt.Errorf("invalid bash input json: %w", err)
	}
	cmd := strings.TrimSpace(in.Command)
	if cmd == "" {
		return fmt.Errorf("command is required")
	}

	lc := strings.ToLower(cmd)
	if strings.Contains(lc, "`") || strings.Contains(lc, "$(") || strings.Contains(lc, "${") || strings.Contains(lc, ">") {
		return fmt.Errorf("command blocked by policy")
	}

	hasCommand := false
	for _, seg := range splitCommandSegments(cmd) {
		base := baseCommand(seg)
		if base == "" {
			continue
		}
		hasCommand = true
		if _, ok := p.allowed[base]; !ok {
			return fmt.Errorf("command not allowed by policy: %s", base)
		}
	}
	if !hasCommand {
		return fmt.Errorf("command is required")
	}

	return nil
}

func splitCommandSegments(command string) []string {
	normalized := command
	for _, sep := range []string{"&&", "||", "\n", ";", "|"} {
		normalized = strings.ReplaceAll(normalized, sep, "\x00")
	}
	parts := strings.Split(normalized, "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func baseCommand(segment string) string {
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return ""
	}
	first := fields[0]
	// Skip env assignment: FOO=bar cmd
	if strings.Contains(first, "=") && len(fields) > 1 {
		first = fields[1]
	}
	if i := strings.LastIndex(first, "/"); i >= 0 {
		first = first[i+1:]
	}
	return strings.TrimSpace(first)
}
