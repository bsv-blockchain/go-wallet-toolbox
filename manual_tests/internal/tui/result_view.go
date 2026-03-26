package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ResultViewMode int

const (
	ResultViewSuccess ResultViewMode = iota
	ResultViewError
)

const (
	resultViewWidth = 100
)

var (
	resultViewSuccessStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("34")). // green border
				Foreground(lipgloss.Color("15"))
	resultViewErrorStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("196")). // red border
				Foreground(lipgloss.Color("15"))
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
)

// ResultView is a generic view to show a message and wait for the user to press enter.
type ResultView struct {
	manager     ManagerInterface
	message     string
	mode        ResultViewMode
	nextView    func() tea.Model
	summary     []string
	summaryView *SummaryView
}

// NewResultView creates a new ResultView.
func NewResultView(manager ManagerInterface, message string, mode ResultViewMode, nextView func() tea.Model, summary []string) *ResultView {
	sv := NewSummaryView(summary, nextView != nil)
	if len(summary) > 0 {
		// Focus the Continue button by default for easier navigation
		sv.FocusContinue()
	}
	return &ResultView{
		manager:     manager,
		message:     message,
		mode:        mode,
		nextView:    nextView,
		summary:     summary,
		summaryView: sv,
	}
}

// Init initializes the model.
func (m *ResultView) Init() tea.Cmd {
	if m.summaryView != nil {
		return m.summaryView.Init()
	}
	return nil
}

// Update handles messages.
func (m *ResultView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.summary) > 0 {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
			case tea.KeyEnter:
				if m.summaryView.ContinueFocused() {
					if m.nextView != nil {
						return m.nextView(), nil
					}
					return m, tea.Quit
				}
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			}
		}
		var cmd tea.Cmd
		_, cmd = m.summaryView.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.nextView != nil {
				return m.nextView(), nil
			}
			return m, tea.Quit
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the model.
func (m *ResultView) View() string {
	var style lipgloss.Style
	switch m.mode { //nolint:exhaustive // ResultViewSuccess uses default styling
	case ResultViewError:
		style = resultViewErrorStyle
	default:
		style = resultViewSuccessStyle
	}
	style = style.Width(resultViewWidth)
	msg := style.Render(m.message)
	if len(m.summary) > 0 {
		return msg + "\n\n" + m.summaryView.View()
	}

	return msg +
		"\n\n" + promptStyle.Render("Press enter to continue.")
}
