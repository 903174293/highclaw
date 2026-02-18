// Package tui implements the terminal user interface.
package tui

import "github.com/charmbracelet/lipgloss"

// HighClaw Logo 像素字（仿 OpenCode 风格）
// 使用 _ ^ ~ 作为阴影标记
var logoLeft = []string{
	"                          ",
	"█▀▀█ ▀ █▀▀▀ █▀▀█ █▀▀▀ █   ",
	"█__█ █ █___ █__█ █___ █__ ",
	"▀▀▀▀ ▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀ ",
}

var logoRight = []string{
	"                    ",
	" █▀▀█ █   █▀▀█ █   █",
	" █___ █__ █__█ █ █ █",
	" ▀▀▀▀ ▀▀▀ ▀▀▀▀ ▀▀▀▀▀",
}

// 渲染 Logo
func renderLogo(width int) string {
	theme := getTheme()
	var result string

	for i := range logoLeft {
		left := lipgloss.NewStyle().Foreground(theme.textMuted).Render(logoLeft[i])
		right := lipgloss.NewStyle().Foreground(theme.text).Bold(true).Render(logoRight[i])
		line := left + " " + right
		// 居中
		padding := (width - lipgloss.Width(line)) / 2
		if padding > 0 {
			line = lipgloss.NewStyle().PaddingLeft(padding).Render(line)
		}
		result += line + "\n"
	}
	return result
}

// 小型 Logo（用于 header）
func renderMiniLogo() string {
	theme := getTheme()
	return lipgloss.NewStyle().Foreground(theme.textMuted).Render("high") +
		lipgloss.NewStyle().Foreground(theme.text).Bold(true).Render("claw")
}
