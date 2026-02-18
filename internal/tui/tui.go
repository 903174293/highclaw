// Package tui implements the terminal user interface.
package tui

import (
	"context"
	"fmt"
	"log/slog"
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

// 页面类型
type pageType int

const (
	pageHome    pageType = iota // 首页（显示 Logo）
	pageSession                 // 会话页面
)

// Options 配置 TUI 启动参数
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
	Key       string
	Label     string
	Channel   string
	UpdatedAt time.Time
	Model     string
}

type tokenUsageInfo struct {
	input  int
	output int
}

type bootMsg struct {
	Sessions []sessionEntry
	Err      error
}

type assistantMsg struct {
	Reply        string
	Err          error
	Duration     time.Duration
	InputTokens  int
	OutputTokens int
}

// Model 表示 TUI 状态
type Model struct {
	opts   Options
	cfg    *config.Config
	runner *agent.Runner

	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	history []agent.ChatMessage
	lines   []chatLine

	sessions       []sessionEntry
	currentSession string

	width  int
	height int
	ready  bool

	page         pageType
	pending      bool
	lastError    string
	lastRTT      time.Duration
	tokenUsage   tokenUsageInfo
	messageQueue []string
	interrupt    int // ESC 连按计数
}

// NewModel 创建新的 TUI Model
func NewModel(opts Options) Model {
	if strings.TrimSpace(opts.Agent) == "" {
		opts.Agent = "main"
	}
	if strings.TrimSpace(opts.Session) == "" {
		opts.Session = "main"
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
	ta.Placeholder = "Say anything... File a TODO in the codebase"
	ta.Focus()
	ta.CharLimit = 10000
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.Prompt = ""

	vp := viewport.New(80, 20)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))

	initialSession := buildSessionKey(opts.Agent, opts.Session)
	if current, err := session.Current(); err == nil && strings.TrimSpace(current) != "" {
		initialSession = strings.TrimSpace(current)
	}

	return Model{
		opts:           opts,
		cfg:            cfg,
		runner:         agent.NewRunner(cfg, logger),
		textarea:       ta,
		viewport:       vp,
		spinner:        sp,
		currentSession: initialSession,
		page:           pageHome,
		messageQueue:   make([]string, 0),
	}
}

// Init 初始化 TUI
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, loadBootCmd(m.opts))
}

