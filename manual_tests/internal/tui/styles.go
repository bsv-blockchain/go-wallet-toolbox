package tui

import "github.com/charmbracelet/lipgloss"

var (
	inputStyle             = lipgloss.NewStyle().Foreground(hotPink)
	calculatedAddressStyle = lipgloss.NewStyle().Foreground(hotBlue).Italic(true)
	navStyle               = lipgloss.NewStyle().Foreground(darkGray)
	navStyleFocused        = lipgloss.NewStyle().Foreground(hotPink).Underline(true)
	errorStyle             = lipgloss.NewStyle().Foreground(hitRed).Bold(true)
)
