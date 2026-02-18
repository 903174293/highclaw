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
	sidebarMinWidth = 34
	inputHeight     = 4
)

type focusTarget int

const (
	focusInput focusTarget = iota
	focusSessions
)

// Options configures TUI startup.
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

type bootMsg struct {
	Sessions  []sessionEntry
	Reachable bool
	Err       error
}

type assistantMsg struct {
	Reply    string
	Err      error
	Duration time.Duration
}

// Model represents the TUI state.
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

	reachable bool
	pending   bool
	lastError string
	lastRTT   time.Duration
}

// NewModel creates a new TUI model.
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
	ta.Placeholder = "Message... (Enter send, Tab switch focus, Ctrl+N new session)"
	ta.Focus()
	ta.CharLimit = 10000
	ta.SetHeight(inputHeight)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))

	initialSession := buildSessionKey(opts.Agent, opts.Session)
	if strings.TrimSpace(opts.Session) == "" || strings.EqualFold(strings.TrimSpace(opts.Session), "main") {
		if current, err := session.Current(); err == nil && strings.TrimSpace(current) != "" {
			initialSession = strings.TrimSpace(current)
		}
	}
	model := Model{
		opts:            opts,
		cfg:             cfg,
		runner:          agent.NewRunner(cfg, logger),
		textarea:        ta,
		viewport:        vp,
		spinner:         sp,
		currentSession:  initialSession,
		selectedSession: 0,
		focus:           focusInput,
		lines: []chatLine{
			{
				Role:      "system",
				Content:   "Welcome to HighClaw TUI. Press Enter to chat, Ctrl+N for a fresh session.",
				Timestamp: time.Now(),
			},
		},
	}
	model.updateViewport()
	return model
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, loadBootCmd(m.opts))
}

// Update handles messages and updates the model.
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
		m.updateViewport()
		return m, nil

	case assistantMsg:
		m.pending = false
		m.lastRTT = msg.Duration
		if msg.Err != nil {
			m.lastError = msg.Err.Error()
			m.appendLine("system", "Request failed: "+msg.Err.Error())
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
		return m, nil

	case spinner.TickMsg:
		if m.pending {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
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
		case tea.KeyBackspace:
			if m.focus == focusSessions && len(m.sessionFilter) > 0 {
				m.sessionFilter = m.sessionFilter[:len(m.sessionFilter)-1]
				m.selectedSession = 0
				return m, nil
			}
		case tea.KeyCtrlR:
			return m, loadBootCmd(m.opts)
		case tea.KeyUp:
			if m.focus == focusSessions {
				m.moveSessionSelection(-1)
				return m, nil
			}
		case tea.KeyDown:
			if m.focus == focusSessions {
				m.moveSessionSelection(1)
				return m, nil
			}
		case tea.KeyEnter:
			if m.pending {
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
			m.pending = true
			m.appendLine("user", text)
			m.history = append(m.history, agent.ChatMessage{Role: "user", Content: text})
			m.persistCurrentSession()
			m.updateViewport()
			cmds = append(cmds, sendMessageCmd(m.runner, m.currentSession, cloneHistory(m.history)))
			cmds = append(cmds, m.spinner.Tick)
			return m, tea.Batch(cmds...)
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

	// Update focused component.
	if m.focus == focusInput {
		var tiCmd tea.Cmd
		m.textarea, tiCmd = m.textarea.Update(msg)
		cmds = append(cmds, tiCmd)
	}
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "\n  Booting HighClaw TUI..."
	}

	header := m.renderHeader()
	body := m.renderBody()
	input := m.renderInput()
	status := m.renderStatus()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, input, status)
}

func (m *Model) renderHeader() string {
	accent := lipgloss.Color("45")
	dim := lipgloss.Color("240")
	bright := lipgloss.Color("252")

	logo := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("⚡ HIGHCLAW")
	ver := lipgloss.NewStyle().Foreground(dim).Render(" v0.1 ")
	pipe := lipgloss.NewStyle().Foreground(dim).Render("│")

	sessionLabel := lastSegment(m.currentSession)
	if len(sessionLabel) > 20 {
		sessionLabel = sessionLabel[:19] + "…"
	}
	info := lipgloss.NewStyle().Foreground(bright).Render(
		fmt.Sprintf(" %s %s session:%s %s model:%s",
			pipe, m.opts.Agent, sessionLabel, pipe, m.cfg.Agent.Model),
	)

	netIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("●")
	if m.reachable {
		netIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("●")
	}

	head := lipgloss.JoinHorizontal(lipgloss.Center, logo, ver, info, " ", netIcon)
	bar := lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("━", max(10, m.width-2)))
	return lipgloss.NewStyle().Padding(0, 1).Width(m.width).
		Render(lipgloss.JoinVertical(lipgloss.Left, head, bar))
}