// Update 处理消息和更新状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resize()
		return m, nil

	case bootMsg:
		if msg.Err != nil {
			m.lastError = msg.Err.Error()
		}
		if len(msg.Sessions) > 0 {
			m.sessions = msg.Sessions
		}
		m.loadCurrentSession()
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

		// 处理队列中的下一条消息
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
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEsc:
			if m.pending {
				m.interrupt++
				if m.interrupt >= 2 {
					// TODO: 实现中断逻辑
					m.pending = false
					m.interrupt = 0
					m.appendLine("system", "Interrupted.")
					m.updateViewport()
				}
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyTab:
			// TODO: 实现 agent 切换
			return m, nil

		case tea.KeyCtrlP:
			// TODO: 实现命令面板
			return m, nil

		case tea.KeyEnter:
			text := strings.TrimSpace(m.textarea.Value())
			if text == "" {
				return m, nil
			}
			m.textarea.Reset()
			m.textarea.SetHeight(1)
			m.lastError = ""
			m.interrupt = 0

			// 切换到会话页面
			if m.page == pageHome {
				m.page = pageSession
			}

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
				m.appendLine("system", "Unknown command: "+cmdName)
				m.updateViewport()
				return m, nil
			}

			// 发送消息
			if m.pending {
				m.messageQueue = append(m.messageQueue, text)
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
	}

	// 更新 textarea
	var tiCmd tea.Cmd
	m.textarea, tiCmd = m.textarea.Update(msg)
	cmds = append(cmds, tiCmd)

	// 更新 viewport
	if m.page == pageSession {
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

// View 渲染 TUI
func (m Model) View() string {
	if !m.ready {
		return "\n  Loading..."
	}

	switch m.page {
	case pageHome:
		return m.renderHomePage()
	case pageSession:
		return m.renderSessionPage()
	default:
		return m.renderHomePage()
	}
}

// renderHomePage 渲染首页（显示 Logo）
func (m *Model) renderHomePage() string {
	theme := getTheme()
	var content strings.Builder

	// 上部空白
	topPadding := (m.height - 15) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	content.WriteString(strings.Repeat("\n", topPadding))

	// Logo
	content.WriteString(renderLogo(m.width))
	content.WriteString("\n")

	// 输入框
	inputWidth := min(75, m.width-4)
	inputStyle := lipgloss.NewStyle().
		Width(inputWidth).
		Border(lipgloss.NormalBorder(), false, false, true, true).
		BorderForeground(theme.primary).
		PaddingLeft(1)

	inputBox := inputStyle.Render(m.textarea.View())
	// 居中
	padding := (m.width - lipgloss.Width(inputBox)) / 2
	if padding > 0 {
		inputBox = lipgloss.NewStyle().PaddingLeft(padding).Render(inputBox)
	}
	content.WriteString(inputBox)
	content.WriteString("\n")

	// 下方提示
	hints := lipgloss.NewStyle().Foreground(theme.textMuted).Render(
		"tab agents  ctrl+p commands")
	hintPadding := (m.width - lipgloss.Width(hints)) / 2
	if hintPadding > 0 {
		hints = lipgloss.NewStyle().PaddingLeft(hintPadding).Render(hints)
	}
	content.WriteString("\n")
	content.WriteString(hints)

	// Tips
	tips := lipgloss.NewStyle().Foreground(theme.textMuted).Render(
		"• Tip Run /compact to summarize long sessions near context limits")
	tipPadding := (m.width - lipgloss.Width(tips)) / 2
	if tipPadding > 0 {
		tips = lipgloss.NewStyle().PaddingLeft(tipPadding).Render(tips)
	}
	content.WriteString("\n\n\n")
	content.WriteString(tips)

	// 填充剩余空间
	currentHeight := strings.Count(content.String(), "\n") + 1
	remaining := m.height - currentHeight - 3
	if remaining > 0 {
		content.WriteString(strings.Repeat("\n", remaining))
	}

	// 底部状态栏
	content.WriteString(m.renderFooter())

	return content.String()
}

// renderSessionPage 渲染会话页面
func (m *Model) renderSessionPage() string {
	theme := getTheme()
	var content strings.Builder

	// Header
	content.WriteString(m.renderSessionHeader())
	content.WriteString("\n")

	// 聊天内容区域
	chatHeight := m.height - 6 // header + input + footer
	if chatHeight < 5 {
		chatHeight = 5
	}
	m.viewport.Height = chatHeight
	m.viewport.Width = m.width - 2
	content.WriteString(lipgloss.NewStyle().PaddingLeft(1).Render(m.viewport.View()))
	content.WriteString("\n")

	// 输入框
	inputStyle := lipgloss.NewStyle().
		Width(m.width-4).
		Border(lipgloss.NormalBorder(), false, false, true, true).
		BorderForeground(theme.primary).
		PaddingLeft(1)

	// 显示 spinner 或输入框
	var inputContent string
	if m.pending {
		inputContent = m.spinner.View() + " Thinking..."
		if len(m.messageQueue) > 0 {
			inputContent += fmt.Sprintf(" (%d queued)", len(m.messageQueue))
		}
	} else {
		inputContent = m.textarea.View()
	}
	content.WriteString(lipgloss.NewStyle().PaddingLeft(1).Render(inputStyle.Render(inputContent)))
	content.WriteString("\n")

	// 底部状态栏
	content.WriteString(m.renderSessionFooter())

	return content.String()
}

// renderSessionHeader 渲染会话页面的 header
func (m *Model) renderSessionHeader() string {
	theme := getTheme()

	// 左侧：会话标题
	title := "# " + lastSegment(m.currentSession)
	left := lipgloss.NewStyle().Foreground(theme.text).Render(title)

	// 右侧：版本和状态
	version := "- HighClaw 0.1.0"
	tagline := "High performance Go runtime."
	right := lipgloss.NewStyle().Foreground(theme.textMuted).Render(version) +
		"\n" + lipgloss.NewStyle().Foreground(theme.textMuted).Render(tagline)

	// 计算间距
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(version)
	gap := m.width - leftWidth - rightWidth - 4
	if gap < 1 {
		gap = 1
	}

	header := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Render(
		left + strings.Repeat(" ", gap) + right)

	return header
}

// renderFooter 渲染首页底部状态栏
func (m *Model) renderFooter() string {
	theme := getTheme()

	// 左侧：目录
	pwd, _ := os.Getwd()
	left := lipgloss.NewStyle().Foreground(theme.textMuted).Render(pwd)

	// 右侧：版本
	right := lipgloss.NewStyle().Foreground(theme.textMuted).Render("v0.1.0")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}

	return lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Render(
		left + strings.Repeat(" ", gap) + right)
}

