package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	summaryWidth = 80
)

var (
	summaryStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("240")).
			PaddingLeft(1)
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15"))
)

type SummaryView struct {
	summary           []string
	cursor            int
	expanded          int
	showContinue      bool
	continueIsFocused bool
}

func NewSummaryView(summary []string, showContinue bool) *SummaryView {
	return &SummaryView{
		summary:           summary,
		expanded:          -1,
		showContinue:      showContinue,
		continueIsFocused: len(summary) == 0,
	}
}

func (m *SummaryView) Init() tea.Cmd {
	return nil
}

func (m *SummaryView) Update(msg tea.Msg) (*SummaryView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if m.continueIsFocused {
				m.continueIsFocused = false
				m.cursor = len(m.summary) - 1
			} else if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown:
			if m.cursor == len(m.summary)-1 {
				m.continueIsFocused = true
			} else if m.cursor < len(m.summary)-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			if m.continueIsFocused {
				return m, nil
			}
			if m.expanded == m.cursor {
				m.expanded = -1 // Collapse
			} else {
				m.expanded = m.cursor // Expand
			}
		}
	}
	return m, nil
}

func (m *SummaryView) View() string {
	var b strings.Builder
	for i, s := range m.summary {
		line := ""
		if m.expanded != i {
			if len(s) > summaryWidth {
				line = s[:summaryWidth-3] + "..."
			} else {
				line = s
			}
		} else {
			line = s
		}

		if !m.continueIsFocused && m.cursor == i {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	continueButton := ""
	if m.showContinue {
		continueButton = to.IfThen(m.continueIsFocused, continueStyleFocused).ElseThen(continueStyle).
			Render("Continue ->")
	}

	return summaryStyle.Render(b.String()) + "\n" + continueButton
}

func (m *SummaryView) ContinueFocused() bool {
	return m.continueIsFocused
}

