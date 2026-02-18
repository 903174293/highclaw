// Package tui implements the terminal user interface.
package tui

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/highclaw/highclaw/internal/agent"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
	"github.com/highclaw/highclaw/internal/gateway/session"
)

const (
	sidebarWidth = 42
	inputHeight  = 3
	headerHeight = 2
	footerHeight = 2
)

type focusTarget int

const (
	focusInput focusTarget = iota
	focusSessions
	focusCommands
)

// Options configures TUI startup
type Options struct {
	GatewayURL string
	Agent      string
	Session    string
	Model      string
}

type chatLine struct {
	Role      string
	Content   string
	Timestamp time.Time
}

type sessionEntry struct {
	Key           string
	Label         string
	Channel       string
	UpdatedAt     time.Time
	ModelProvider string
	Model         string
	ContextTokens int
	TotalTokens   int
}

// tokenUsageInfo tracks token consumption
type tokenUsageInfo struct {
	input  int
	output int
}

type bootMsg struct {
	Sessions  []sessionEntry
	Reachable bool
	Err       error
}

type assistantMsg struct {
	Reply        string
	Err          error
	Duration     time.Duration
	InputTokens  int
	OutputTokens int
}

// Model represents the TUI state
type Model struct {
	opts   Options
	cfg    *config.Config
	runner *agent.Runner

	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	history []agent.ChatMessage
	lines   []chatLine

	sessions        []sessionEntry
	selectedSession int
	currentSession  string
	sessionFilter   string

	width  int
	height int
	ready  bool

	focus focusTarget

	reachable  bool
	pending    bool
	lastError  string
	lastRTT    time.Duration
	tokenUsage tokenUsageInfo

	// 消息队列（修复并发发送 bug）
	messageQueue []string

	// 命令模式
	showCommands    bool
	commandFilter   string
	selectedCommand int

	// 启动画面
	showSplash bool
}