// renderSessionFooter 渲染会话页面底部状态栏
func (m *Model) renderSessionFooter() string {
	theme := getTheme()

	// 左侧：状态
	var leftParts []string
	if m.pending {
		leftParts = append(leftParts,
			lipgloss.NewStyle().
				Background(theme.primary).
				Foreground(lipgloss.Color("#000000")).
				Padding(0, 1).
				Render("ACTIVE"))
		leftParts = append(leftParts,
			lipgloss.NewStyle().Foreground(theme.textMuted).Render("esc interrupt"))
	}
	left := strings.Join(leftParts, " ")

	// 右侧：快捷键提示
	hints := []string{
		lipgloss.NewStyle().Foreground(theme.textMuted).Render("tab agents"),
		lipgloss.NewStyle().Foreground(theme.textMuted).Render("ctrl+p commands"),
	}
	right := strings.Join(hints, "  ")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}

	return lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Render(
		left + strings.Repeat(" ", gap) + right)
}

func (m *Model) resize() {
	m.viewport.Width = m.width - 2
	m.viewport.Height = m.height - 8
	m.textarea.SetWidth(m.width - 6)
}

func (m *Model) appendLine(role, content string) {
	m.lines = append(m.lines, chatLine{
		Role:      role,
		Content:   strings.TrimSpace(content),
		Timestamp: time.Now(),
	})
}

func (m *Model) updateViewport() {
	theme := getTheme()
	var b strings.Builder

	for _, line := range m.lines {
		switch line.Role {
		case "user":
			// 用户消息：带竖线边框
			border := lipgloss.NewStyle().Foreground(theme.primary).Render("│ ")
			content := lipgloss.NewStyle().Foreground(theme.text).Render(line.Content)
			b.WriteString(border + content + "\n\n")

		case "assistant":
			// Assistant 消息
			icon := lipgloss.NewStyle().Foreground(theme.textMuted).Render("▶ ")
			label := lipgloss.NewStyle().Foreground(theme.textMuted).Render("Sisyphus (Ultraworker)")
			model := lipgloss.NewStyle().Foreground(theme.textMuted).Render(" - " + m.cfg.Agent.Model)
			b.WriteString(icon + label + model + "\n")
			content := lipgloss.NewStyle().Foreground(theme.text).Width(m.viewport.Width - 4).Render(line.Content)
			b.WriteString(content + "\n\n")

		case "system":
			// 系统消息
			content := lipgloss.NewStyle().Foreground(theme.textMuted).Italic(true).Render(line.Content)
			b.WriteString(content + "\n\n")
		}
	}

	m.viewport.SetContent(strings.TrimRight(b.String(), "\n"))
	m.viewport.GotoBottom()
}

func (m *Model) loadCurrentSession() {
	m.history = nil
	m.lines = nil
	sess, err := session.Load(m.currentSession)
	if err != nil {
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
	// 如果有历史消息，直接进入会话页面
	if len(m.lines) > 0 {
		m.page = pageSession
		m.updateViewport()
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
}

func (m *Model) startNewSession() {
	m.history = nil
	m.lines = nil
	newKey := buildSessionKey(m.opts.Agent, fmt.Sprintf("session-%d", time.Now().Unix()%100000))
	m.currentSession = newKey
	_ = session.SetCurrent(newKey)
	m.page = pageHome
}

func loadBootCmd(opts Options) tea.Cmd {
	return func() tea.Msg {
		sessions, err := loadSessionIndex(opts.Agent)
		return bootMsg{Sessions: sessions, Err: err}
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

// Run 使用默认选项启动 TUI
func Run() error {
	return RunWithOptions(Options{})
}

// RunWithOptions 使用自定义选项启动 TUI
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
