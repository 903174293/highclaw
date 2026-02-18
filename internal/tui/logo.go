// Package tui implements the terminal user interface.
package tui

import "github.com/charmbracelet/lipgloss"

// 闪电边框
const lightningBorder = "⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡"

// HighClaw 主 Logo
const mainLogo = `
    ██╗  ██╗██╗ ██████╗ ██╗  ██╗ ██████╗██╗      █████╗ ██╗    ██╗
    ██║  ██║██║██╔════╝ ██║  ██║██╔════╝██║     ██╔══██╗██║    ██║
    ███████║██║██║  ███╗███████║██║     ██║     ███████║██║ █╗ ██║
    ██╔══██║██║██║   ██║██╔══██║██║     ██║     ██╔══██║██║███╗██║
    ██║  ██║██║╚██████╔╝██║  ██║╚██████╗███████╗██║  ██║╚███╔███╔╝
    ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ `

// 标语
const tagline = "    High performance. Built for speed and reliability. 100% Go 100% Agnostic."

// 小型 Logo
const miniLogo = "⚡ HighClaw"

// Logo 样式
var (
	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Bold(true)

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	taglineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Italic(true)
)

// RenderFullLogo 渲染完整的带边框 Logo
func RenderFullLogo() string {
	return borderStyle.Render(lightningBorder) + "\n" +
		logoStyle.Render(mainLogo) + "\n\n" +
		taglineStyle.Render(tagline) + "\n\n" +
		borderStyle.Render(lightningBorder)
}

// RenderLogo 渲染主 Logo（不带边框）
func RenderLogo() string {
	return logoStyle.Render(mainLogo)
}

// RenderMiniLogo 渲染小型 Logo
func RenderMiniLogo() string {
	return logoStyle.Render(miniLogo)
}

// RenderBorder 渲染闪电边框
func RenderBorder() string {
	return borderStyle.Render(lightningBorder)
}
