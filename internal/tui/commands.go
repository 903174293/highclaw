// Package tui 提供命令处理
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/highclaw/highclaw/internal/gateway/session"
)

// Command 定义斜杠命令
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Category    string
	Handler     func(m *Model, args []string) (string, error)
}

// getBuiltinCommands 返回内置命令列表
func getBuiltinCommands() []Command {
	return []Command{
		// Session 命令
		{Name: "sessions", Aliases: []string{"s"}, Description: "List all sessions", Category: "Session", Handler: cmdListSessions},
		{Name: "switch", Aliases: []string{"sw"}, Description: "Switch to another session", Category: "Session", Handler: cmdSwitchSession},
		{Name: "new", Aliases: []string{"n"}, Description: "Start a new session", Category: "Session", Handler: cmdNewSession},
		{Name: "delete", Aliases: []string{"rm"}, Description: "Delete a session", Category: "Session", Handler: cmdDeleteSession},
		{Name: "rename", Aliases: nil, Description: "Rename current session", Category: "Session", Handler: cmdRenameSession},
		{Name: "compact", Aliases: nil, Description: "Compact session history", Category: "Session", Handler: cmdCompact},

		// Model 命令
		{Name: "model", Aliases: []string{"m"}, Description: "Switch model", Category: "Model", Handler: cmdSetModel},
		{Name: "models", Aliases: nil, Description: "List available models", Category: "Model", Handler: cmdListModels},

		// 系统命令
		{Name: "clear", Aliases: []string{"c"}, Description: "Clear chat display", Category: "System", Handler: cmdClear},
		{Name: "reload", Aliases: nil, Description: "Reload configuration", Category: "System", Handler: cmdReload},
		{Name: "help", Aliases: []string{"h", "?"}, Description: "Show help", Category: "System", Handler: cmdHelp},
		{Name: "quit", Aliases: []string{"q", "exit"}, Description: "Exit TUI", Category: "System", Handler: cmdQuit},

		// Shell 命令
		{Name: "sh", Aliases: []string{"shell", "!"}, Description: "Run shell command", Category: "Shell", Handler: cmdShell},
		{Name: "cd", Aliases: nil, Description: "Change directory", Category: "Shell", Handler: cmdCd},
		{Name: "pwd", Aliases: nil, Description: "Print working directory", Category: "Shell", Handler: cmdPwd},

		// 信息命令
		{Name: "tokens", Aliases: []string{"t"}, Description: "Show token usage", Category: "Info", Handler: cmdTokens},
		{Name: "info", Aliases: nil, Description: "Show session info", Category: "Info", Handler: cmdInfo},
	}
}

// parseCommand 解析命令和参数
func parseCommand(text string) (string, []string) {
	text = strings.TrimPrefix(strings.TrimSpace(text), "/")
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}
	return strings.ToLower(parts[0]), parts[1:]
}

// findCommand 查找命令
func findCommand(name string) *Command {
	name = strings.ToLower(name)
	for _, cmd := range getBuiltinCommands() {
		if cmd.Name == name {
			return &cmd
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return &cmd
			}
		}
	}
	return nil
}

// Command handlers

func cmdListSessions(m *Model, args []string) (string, error) {
	if len(m.sessions) == 0 {
		return "No sessions found", nil
	}
	var b strings.Builder
	b.WriteString("Sessions:\n")
	for i, s := range m.sessions {
		marker := "  "
		if s.Key == m.currentSession {
			marker = "→ "
		}
		label := lastSegment(s.Key)
		updated := relativeTime(s.UpdatedAt)
		b.WriteString(fmt.Sprintf("%s%d. %s (%s)\n", marker, i+1, label, updated))
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func cmdSwitchSession(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: /switch <session-name or number>")
	}
	target := strings.TrimSpace(args[0])

	// 尝试数字索引
	var idx int
	if n, _ := fmt.Sscanf(target, "%d", &idx); n == 1 && idx > 0 && idx <= len(m.sessions) {
		m.currentSession = m.sessions[idx-1].Key
		_ = session.SetCurrent(m.currentSession)
		m.loadCurrentSession()
		return fmt.Sprintf("Switched to: %s", lastSegment(m.currentSession)), nil
	}

	// 尝试名称匹配
	for _, s := range m.sessions {
		if strings.Contains(strings.ToLower(s.Key), strings.ToLower(target)) {
			m.currentSession = s.Key
			_ = session.SetCurrent(m.currentSession)
			m.loadCurrentSession()
			return fmt.Sprintf("Switched to: %s", lastSegment(m.currentSession)), nil
		}
	}

	return "", fmt.Errorf("session not found: %s", target)
}

func cmdNewSession(m *Model, args []string) (string, error) {
	m.startNewSession()
	return fmt.Sprintf("New session: %s", lastSegment(m.currentSession)), nil
}

func cmdDeleteSession(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: /delete <session-name or number>")
	}
	target := strings.TrimSpace(args[0])

	var keyToDelete string
	var idx int
	if n, _ := fmt.Sscanf(target, "%d", &idx); n == 1 && idx > 0 && idx <= len(m.sessions) {
		keyToDelete = m.sessions[idx-1].Key
	} else {
		for _, s := range m.sessions {
			if strings.Contains(strings.ToLower(s.Key), strings.ToLower(target)) {
				keyToDelete = s.Key
				break
			}
		}
	}

	if keyToDelete == "" {
		return "", fmt.Errorf("session not found: %s", target)
	}

	if keyToDelete == m.currentSession {
		return "", fmt.Errorf("cannot delete current session")
	}

	if err := session.Delete(keyToDelete); err != nil {
		return "", err
	}

	// 刷新列表
	newSessions := make([]sessionEntry, 0, len(m.sessions)-1)
	for _, s := range m.sessions {
		if s.Key != keyToDelete {
			newSessions = append(newSessions, s)
		}
	}
	m.sessions = newSessions

	return fmt.Sprintf("Deleted: %s", lastSegment(keyToDelete)), nil
}