// NewModel creates a new TUI model
func NewModel(opts Options) Model {
	if strings.TrimSpace(opts.Agent) == "" {
		opts.Agent = "main"
	}
	if strings.TrimSpace(opts.Session) == "" {
		opts.Session = "main"
	}
	if strings.TrimSpace(opts.GatewayURL) == "" {
		opts.GatewayURL = "ws://127.0.0.1:18789"
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	if strings.TrimSpace(opts.Model) != "" {
		cfg.Agent.Model = strings.TrimSpace(opts.Model)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	ta := textarea.New()
	ta.Placeholder = "Type message or /help for commands..."
	ta.Focus()
	ta.CharLimit = 10000
	ta.SetHeight(inputHeight)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))

	initialSession := buildSessionKey(opts.Agent, opts.Session)
	if strings.TrimSpace(opts.Session) == "" || strings.EqualFold(strings.TrimSpace(opts.Session), "main") {
		if current, err := session.Current(); err == nil && strings.TrimSpace(current) != "" {
			initialSession = strings.TrimSpace(current)
		}
	}

	return Model{
		opts:           opts,
		cfg:            cfg,
		runner:         agent.NewRunner(cfg, logger),
		textarea:       ta,
		viewport:       vp,
		spinner:        sp,
		currentSession: initialSession,
		focus:          focusInput,
		showSplash:     true,
		messageQueue:   make([]string, 0),
	}
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, loadBootCmd(m.opts))
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resize()
		m.updateViewport()
		return m, nil

	case bootMsg:
		m.showSplash = false
		if msg.Err != nil {
			m.lastError = msg.Err.Error()
		}
		m.reachable = msg.Reachable
		if len(msg.Sessions) > 0 {
			m.sessions = msg.Sessions
			m.selectedSession = 0
			if strings.TrimSpace(m.currentSession) == "" {
				m.currentSession = msg.Sessions[0].Key
			}
			for i := range msg.Sessions {
				if msg.Sessions[i].Key == m.currentSession {
					m.selectedSession = i
					break
				}
			}
		}
		m.loadCurrentSession()
		if len(m.lines) == 0 {
			m.appendLine("system", "Welcome to HighClaw! Type /help for commands.")
		}
		m.updateViewport()
		return m, nil

	case assistantMsg:
		m.pending = false
		m.lastRTT = msg.Duration
		m.tokenUsage.input += msg.InputTokens
		m.tokenUsage.output += msg.OutputTokens

		if msg.Err != nil {
			m.lastError = msg.Err.Error()
			m.appendLine("system", "Error: "+msg.Err.Error())
		} else {
			reply := strings.TrimSpace(msg.Reply)
			if reply == "" {
				reply = "(empty response)"
			}
			m.appendLine("assistant", reply)
			m.history = append(m.history, agent.ChatMessage{Role: "assistant", Content: reply})
			m.persistCurrentSession()
		}
		m.updateViewport()

		// 检查消息队列，处理下一条
		if len(m.messageQueue) > 0 {
			nextMsg := m.messageQueue[0]
			m.messageQueue = m.messageQueue[1:]
			m.pending = true
			m.appendLine("user", nextMsg)
			m.history = append(m.history, agent.ChatMessage{Role: "user", Content: nextMsg})
			m.persistCurrentSession()
			m.updateViewport()
			cmds = append(cmds, sendMessageCmd(m.runner, m.currentSession, cloneHistory(m.history)))
			cmds = append(cmds, m.spinner.Tick)
		}
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		if m.pending {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// 处理全局快捷键
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.showCommands {
				m.showCommands = false
				m.commandFilter = ""
				return m, nil
			}
			if m.showSplash {
				m.showSplash = false
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyTab:
			if m.showCommands {
				m.showCommands = false
				m.commandFilter = ""
			}
			if m.focus == focusInput {
				m.focus = focusSessions
				m.textarea.Blur()
			} else {
				m.focus = focusInput
				m.textarea.Focus()
			}
			return m, nil
		case tea.KeyCtrlN:
			m.startNewSession()
			return m, nil
		case tea.KeyCtrlL:
			m.lines = nil
			m.history = nil
			m.updateViewport()
			return m, nil
		case tea.KeyCtrlR:
			return m, loadBootCmd(m.opts)
		case tea.KeyUp:
			if m.focus == focusSessions {
				m.moveSessionSelection(-1)
				return m, nil
			}
			if m.showCommands {
				m.moveCommandSelection(-1)
				return m, nil
			}
		case tea.KeyDown:
			if m.focus == focusSessions {
				m.moveSessionSelection(1)
				return m, nil
			}
			if m.showCommands {
				m.moveCommandSelection(1)
				return m, nil
			}
		case tea.KeyBackspace:
			if m.focus == focusSessions && len(m.sessionFilter) > 0 {
				m.sessionFilter = m.sessionFilter[:len(m.sessionFilter)-1]
				m.selectedSession = 0
				return m, nil
			}
		case tea.KeyEnter:
			if m.showSplash {
				m.showSplash = false
				return m, nil
			}

			if m.showCommands {
				// 执行选中的命令
				filtered := m.filteredCommands()
				if len(filtered) > 0 && m.selectedCommand < len(filtered) {
					cmd := filtered[m.selectedCommand]
					m.showCommands = false
					m.commandFilter = ""
					m.textarea.SetValue("/" + cmd.Name + " ")
				}
				return m, nil
			}

			if m.focus == focusSessions {
				m.activateSelectedSession()
				return m, nil
			}

			text := strings.TrimSpace(m.textarea.Value())
			if text == "" {
				return m, nil
			}
			m.textarea.Reset()
			m.lastError = ""

			// 检查是否是命令
			if strings.HasPrefix(text, "/") {
				cmdName, args := parseCommand(text)
				if cmd := findCommand(cmdName); cmd != nil {
					result, err := cmd.Handler(&m, args)
					if err != nil {
						m.appendLine("system", "Error: "+err.Error())
					} else if result == "__QUIT__" {
						return m, tea.Quit
					} else if result == "__RELOAD__" {
						return m, loadBootCmd(m.opts)
					} else if result != "" {
						m.appendLine("system", result)
					}
					m.updateViewport()
					return m, nil
				}
				m.appendLine("system", "Unknown command: "+cmdName+". Type /help for list.")
				m.updateViewport()
				return m, nil
			}

			// 发送消息（带队列支持）
			if m.pending {
				// 加入队列而不是丢弃
				m.messageQueue = append(m.messageQueue, text)
				m.appendLine("system", fmt.Sprintf("[Queued #%d] %s", len(m.messageQueue), text))
				m.updateViewport()
				return m, nil
			}

			m.pending = true
			m.appendLine("user", text)
			m.history = append(m.history, agent.ChatMessage{Role: "user", Content: text})
			m.persistCurrentSession()
			m.updateViewport()
			cmds = append(cmds, sendMessageCmd(m.runner, m.currentSession, cloneHistory(m.history)))
			cmds = append(cmds, m.spinner.Tick)
			return m, tea.Batch(cmds...)
		}

		// 检测 / 字符开启命令模式
		if m.focus == focusInput && msg.Type == tea.KeyRunes {
			text := m.textarea.Value()
			if text == "" && msg.String() == "/" {
				m.showCommands = true
				m.commandFilter = ""
				m.selectedCommand = 0
			} else if m.showCommands && strings.HasPrefix(text, "/") {
				m.commandFilter = strings.TrimPrefix(text, "/")
				m.selectedCommand = 0
			}
		}

		if m.focus == focusSessions && msg.Type == tea.KeyRunes {
			typed := strings.TrimSpace(msg.String())
			if typed != "" {
				m.sessionFilter += typed
				m.selectedSession = 0
				return m, nil
			}
		}
	}

	// Update focused component
	if m.focus == focusInput {
		var tiCmd tea.Cmd
		m.textarea, tiCmd = m.textarea.Update(msg)
		cmds = append(cmds, tiCmd)

		// 更新命令过滤
		if m.showCommands {
			text := m.textarea.Value()
			if strings.HasPrefix(text, "/") {
				m.commandFilter = strings.TrimPrefix(text, "/")
			} else {
				m.showCommands = false
			}
		}
	}
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	if !m.ready {
		return "\n  Loading HighClaw..."
	}

	if m.showSplash {
		return m.renderSplash()
	}

	header := m.renderHeader()
	body := m.renderBody()
	input := m.renderInput()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, input, footer)
}

