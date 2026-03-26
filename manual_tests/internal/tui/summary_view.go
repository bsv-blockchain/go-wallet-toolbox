package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

const (
	summaryWidth     = 100
	maxExpandedLines = 10
)

var (
	summaryStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("240")).
			PaddingLeft(1)
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15"))
	pageInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")).
			Italic(true)
)

type SummaryView struct {
	summary           []string
	cursor            int
	expanded          int
	expandedPage      int
	showContinue      bool
	continueIsFocused bool
}

func NewSummaryView(summary []string, showContinue bool) *SummaryView {
	return &SummaryView{
		summary:           summary,
		expanded:          -1,
		expandedPage:      0,
		showContinue:      showContinue,
		continueIsFocused: len(summary) == 0,
	}
}

func (m *SummaryView) Init() tea.Cmd {
	return nil
}

func (m *SummaryView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.expanded >= 0 && !m.continueIsFocused {
			switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
			case tea.KeyLeft, tea.KeyCtrlB:
				if m.expandedPage > 0 {
					m.expandedPage--
				}
				return m, nil
			case tea.KeyRight, tea.KeyCtrlF:
				lines := m.getExpandedLines(m.expanded)
				totalPages := (len(lines) + maxExpandedLines - 1) / maxExpandedLines
				if m.expandedPage < totalPages-1 {
					m.expandedPage++
				}
				return m, nil
			}
		}

		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyUp:
			if m.continueIsFocused {
				m.continueIsFocused = false
				m.cursor = len(m.summary) - 1
			} else if m.cursor > 0 {
				m.cursor--
				m.expandedPage = 0
			}
		case tea.KeyDown:
			if m.cursor == len(m.summary)-1 {
				m.continueIsFocused = true
			} else if m.cursor < len(m.summary)-1 {
				m.cursor++
				m.expandedPage = 0
			}
		case tea.KeyEnter:
			if m.continueIsFocused {
				return m, nil
			}
			if m.expanded == m.cursor {
				m.expanded = -1 // Collapse
				m.expandedPage = 0
			} else {
				m.expanded = m.cursor // Expand
				m.expandedPage = 0
			}
		}
	}
	return m, nil
}

func wrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}

	var lines []string
	for i := 0; i < len(text); i += width {
		end := i + width
		if end > len(text) {
			end = len(text)
		}
		lines = append(lines, text[i:end])
	}
	return lines
}

func isLongHexString(s string) bool {
	if len(s) < summaryWidth {
		return false
	}

	return strings.Contains(s, "0101010") ||
		strings.Contains(s, "atomic beef") ||
		strings.Contains(s, "locking script") ||
		strings.Contains(s, "CreateActionResult") ||
		(len(s) > summaryWidth && strings.ContainsAny(s, "0123456789abcdefABCDEF"))
}

func (m *SummaryView) getExpandedLines(index int) []string {
	if index < 0 || index >= len(m.summary) {
		return []string{}
	}

	s := m.summary[index]
	if isLongHexString(s) {
		return wrapText(s, summaryWidth)
	}
	return []string{s}
}

func (m *SummaryView) getPagedLines(index int) []string {
	lines := m.getExpandedLines(index)

	startIdx := m.expandedPage * maxExpandedLines
	endIdx := startIdx + maxExpandedLines

	if startIdx >= len(lines) {
		return []string{}
	}

	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	return lines[startIdx:endIdx]
}

func (m *SummaryView) View() string {
	var b strings.Builder
	for i, s := range m.summary {
		if m.expanded != i {
			if len(s) > summaryWidth {
				line := s[:summaryWidth-3] + "..."
				if !m.continueIsFocused && m.cursor == i {
					b.WriteString(selectedStyle.Render(line))
				} else {
					b.WriteString(line)
				}
			} else {
				if !m.continueIsFocused && m.cursor == i {
					b.WriteString(selectedStyle.Render(s))
				} else {
					b.WriteString(s)
				}
			}
			b.WriteString("\n")
		} else {
			lines := m.getPagedLines(i)
			totalLines := len(m.getExpandedLines(i))
			totalPages := (totalLines + maxExpandedLines - 1) / maxExpandedLines

			for j, line := range lines {
				if !m.continueIsFocused && m.cursor == i && j == 0 {
					b.WriteString(selectedStyle.Render(line))
				} else {
					b.WriteString(line)
				}
				b.WriteString("\n")
			}

			if totalPages > 1 {
				pageInfo := pageInfoStyle.Render(
					fmt.Sprintf("Page %d/%d (←/→ to navigate, Enter to collapse)",
						m.expandedPage+1, totalPages))
				b.WriteString(pageInfo + "\n")
			}
		}
	}

	continueButton := ""
	if m.showContinue {
		continueButton = to.IfThen(m.continueIsFocused, navStyleFocused).ElseThen(navStyle).
			Render(fixtures.ButtonContinue)
	}

	return summaryStyle.Render(b.String()) + "\n" + continueButton
}

func (m *SummaryView) ContinueFocused() bool {
	return m.continueIsFocused
}

// FocusContinue programmatically focuses the Continue button
func (m *SummaryView) FocusContinue() {
	m.continueIsFocused = true
}
