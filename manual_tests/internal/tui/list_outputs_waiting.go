package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type ListOutputsWaiting struct {
	manager       ManagerInterface
	user          *fixtures.UserConfig
	limit         uint32
	offset        uint32
	spinner       spinner.Model
	includeLabels bool
	basket        string
}

func NewListOutputsWaiting(manager ManagerInterface, user *fixtures.UserConfig, limit, offset uint32, includeLabels bool, basket string) *ListOutputsWaiting {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &ListOutputsWaiting{
		manager:       manager,
		user:          user,
		limit:         limit,
		offset:        offset,
		spinner:       s,
		includeLabels: includeLabels,
		basket:        basket,
	}
}

func (m *ListOutputsWaiting) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.listOutputs)
}

type listOutputsResultMsg struct {
	err     error
	summary fixtures.Summary
}

func (m *ListOutputsWaiting) listOutputs() tea.Msg {
	summary, err := m.manager.ListOutputs(*m.user, m.limit, m.offset, m.includeLabels, m.basket)
	return listOutputsResultMsg{err: err, summary: summary}
}

func (m *ListOutputsWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case listOutputsResultMsg:
		goToSelectAction := func() tea.Model { return NewSelectAction(m.manager, m.user) }

		mode := ResultViewSuccess
		resultMsg := "Outputs listed successfully!"
		if msg.err != nil {
			mode = ResultViewError
			resultMsg = "Failed to list outputs: " + msg.err.Error()
		}

		resultView := NewResultView(m.manager, resultMsg, mode, goToSelectAction, msg.summary)
		return resultView, resultView.Init()
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m *ListOutputsWaiting) View() string {
	return fmt.Sprintf("%s Listing outputs...", m.spinner.View())
}
