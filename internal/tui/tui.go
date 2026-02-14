// Package tui implements the terminal user interface.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the TUI state.
type Model struct {
	viewport viewport.Model
	textarea textarea.Model
	messages []string
	width    int
	height   int
	ready    bool
}

// NewModel creates a new TUI model.
func NewModel() Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (Ctrl+C to quit)"
	ta.Focus()
	ta.CharLimit = 10000
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to HighClaw TUI!\n\nConnecting to gateway...")

	return Model{
		textarea: ta,
		viewport: vp,
		messages: []string{
			"Welcome to HighClaw TUI!",
			"",
			"Connecting to gateway...",
		},
		ready: false,
	}
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			// Send message
			text := m.textarea.Value()
			if text != "" {
				m.messages = append(m.messages, fmt.Sprintf("You: %s", text))
				m.messages = append(m.messages, "Assistant: [Response placeholder]")
				m.textarea.Reset()
				m.updateViewport()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6)
			m.viewport.YPosition = 0
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}

		m.textarea.SetWidth(msg.Width)
		m.updateViewport()
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Padding(0, 1)

	header := headerStyle.Render("🦀 HighClaw TUI")

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1)

	footer := footerStyle.Render("Ctrl+C to quit • Enter to send")

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\n%s",
		header,
		m.viewport.View(),
		m.textarea.View(),
		footer,
	)
}

// updateViewport updates the viewport content with current messages.
func (m *Model) updateViewport() {
	content := strings.Join(m.messages, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

// Run starts the TUI.
func Run() error {
	p := tea.NewProgram(
		NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	return nil
}

