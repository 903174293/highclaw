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
	pageHome    pageType = iota // 首页
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
	interrupt    int
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
	ta.Placeholder = "Say anything... Fix a TODO in the codebase"
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

// Update 处理消息
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

		// 处理队列
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
					m.pending = false
					m.interrupt = 0
					m.appendLine("system", "Interrupted.")
					m.updateViewport()
				}
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyTab:
			// TODO: agent 切换
			return m, nil

		case tea.KeyCtrlP:
			// TODO: 命令面板
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

			if m.page == pageHome {
				m.page = pageSession
			}

			// 命令处理
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

	// 更新组件
	var tiCmd tea.Cmd
	m.textarea, tiCmd = m.textarea.Update(msg)
	cmds = append(cmds, tiCmd)

	if m.page == pageSession {
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

// View 渲染界面
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

// renderHomePage 渲染首页
func (m *Model) renderHomePage() string {
	theme := getTheme()
	var b strings.Builder

	// 上半部分空白
	topPadding := (m.height - 12) / 2
	if topPadding < 2 {
		topPadding = 2
	}
	b.WriteString(strings.Repeat("\n", topPadding))

	// Logo
	b.WriteString(renderLogo(m.width))
	b.WriteString("\n")

	// 输入框区域
	inputWidth := min(75, m.width-4)
	padding := (m.width - inputWidth) / 2
	if padding < 0 {
		padding = 0
	}

	// 输入框：左边竖线 + 内容
	leftBorder := lipgloss.NewStyle().Foreground(theme.primary).Render("┃ ")
	inputContent := m.textarea.View()
	inputLine := leftBorder + inputContent

	// 底部竖线结束符
	bottomBorder := lipgloss.NewStyle().Foreground(theme.primary).Render("╹")

	b.WriteString(strings.Repeat(" ", padding) + inputLine + "\n")
	b.WriteString(strings.Repeat(" ", padding) + bottomBorder + "\n")

	// Agent/Model 信息
	agentInfo := lipgloss.NewStyle().Foreground(theme.textMuted).Render(
		fmt.Sprintf("Sisyphus (Ultraworker)  %s", m.cfg.Agent.Model))
	b.WriteString(strings.Repeat(" ", padding+2) + agentInfo + "\n")

	// 快捷键提示
	hints := lipgloss.NewStyle().Foreground(theme.textMuted).Render(
		"tab agents  ctrl+p commands")
	b.WriteString(strings.Repeat(" ", padding+2) + hints + "\n")

	// Tips
	b.WriteString("\n")
	tips := lipgloss.NewStyle().Foreground(theme.textMuted).Render(
		"• Tip Run /compact to summarize long sessions near context limits")
	tipPadding := (m.width - lipgloss.Width(tips)) / 2
	if tipPadding < 0 {
		tipPadding = 0
	}
	b.WriteString(strings.Repeat(" ", tipPadding) + tips)

	// 填充到底部
	currentLines := strings.Count(b.String(), "\n") + 1
	remaining := m.height - currentLines - 2
	if remaining > 0 {
		b.WriteString(strings.Repeat("\n", remaining))
	}

	// Footer
	b.WriteString("\n" + m.renderFooter())

	return b.String()
}

// renderSessionPage 渲染会话页面
func (m *Model) renderSessionPage() string {
	theme := getTheme()
	var b strings.Builder

	// Header
	b.WriteString(m.renderSessionHeader())
	b.WriteString("\n")

	// 聊天区域
	chatHeight := m.height - 7
	if chatHeight < 5 {
		chatHeight = 5
	}
	m.viewport.Height = chatHeight
	m.viewport.Width = m.width - 4
	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(m.viewport.View()))
	b.WriteString("\n")

	// 输入框
	leftBorder := lipgloss.NewStyle().Foreground(theme.primary).Render("┃ ")
	var inputContent string
	if m.pending {
		inputContent = m.spinner.View() + " "
		if len(m.messageQueue) > 0 {
			inputContent += fmt.Sprintf("(%d queued) ", len(m.messageQueue))
		}
	} else {
		inputContent = m.textarea.View()
	}
	b.WriteString("  " + leftBorder + inputContent + "\n")
	bottomBorder := lipgloss.NewStyle().Foreground(theme.primary).Render("╹")
	b.WriteString("  " + bottomBorder + "\n")

	// Footer
	b.WriteString(m.renderSessionFooter())

	return b.String()
}

