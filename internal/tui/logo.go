// Package tui 提供 logo 渲染
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HighClaw Logo 像素字（仿 OpenCode 风格）
var logoLines = []string{
	"⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡",
	"██╗  ██╗██╗ ██████╗ ██╗  ██╗ ██████╗██╗      █████╗ ██╗    ██╗",
	"██║  ██║██║██╔════╝ ██║  ██║██╔════╝██║     ██╔══██╗██║    ██║",
	"███████║██║██║  ███╗███████║██║     ██║     ███████║██║ █╗ ██║",
	"██╔══██║██║██║   ██║██╔══██║██║     ██║     ██╔══██║██║███╗██║",
	"██║  ██║██║╚██████╔╝██║  ██║╚██████╗███████╗██║  ██║╚███╔███╔╝",
	"╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝",
	"⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡",
}

var tagline = "High performance. Built for speed and reliability. 100% Go 100% Agnostic."

// renderLogo 渲染 Logo
func renderLogo(termWidth int) string {
	theme := getTheme()

	// 计算 logo 宽度
	maxWidth := 0
	for _, line := range logoLines {
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
	}

	var b strings.Builder

	// Logo 行，居中
	for _, line := range logoLines {
		styledLine := lipgloss.NewStyle().Foreground(theme.primary).Bold(true).Render(line)
		padding := (termWidth - lipgloss.Width(line)) / 2
		if padding < 0 {
			padding = 0
		}
		b.WriteString(strings.Repeat(" ", padding) + styledLine + "\n")
	}

	// Tagline
	styledTagline := lipgloss.NewStyle().Foreground(theme.textMuted).Italic(true).Render(tagline)
	taglinePadding := (termWidth - lipgloss.Width(tagline)) / 2
	if taglinePadding < 0 {
		taglinePadding = 0
	}
	b.WriteString(strings.Repeat(" ", taglinePadding) + styledTagline)

	return b.String()
}

// renderMiniLogo 渲染简化 logo
func renderMiniLogo() string {
	theme := getTheme()
	return lipgloss.NewStyle().Foreground(theme.primary).Bold(true).Render("⚡ HighClaw")
}