func (m *Model) renderSplash() string {
	logo := RenderFullLogo()
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("\n\n    Press Enter to continue...")

	content := logo + hint

	// 居中显示
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(content)
}

func (m *Model) renderHeader() string {
	dim := lipgloss.Color("240")
	accent := lipgloss.Color("226")
	bright := lipgloss.Color("252")

	// 左侧：Logo 和 session
	logo := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(miniLogo)

	sessionLabel := lastSegment(m.currentSession)
	if len(sessionLabel) > 20 {
		sessionLabel = sessionLabel[:19] + "…"
	}

	sep := lipgloss.NewStyle().Foreground(dim).Render(" │ ")

	sessionInfo := lipgloss.NewStyle().Foreground(bright).Render(sessionLabel)
	modelInfo := lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Render(m.cfg.Agent.Model)

	// 右侧：Token 和状态
	tokenInfo := lipgloss.NewStyle().Foreground(dim).Render(
		fmt.Sprintf("%d tokens", m.tokenUsage.input+m.tokenUsage.output))

	netIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("●")
	if m.reachable {
		netIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("●")
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, logo, sep, sessionInfo, sep, modelInfo)
	right := lipgloss.JoinHorizontal(lipgloss.Center, tokenInfo, " ", netIcon)

	// 计算中间空白
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := m.width - leftWidth - rightWidth - 4
	if gap < 1 {
		gap = 1
	}

	headerLine := left + strings.Repeat(" ", gap) + right
	border := lipgloss.NewStyle().Foreground(accent).Render(strings.Repeat("─", m.width-2))

	return lipgloss.NewStyle().Padding(0, 1).Render(headerLine + "\n" + border)
}

func (m *Model) renderBody() string {
	mainWidth := m.width - sidebarWidth - 5
	bodyHeight := m.height - inputHeight - headerHeight - footerHeight - 4

	borderColor := lipgloss.Color("238")
	accentBorder := lipgloss.Color("226")

	// Sidebar
	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(bodyHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentBorder).
		Padding(0, 1)

	// Main chat area
	mainStyle := lipgloss.NewStyle().
		Width(mainWidth).
		Height(bodyHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	sidebar := sidebarStyle.Render(m.renderSidebar())
	main := mainStyle.Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", main)
}

func (m *Model) renderSidebar() string {
	var b strings.Builder
	accent := lipgloss.Color("226")
	dim := lipgloss.Color("240")
	bright := lipgloss.Color("252")

	// Sessions title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)
	if m.focus == focusSessions {
		titleStyle = titleStyle.Underline(true)
	}
	b.WriteString(titleStyle.Render("⚡ Sessions"))

	// Token summary
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(dim).Render(
		fmt.Sprintf("   %d in / %d out", m.tokenUsage.input, m.tokenUsage.output)))

	// Divider
	b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("─", sidebarWidth-4)))

	if m.sessionFilter != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Render("🔍 "+m.sessionFilter))
	}

	visible := m.filteredSessions()
	if len(visible) == 0 {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(" (no sessions)"))
		return b.String()
	}

	limit := min(len(visible), 15)
	for i := 0; i < limit; i++ {
		s := visible[i]
		isCurrent := s.Key == m.currentSession
		isSelected := i == m.selectedSession

		prefix := "  "
		style := lipgloss.NewStyle().Foreground(bright)

		if isSelected && m.focus == focusSessions {
			prefix = "▶ "
			style = lipgloss.NewStyle().Foreground(accent).Bold(true)
		} else if isCurrent {
			prefix = "● "
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
		}

		label := shortSession(s.Key, 20)
		meta := relativeTime(s.UpdatedAt)
		b.WriteString("\n" + style.Render(prefix+label))
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render("    "+meta))
	}

	if len(visible) > limit {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(
			fmt.Sprintf(" +%d more", len(visible)-limit)))
	}

	return b.String()
}