// renderSessionHeader 渲染 session header
func (m *Model) renderSessionHeader() string {
	theme := getTheme()

	// 左侧：标题
	title := lipgloss.NewStyle().Bold(true).Foreground(theme.text).Render(
		"# " + lastSegment(m.currentSession))

	// 右侧：token 信息
	tokenInfo := ""
	if m.tokenUsage.input+m.tokenUsage.output > 0 {
		tokenInfo = fmt.Sprintf("%d tokens", m.tokenUsage.input+m.tokenUsage.output)
	}
	right := lipgloss.NewStyle().Foreground(theme.textMuted).Render(tokenInfo)

	// 计算间距
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}

	// Header 带左边框
	content := title + strings.Repeat(" ", gap) + right
	leftBorder := lipgloss.NewStyle().Foreground(theme.border).Render("┃")

	return "  " + leftBorder + " " + content
}

// renderFooter 渲染首页 footer
func (m *Model) renderFooter() string {
	theme := getTheme()

	pwd, _ := os.Getwd()
	left := lipgloss.NewStyle().Foreground(theme.textMuted).Render(pwd)
	right := lipgloss.NewStyle().Foreground(theme.textMuted).Render("v0.1.0")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}

	return "  " + left + strings.Repeat(" ", gap) + right
}

// renderSessionFooter 渲染 session footer
func (m *Model) renderSessionFooter() string {
	theme := getTheme()

	// 左侧
	var leftParts []string
	if m.pending {
		active := lipgloss.NewStyle().
			Background(theme.primary).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Render("ACTIVE")
		leftParts = append(leftParts, active)

		escHint := "esc "
		if m.interrupt > 0 {
			escHint += lipgloss.NewStyle().Foreground(theme.primary).Render("again to interrupt")
		} else {
			escHint += lipgloss.NewStyle().Foreground(theme.textMuted).Render("interrupt")
		}
		leftParts = append(leftParts, escHint)
	}
	left := strings.Join(leftParts, " ")

	// 右侧
	hints := lipgloss.NewStyle().Foreground(theme.textMuted).Render("tab agents  ctrl+p commands")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(hints) - 4
	if gap < 1 {
		gap = 1
	}

	return "  " + left + strings.Repeat(" ", gap) + hints
}

func (m *Model) resize() {
	m.viewport.Width = m.width - 4
	m.viewport.Height = m.height - 8
	m.textarea.SetWidth(min(70, m.width-10))
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

	for i, line := range m.lines {
		switch line.Role {
		case "user":
			// 用户消息：绿色左边框
			border := lipgloss.NewStyle().Foreground(theme.primary).Render("┃")
			content := lipgloss.NewStyle().Foreground(theme.text).Render(line.Content)
			b.WriteString(border + " " + content)
			if i < len(m.lines)-1 {
				b.WriteString("\n\n")
			}

		case "assistant":
			// Assistant 标签
			label := lipgloss.NewStyle().Foreground(theme.textMuted).Render(
				"▶ Sisyphus (Ultraworker) - " + m.cfg.Agent.Model)
			b.WriteString(label + "\n")
			content := lipgloss.NewStyle().Foreground(theme.text).Width(m.viewport.Width - 2).Render(line.Content)
			b.WriteString(content)
			if i < len(m.lines)-1 {
				b.WriteString("\n\n")
			}

		case "system":
			content := lipgloss.NewStyle().Foreground(theme.textMuted).Italic(true).Render(line.Content)
			b.WriteString(content)
			if i < len(m.lines)-1 {
				b.WriteString("\n\n")
			}
		}
	}

	m.viewport.SetContent(b.String())
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

// Run 启动 TUI
func Run() error {
	return RunWithOptions(Options{})
}

// RunWithOptions 使用选项启动 TUI
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
