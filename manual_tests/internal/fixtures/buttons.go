package fixtures

import "github.com/charmbracelet/lipgloss"

var (
	FocusedButton = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	BlurredButton = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

const (
	ButtonBack     = "<- Back"
	ButtonContinue = "Continue ->"
)