func (m *Model) renderInput() string {
	accent := lipgloss.Color("226")
	dim := lipgloss.Color("240")

	// Input label
	label := "Message"
	labelColor := lipgloss.Color("252")

	if m.pending {
		label = m.spinner.View() + " Thinking..."
		labelColor = accent
		if len(m.messageQueue) > 0 {
			label += fmt.Sprintf(" (%d queued)", len(m.messageQueue))
		}
	}

	labelStr := lipgloss.NewStyle().Bold(true).Foreground(labelColor).Render(label)
	hint := lipgloss.NewStyle().Foreground(dim).Render(" (/ for commands)")

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(m.width - 4)

	content := labelStr + hint + "\n" + m.textarea.View()

	// 显示命令列表
	if m.showCommands {
		content += "\n" + m.renderCommandList()
	}

	return inputBox.Render(content)
}

func (m *Model) renderCommandList() string {
	filtered := m.filteredCommands()
	if len(filtered) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  No matching commands")
	}

	var b strings.Builder
	limit := min(len(filtered), 8)

	for i := 0; i < limit; i++ {
		cmd := filtered[i]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		prefix := "  "

		if i == m.selectedCommand {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
			prefix = "▶ "
		}

		desc := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" - " + cmd.Description)
		b.WriteString(style.Render(prefix+"/"+cmd.Name) + desc + "\n")
	}

	return b.String()
}

func (m *Model) renderFooter() string {
	dim := lipgloss.Color("240")

	// Left: error or RTT
	var left string
	if m.lastError != "" {
		left = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗ " + truncStr(m.lastError, 40))
	} else if m.lastRTT > 0 {
		left = lipgloss.NewStyle().Foreground(dim).Render(fmt.Sprintf("⏱ %s", m.lastRTT.Round(time.Millisecond)))
	}

	// Right: keyboard hints
	hints := []string{
		"Tab:focus",
		"Ctrl+N:new",
		"Ctrl+L:clear",
		"/help",
	}
	right := lipgloss.NewStyle().Foreground(dim).Render(strings.Join(hints, " │ "))

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := m.width - leftWidth - rightWidth - 4
	if gap < 1 {
		gap = 1
	}

	return lipgloss.NewStyle().Padding(0, 1).Render(left + strings.Repeat(" ", gap) + right)
}

func (m *Model) resize() {
	mainWidth := m.width - sidebarWidth - 10
	mainHeight := m.height - inputHeight - headerHeight - footerHeight - 8
	if mainWidth < 20 {
		mainWidth = 20
	}
	if mainHeight < 5 {
		mainHeight = 5
	}
	m.viewport.Width = mainWidth
	m.viewport.Height = mainHeight
	m.textarea.SetWidth(m.width - 10)
}

func (m *Model) appendLine(role, content string) {
	m.lines = append(m.lines, chatLine{
		Role:      role,
		Content:   strings.TrimSpace(content),
		Timestamp: time.Now(),
	})
}