func (m *Model) renderBody() string {
	sidebarWidth := max(sidebarMinWidth, m.width/4)
	mainWidth := max(20, m.width-sidebarWidth-4)
	bodyHeight := max(8, m.height-inputHeight-6)

	borderColor := lipgloss.Color("238")
	accentBorder := lipgloss.Color("62")

	sidebarBorder := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "┌", TopRight: "┬", BottomLeft: "└", BottomRight: "┴",
	}
	mainBorder := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "┬", TopRight: "┐", BottomLeft: "┴", BottomRight: "┘",
	}

	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(bodyHeight).
		Border(sidebarBorder).
		BorderForeground(accentBorder).
		Padding(0, 1)

	mainStyle := lipgloss.NewStyle().
		Width(mainWidth).
		Height(bodyHeight).
		Border(mainBorder).
		BorderForeground(borderColor).
		Padding(0, 1)

	sidebar := sidebarStyle.Render(m.renderSessions())
	main := mainStyle.Render(m.viewport.View())
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

func (m *Model) renderSessions() string {
	var b strings.Builder
	accent := lipgloss.Color("45")
	dim := lipgloss.Color("240")
	bright := lipgloss.Color("252")

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)
	if m.focus == focusSessions {
		titleStyle = titleStyle.Background(lipgloss.Color("236"))
	}
	b.WriteString(titleStyle.Render(" ⚡ Sessions "))

	filterText := strings.TrimSpace(m.sessionFilter)
	if filterText != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Render(" 🔍 "+filterText))
	}
	b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("─", sidebarMinWidth-4)))

	visible := m.filteredSessions()
	if len(visible) == 0 {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(" (empty)"))
		return b.String()
	}
	limit := len(visible)
	if limit > 20 {
		limit = 20
	}
	shown := visible[:limit]
	groups := []struct {
		title string
		icon  string
		id    string
	}{
		{title: "CLI", icon: "▸", id: "cli"},
		{title: "TUI", icon: "▸", id: "tui"},
		{title: "OTHER", icon: "▸", id: "other"},
	}
	for _, g := range groups {
		groupHas := false
		for i, s := range shown {
			if sessionGroupID(s) != g.id {
				continue
			}
			if !groupHas {
				groupLabel := lipgloss.NewStyle().Foreground(dim).Bold(true).Render(g.icon + " " + g.title)
				b.WriteString("\n" + groupLabel)
				groupHas = true
			}
			isCurrent := s.Key == m.currentSession
			isSelected := i == m.selectedSession

			prefix := "  "
			style := lipgloss.NewStyle().Foreground(bright)
			if isSelected && m.focus == focusSessions {
				prefix = "▶ "
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true).Background(lipgloss.Color("236"))
			} else if isCurrent {
				prefix = "● "
				style = lipgloss.NewStyle().Foreground(accent)
			}
			label := s.Label
			if label == "" {
				label = s.Key
			}
			meta := fmt.Sprintf("%s · %s", shortSession(label, 20), relativeTime(s.UpdatedAt))
			b.WriteString("\n" + style.Render(prefix+meta))
		}
	}
	if len(visible) > limit {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(fmt.Sprintf(" +%d more", len(visible)-limit)))
	}
	return b.String()
}

func (m *Model) renderInput() string {
	label := " ✏ Input"
	labelColor := lipgloss.Color("111")
	if m.pending {
		label = " ⏳ " + m.spinner.View() + " Thinking..."
		labelColor = lipgloss.Color("214")
	}
	inputBorder := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "├", TopRight: "┤", BottomLeft: "└", BottomRight: "┘",
	}
	box := lipgloss.NewStyle().
		Border(inputBorder).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(max(20, m.width-2))
	return box.Render(lipgloss.NewStyle().Bold(true).Foreground(labelColor).Render(label) + "\n" + m.textarea.View())
}

