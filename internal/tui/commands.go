// Package tui implements command system for TUI.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// Command represents a TUI slash command
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Category    string
	Handler     func(m *Model, args []string) (string, error)
}

// getBuiltinCommands returns the list of all built-in commands
func getBuiltinCommands() []Command {
	return []Command{
		// Session commands
		{Name: "sessions", Aliases: []string{"session", "s"}, Description: "List all sessions", Category: "Session", Handler: cmdListSessions},
		{Name: "switch", Aliases: []string{"sw"}, Description: "Switch to a session", Category: "Session", Handler: cmdSwitchSession},
		{Name: "new", Aliases: []string{"n"}, Description: "Create new session", Category: "Session", Handler: cmdNewSession},
		{Name: "delete", Aliases: []string{"del", "rm"}, Description: "Delete a session", Category: "Session", Handler: cmdDeleteSession},
		{Name: "rename", Aliases: []string{"ren"}, Description: "Rename current session", Category: "Session", Handler: cmdRenameSession},

		// Model commands
		{Name: "model", Aliases: []string{"m"}, Description: "Switch model", Category: "Model", Handler: cmdSwitchModel},
		{Name: "models", Aliases: []string{"ml"}, Description: "List models", Category: "Model", Handler: cmdListModels},

		// System commands
		{Name: "clear", Aliases: []string{"cls", "c"}, Description: "Clear chat", Category: "System", Handler: cmdClearChat},
		{Name: "reload", Aliases: []string{"r"}, Description: "Reload sessions", Category: "System", Handler: cmdReload},
		{Name: "help", Aliases: []string{"h", "?"}, Description: "Show help", Category: "System", Handler: cmdHelp},
		{Name: "quit", Aliases: []string{"q", "exit"}, Description: "Quit TUI", Category: "System", Handler: cmdQuit},

		// Shell commands
		{Name: "sh", Aliases: []string{"shell", "!"}, Description: "Run shell command", Category: "Shell", Handler: cmdShell},
		{Name: "cd", Aliases: nil, Description: "Change directory", Category: "Shell", Handler: cmdCd},
		{Name: "pwd", Aliases: nil, Description: "Print working dir", Category: "Shell", Handler: cmdPwd},

		// Info commands
		{Name: "tokens", Aliases: []string{"token", "t"}, Description: "Show token usage", Category: "Info", Handler: cmdTokens},
		{Name: "info", Aliases: []string{"i"}, Description: "Show session info", Category: "Info", Handler: cmdInfo},
		{Name: "copy", Aliases: []string{"cp"}, Description: "Copy last response", Category: "Info", Handler: cmdCopy},
	}
}

func findCommand(name string) *Command {
	name = strings.ToLower(strings.TrimSpace(name))
	cmds := getBuiltinCommands()
	for i := range cmds {
		if cmds[i].Name == name {
			return &cmds[i]
		}
		for _, alias := range cmds[i].Aliases {
			if alias == name {
				return &cmds[i]
			}
		}
	}
	return nil
}

func parseCommand(input string) (name string, args []string) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return "", nil
	}
	input = strings.TrimPrefix(input, "/")
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func cmdListSessions(m *Model, args []string) (string, error) {
	if len(m.sessions) == 0 {
		return "No sessions found.", nil
	}
	var b strings.Builder
	b.WriteString("Sessions:\n")
	for i, s := range m.sessions {
		cur := " "
		if s.Key == m.currentSession {
			cur = "*"
		}
		b.WriteString(fmt.Sprintf(" %s [%d] %s (%s)\n", cur, i, s.Key, relativeTime(s.UpdatedAt)))
	}
	return b.String(), nil
}

func cmdSwitchSession(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return "Usage: /switch <name-or-index>", nil
	}
	target := strings.Join(args, " ")
	var idx int
	if _, err := fmt.Sscanf(target, "%d", &idx); err == nil && idx >= 0 && idx < len(m.sessions) {
		m.currentSession = m.sessions[idx].Key
		m.loadCurrentSession()
		return fmt.Sprintf("Switched to: %s", m.currentSession), nil
	}
	for _, s := range m.sessions {
		if strings.Contains(strings.ToLower(s.Key), strings.ToLower(target)) {
			m.currentSession = s.Key
			m.loadCurrentSession()
			return fmt.Sprintf("Switched to: %s", m.currentSession), nil
		}
	}
	return fmt.Sprintf("Session not found: %s", target), nil
}