func (m *Model) updateViewport() {
	if m.viewport.Width <= 0 {
		return
	}
	var b strings.Builder
	dim := lipgloss.Color("240")

	for i, line := range m.lines {
		stamp := lipgloss.NewStyle().Foreground(dim).Render(line.Timestamp.Format("15:04"))

		var roleIcon string
		var roleStyle lipgloss.Style

		switch line.Role {
		case "user":
			roleIcon = "▸"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
		case "assistant":
			roleIcon = "◂"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		default:
			roleIcon = "─"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226"))
		}

		header := fmt.Sprintf(" %s %s %s", stamp, roleStyle.Render(roleIcon), roleStyle.Render(strings.ToUpper(line.Role)))
		b.WriteString(header + "\n")

		// Content with wrapping
		content := lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Width(m.viewport.Width - 4).
			Render(line.Content)
		b.WriteString("   " + content)

		if i < len(m.lines)-1 {
			b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("·", min(m.viewport.Width-6, 60))))
		}
		b.WriteString("\n")
	}

	m.viewport.SetContent(strings.TrimRight(b.String(), "\n"))
	m.viewport.GotoBottom()
}

func (m *Model) startNewSession() {
	m.history = nil
	m.lines = nil
	newKey := buildSessionKey(m.opts.Agent, fmt.Sprintf("session-%d", time.Now().Unix()%100000))
	m.currentSession = newKey
	_ = session.SetCurrent(newKey)
	m.sessionFilter = ""
	m.selectedSession = 0
	m.appendLine("system", "New session: "+newKey)
	m.persistCurrentSession()
	m.updateViewport()
}

func (m *Model) moveSessionSelection(delta int) {
	visible := m.filteredSessions()
	if len(visible) == 0 {
		return
	}
	m.selectedSession += delta
	if m.selectedSession < 0 {
		m.selectedSession = 0
	}
	if m.selectedSession >= len(visible) {
		m.selectedSession = len(visible) - 1
	}
}

func (m *Model) moveCommandSelection(delta int) {
	filtered := m.filteredCommands()
	if len(filtered) == 0 {
		return
	}
	m.selectedCommand += delta
	if m.selectedCommand < 0 {
		m.selectedCommand = 0
	}
	if m.selectedCommand >= len(filtered) {
		m.selectedCommand = len(filtered) - 1
	}
}

func (m *Model) filteredCommands() []Command {
	if m.commandFilter == "" {
		return getBuiltinCommands()
	}
	filter := strings.ToLower(m.commandFilter)
	var result []Command
	for _, cmd := range getBuiltinCommands() {
		if strings.HasPrefix(cmd.Name, filter) {
			result = append(result, cmd)
			continue
		}
		for _, alias := range cmd.Aliases {
			if strings.HasPrefix(alias, filter) {
				result = append(result, cmd)
				break
			}
		}
	}
	return result
}

func (m *Model) activateSelectedSession() {
	visible := m.filteredSessions()
	if len(visible) == 0 || m.selectedSession < 0 || m.selectedSession >= len(visible) {
		return
	}
	s := visible[m.selectedSession]
	m.currentSession = s.Key
	_ = session.SetCurrent(s.Key)
	m.loadCurrentSession()
	m.updateViewport()
}

func (m *Model) loadCurrentSession() {
	m.history = nil
	m.lines = nil
	sess, err := session.Load(m.currentSession)
	if err != nil {
		m.appendLine("system", fmt.Sprintf("Session: %s", m.currentSession))
		return
	}
	for _, msg := range sess.Messages() {
		ts := time.Now()
		if msg.Timestamp > 0 {
			ts = time.UnixMilli(msg.Timestamp)
		}
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if role == "" || content == "" {
			continue
		}
		m.lines = append(m.lines, chatLine{Role: role, Content: content, Timestamp: ts})
		if role == "user" || role == "assistant" || role == "system" {
			m.history = append(m.history, agent.ChatMessage{Role: role, Content: content})
		}
	}
	if len(m.lines) == 0 {
		m.appendLine("system", fmt.Sprintf("Session: %s", m.currentSession))
	}
}

func (m *Model) persistCurrentSession() {
	history := make([]protocol.ChatMessage, 0, len(m.history))
	for _, h := range m.history {
		role := strings.TrimSpace(h.Role)
		content := strings.TrimSpace(h.Content)
		if role == "" || content == "" {
			continue
		}
		history = append(history, protocol.ChatMessage{
			Role:      role,
			Content:   content,
			Channel:   "tui",
			Timestamp: time.Now().UnixMilli(),
		})
	}
	_ = session.SaveFromHistory(m.currentSession, "tui", m.opts.Agent, m.cfg.Agent.Model, history)
	m.upsertSessionEntry(sessionEntry{
		Key:       m.currentSession,
		Label:     m.currentSession,
		Channel:   "tui",
		UpdatedAt: time.Now(),
		Model:     m.cfg.Agent.Model,
	})
}