func (m *Model) renderStatus() string {
	dim := lipgloss.Color("240")
	bright := lipgloss.Color("250")

	netIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗")
	netLabel := "offline"
	if m.reachable {
		netIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✓")
		netLabel = "online"
	}

	errText := ""
	if m.lastError != "" {
		errText = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(" │ " + m.lastError)
	}
	rtt := ""
	if m.lastRTT > 0 {
		rtt = lipgloss.NewStyle().Foreground(dim).Render(fmt.Sprintf(" │ %s", m.lastRTT.Round(time.Millisecond)))
	}

	line1 := lipgloss.NewStyle().Foreground(bright).Render(
		fmt.Sprintf(" %s %s │ %s │ %s", netIcon, netLabel, m.currentSession, m.cfg.Agent.Model),
	)
	kb := lipgloss.NewStyle().Foreground(dim).Render(
		" Tab:focus  ↑↓:nav  Enter:send  Ctrl+N:new  Ctrl+R:reload  Ctrl+C:quit",
	)
	return lipgloss.NewStyle().Padding(0, 0).
		Render(line1 + rtt + errText + "\n" + kb)
}

func (m *Model) resize() {
	sidebarWidth := max(sidebarMinWidth, m.width/4)
	mainWidth := max(20, m.width-sidebarWidth-7)
	mainHeight := max(8, m.height-inputHeight-10)
	m.viewport.Width = mainWidth
	m.viewport.Height = mainHeight
	m.textarea.SetWidth(max(20, m.width-8))
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
		roleIcon := "│"
		roleStyle := lipgloss.NewStyle().Bold(true)
		switch line.Role {
		case "user":
			roleIcon = "▸"
			roleStyle = roleStyle.Foreground(lipgloss.Color("81"))
		case "assistant":
			roleIcon = "◂"
			roleStyle = roleStyle.Foreground(lipgloss.Color("205"))
		default:
			roleIcon = "─"
			roleStyle = roleStyle.Foreground(lipgloss.Color("214"))
		}
		header := fmt.Sprintf(" %s %s %s", stamp, roleStyle.Render(roleIcon+" "+strings.ToUpper(line.Role)), "")
		b.WriteString(header)
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 3).Render(line.Content))
		if i < len(m.lines)-1 {
			b.WriteString("\n" + lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("·", max(10, m.viewport.Width-6))))
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
	m.appendLine("system", "Started new session: "+newKey)
	m.persistCurrentSession()
	m.updateViewport()
}

func (m *Model) moveSessionSelection(delta int) {
	visible := m.filteredSessions()
	if len(visible) == 0 {
		m.selectedSession = 0
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
		m.appendLine("system", fmt.Sprintf("Switched session: %s", m.currentSession))
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
		m.lines = append(m.lines, chatLine{
			Role:      role,
			Content:   content,
			Timestamp: ts,
		})
		if role == "user" || role == "assistant" || role == "system" {
			m.history = append(m.history, agent.ChatMessage{Role: role, Content: content})
		}
	}
	if len(m.lines) == 0 {
		m.appendLine("system", fmt.Sprintf("Switched session: %s", m.currentSession))
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
		return assistantMsg{Reply: resp.Reply, Duration: time.Since(start)}
	}
}

func loadSessionIndex(agentID string) ([]sessionEntry, error) {
	entries := make([]sessionEntry, 0, 64)
	byKey := map[string]sessionEntry{}

	// Merge HighClaw session files.
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
			strings.Contains(strings.ToLower(s.Label), q) ||
			strings.Contains(strings.ToLower(s.Channel), q) {
			out = append(out, s)
		}
	}
	return out
}

func sessionGroupID(s sessionEntry) string {
	ch := strings.ToLower(strings.TrimSpace(s.Channel))
	key := strings.ToLower(s.Key)
	if ch == "cli" || strings.Contains(key, ":cli-") {
		return "cli"
	}
	if ch == "tui" {
		return "tui"
	}
	return "other"
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
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:1]
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Run starts the TUI with default options.
func Run() error {
	return RunWithOptions(Options{})
}

// RunWithOptions starts the TUI with custom options.
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
