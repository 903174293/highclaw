// Package tui implements the terminal user interface.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme 定义 TUI 的颜色主题
type Theme struct {
	background   lipgloss.Color
	text         lipgloss.Color
	textMuted    lipgloss.Color
	primary      lipgloss.Color
	success      lipgloss.Color
	warning      lipgloss.Color
	error        lipgloss.Color
	border       lipgloss.Color
	borderActive lipgloss.Color
}

// 默认主题（暗色，类似 OpenCode）
func getTheme() Theme {
	return Theme{
		background:   lipgloss.Color("#1a1a1a"),
		text:         lipgloss.Color("#e0e0e0"),
		textMuted:    lipgloss.Color("#666666"),
		primary:      lipgloss.Color("#22c55e"), // 绿色
		success:      lipgloss.Color("#22c55e"),
		warning:      lipgloss.Color("#eab308"),
		error:        lipgloss.Color("#ef4444"),
		border:       lipgloss.Color("#333333"),
		borderActive: lipgloss.Color("#22c55e"),
	}
}