func loadBootCmd(opts Options) tea.Cmd {
	return func() tea.Msg {
		sessions, err := loadSessionIndex(opts.Agent)
		reachable := probeGatewayReachable(opts.GatewayURL)
		return bootMsg{Sessions: sessions, Reachable: reachable, Err: err}
	}
}

func sendMessageCmd(runner *agent.Runner, sessionKey string, history []agent.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		req := &agent.RunRequest{
			SessionKey: sessionKey,
			Channel:    "tui",
			Message:    history[len(history)-1].Content,
			History:    history,
		}
		resp, err := runner.Run(ctx, req)
		if err != nil {
			return assistantMsg{Err: err, Duration: time.Since(start)}
		}
		return assistantMsg{
			Reply:        resp.Reply,
			Duration:     time.Since(start),
			InputTokens:  resp.TokensUsed.InputTokens,
			OutputTokens: resp.TokensUsed.OutputTokens,
		}
	}
}

func loadSessionIndex(agentID string) ([]sessionEntry, error) {
	entries := make([]sessionEntry, 0, 64)
	byKey := map[string]sessionEntry{}

	if local, err := session.LoadAll(); err == nil {
		for _, s := range local {
			e := sessionEntry{
				Key:       s.Key,
				Label:     s.Key,
				Channel:   s.Channel,
				UpdatedAt: s.LastActivityAt,
				Model:     s.Model,
			}
			if old, ok := byKey[e.Key]; ok && old.UpdatedAt.After(e.UpdatedAt) {
				continue
			}
			byKey[e.Key] = e
		}
	}

	for _, v := range byKey {
		entries = append(entries, v)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
	})
	return entries, nil
}

func probeGatewayReachable(gatewayURL string) bool {
	u, err := url.Parse(strings.TrimSpace(gatewayURL))
	if err != nil || u.Host == "" {
		return false
	}
	scheme := "http"
	if strings.EqualFold(u.Scheme, "wss") {
		scheme = "https"
	}
	base := scheme + "://" + u.Host
	client := http.Client{Timeout: 1200 * time.Millisecond}
	paths := []string{"/api/health", "/health"}
	for _, p := range paths {
		resp, err := client.Get(base + p)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return true
			}
		}
	}
	return false
}

func buildSessionKey(agentID, sessionName string) string {
	if strings.HasPrefix(sessionName, "agent:") {
		return sessionName
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = "main"
	}
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		sessionName = "main"
	}
	return fmt.Sprintf("agent:%s:%s", agentID, sessionName)
}

func cloneHistory(in []agent.ChatMessage) []agent.ChatMessage {
	out := make([]agent.ChatMessage, len(in))
	copy(out, in)
	return out
}

func (m *Model) filteredSessions() []sessionEntry {
	if len(m.sessions) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(m.sessionFilter))
	if q == "" {
		out := make([]sessionEntry, len(m.sessions))
		copy(out, m.sessions)
		return out
	}
	out := make([]sessionEntry, 0, len(m.sessions))
	for _, s := range m.sessions {
		if strings.Contains(strings.ToLower(s.Key), q) ||
			strings.Contains(strings.ToLower(s.Label), q) {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) upsertSessionEntry(e sessionEntry) {
	for i := range m.sessions {
		if m.sessions[i].Key == e.Key {
			m.sessions[i] = e
			sort.Slice(m.sessions, func(a, b int) bool {
				return m.sessions[a].UpdatedAt.After(m.sessions[b].UpdatedAt)
			})
			return
		}
	}
	m.sessions = append(m.sessions, e)
	sort.Slice(m.sessions, func(a, b int) bool {
		return m.sessions[a].UpdatedAt.After(m.sessions[b].UpdatedAt)
	})
}

func shortSession(s string, maxLen int) string {
	parts := strings.Split(s, ":")
	if len(parts) > 0 {
		s = parts[len(parts)-1]
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func lastSegment(s string) string {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) == 0 {
		return s
	}
	return parts[len(parts)-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Run starts the TUI with default options
func Run() error {
	return RunWithOptions(Options{})
}

// RunWithOptions starts the TUI with custom options
func RunWithOptions(opts Options) error {
	p := tea.NewProgram(
		NewModel(opts),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	return nil
}