func cmdNewSession(m *Model, args []string) (string, error) {
	m.startNewSession()
	return fmt.Sprintf("Created: %s", m.currentSession), nil
}

func cmdDeleteSession(m *Model, args []string) (string, error) {
	return "Delete not implemented yet.", nil
}

func cmdRenameSession(m *Model, args []string) (string, error) {
	return "Rename not implemented yet.", nil
}

func cmdSwitchModel(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return fmt.Sprintf("Current: %s\nUsage: /model <name>", m.cfg.Agent.Model), nil
	}
	m.cfg.Agent.Model = strings.Join(args, " ")
	return fmt.Sprintf("Model: %s", m.cfg.Agent.Model), nil
}

func cmdListModels(m *Model, args []string) (string, error) {
	models := []string{"gpt-4o", "gpt-4o-mini", "claude-3-5-sonnet-20241022", "glm-4-flash", "deepseek-chat"}
	var b strings.Builder
	b.WriteString("Models:\n")
	for _, model := range models {
		cur := " "
		if model == m.cfg.Agent.Model {
			cur = "*"
		}
		b.WriteString(fmt.Sprintf(" %s %s\n", cur, model))
	}
	return b.String(), nil
}

func cmdClearChat(m *Model, args []string) (string, error) {
	m.lines = nil
	m.history = nil
	m.updateViewport()
	return "Chat cleared.", nil
}

func cmdReload(m *Model, args []string) (string, error) {
	return "__RELOAD__", nil
}

func cmdHelp(m *Model, args []string) (string, error) {
	cmds := getBuiltinCommands()
	cats := make(map[string][]Command)
	for _, cmd := range cmds {
		cats[cmd.Category] = append(cats[cmd.Category], cmd)
	}
	var b strings.Builder
	b.WriteString("Commands:\n\n")
	keys := make([]string, 0, len(cats))
	for k := range cats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, cat := range keys {
		cmdList := cats[cat]
		b.WriteString(fmt.Sprintf("[%s]\n", cat))
		for _, cmd := range cmdList {
			als := ""
			if len(cmd.Aliases) > 0 {
				als = fmt.Sprintf(" (/%s)", strings.Join(cmd.Aliases, ", /"))
			}
			b.WriteString(fmt.Sprintf("  /%s%s - %s\n", cmd.Name, als, cmd.Description))
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func cmdQuit(m *Model, args []string) (string, error) {
	return "__QUIT__", nil
}

func cmdShell(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return "Usage: /sh <command>", nil
	}
	cmdStr := strings.Join(args, " ")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("$ %s\nError: %v\n%s", cmdStr, err, string(output)), nil
	}
	return fmt.Sprintf("$ %s\n%s", cmdStr, string(output)), nil
}

func cmdCd(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return "Usage: /cd <dir>", nil
	}
	dir := strings.Join(args, " ")
	if err := os.Chdir(dir); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	pwd, _ := os.Getwd()
	return fmt.Sprintf("Changed to: %s", pwd), nil
}

func cmdPwd(m *Model, args []string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return pwd, nil
}

func cmdTokens(m *Model, args []string) (string, error) {
	return fmt.Sprintf("Tokens:\n  Input:  %d\n  Output: %d\n  Total:  %d",
		m.tokenUsage.input, m.tokenUsage.output, m.tokenUsage.input+m.tokenUsage.output), nil
}

func cmdInfo(m *Model, args []string) (string, error) {
	status := "Offline"
	if m.reachable {
		status = "Online"
	}
	return fmt.Sprintf("Info:\n  Session: %s\n  Model:   %s\n  Agent:   %s\n  Msgs:    %d\n  Status:  %s",
		m.currentSession, m.cfg.Agent.Model, m.opts.Agent, len(m.history), status), nil
}

func cmdCopy(m *Model, args []string) (string, error) {
	for i := len(m.lines) - 1; i >= 0; i-- {
		if m.lines[i].Role == "assistant" {
			return fmt.Sprintf("Copied: %s...", truncStr(m.lines[i].Content, 50)), nil
		}
	}
	return "No assistant message to copy.", nil
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