func cmdRenameSession(m *Model, args []string) (string, error) {
	return "", fmt.Errorf("rename not implemented yet")
}

func cmdCompact(m *Model, args []string) (string, error) {
	return "Compact not implemented yet. Use /new to start fresh.", nil
}

func cmdSetModel(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return fmt.Sprintf("Current model: %s", m.cfg.Agent.Model), nil
	}
	newModel := strings.TrimSpace(args[0])
	m.cfg.Agent.Model = newModel
	return fmt.Sprintf("Model set to: %s", newModel), nil
}

func cmdListModels(m *Model, args []string) (string, error) {
	models := []string{
		"claude-sonnet-4-20250514",
		"claude-3-5-sonnet-20241022",
		"gpt-4o",
		"gpt-4o-mini",
		"deepseek-chat",
		"deepseek-reasoner",
	}
	var b strings.Builder
	b.WriteString("Available models:\n")
	for _, model := range models {
		marker := "  "
		if model == m.cfg.Agent.Model {
			marker = "→ "
		}
		b.WriteString(marker + model + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func cmdClear(m *Model, args []string) (string, error) {
	m.lines = nil
	m.updateViewport()
	return "", nil
}

func cmdReload(m *Model, args []string) (string, error) {
	return "__RELOAD__", nil
}

func cmdHelp(m *Model, args []string) (string, error) {
	commands := getBuiltinCommands()
	byCategory := make(map[string][]Command)
	for _, cmd := range commands {
		byCategory[cmd.Category] = append(byCategory[cmd.Category], cmd)
	}

	var categories []string
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	var b strings.Builder
	b.WriteString("Commands:\n")
	for _, cat := range categories {
		b.WriteString(fmt.Sprintf("\n[%s]\n", cat))
		for _, cmd := range byCategory[cat] {
			aliases := ""
			if len(cmd.Aliases) > 0 {
				aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
			}
			b.WriteString(fmt.Sprintf("  /%s%s - %s\n", cmd.Name, aliases, cmd.Description))
		}
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func cmdQuit(m *Model, args []string) (string, error) {
	return "__QUIT__", nil
}

func cmdShell(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: /sh <command>")
	}
	cmdStr := strings.Join(args, " ")
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Sprintf("$ %s\n%s\nError: %v", cmdStr, result, err), nil
	}
	return fmt.Sprintf("$ %s\n%s", cmdStr, result), nil
}

func cmdCd(m *Model, args []string) (string, error) {
	if len(args) == 0 {
		home, _ := os.UserHomeDir()
		if home != "" {
			if err := os.Chdir(home); err != nil {
				return "", err
			}
			return home, nil
		}
		return "", fmt.Errorf("no HOME directory")
	}
	target := strings.TrimSpace(args[0])
	if err := os.Chdir(target); err != nil {
		return "", err
	}
	pwd, _ := os.Getwd()
	return pwd, nil
}

func cmdPwd(m *Model, args []string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return pwd, nil
}

func cmdTokens(m *Model, args []string) (string, error) {
	return fmt.Sprintf("Input: %d, Output: %d, Total: %d",
		m.tokenUsage.input, m.tokenUsage.output, m.tokenUsage.input+m.tokenUsage.output), nil
}

func cmdInfo(m *Model, args []string) (string, error) {
	pwd, _ := os.Getwd()
	var b strings.Builder
	b.WriteString("Session Info:\n")
	b.WriteString(fmt.Sprintf("  Session: %s\n", m.currentSession))
	b.WriteString(fmt.Sprintf("  Model: %s\n", m.cfg.Agent.Model))
	b.WriteString(fmt.Sprintf("  Messages: %d\n", len(m.history)))
	b.WriteString(fmt.Sprintf("  Tokens: %d (in) / %d (out)\n", m.tokenUsage.input, m.tokenUsage.output))
	b.WriteString(fmt.Sprintf("  CWD: %s", pwd))
	return b.String(), nil
}
