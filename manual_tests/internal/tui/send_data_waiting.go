package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type SendDataWaiting struct {
	manager ManagerInterface
	user    *fixtures.UserConfig
	data    string
	spinner spinner.Model
}

func NewSendDataWaiting(manager ManagerInterface, user *fixtures.UserConfig, data string) *SendDataWaiting {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &SendDataWaiting{
		manager: manager,
		user:    user,
		data:    data,
		spinner: s,
	}
}

func (m *SendDataWaiting) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.sendData)
}

type sendDataResultMsg struct {
	err      error
	summary  fixtures.Summary
	duration time.Duration
}

func (m *SendDataWaiting) sendData() tea.Msg {
	startTime := time.Now()
	_, summary, err := m.manager.CreateActionWithData(*m.user, m.data)
	duration := time.Since(startTime)
	return sendDataResultMsg{
		err:      err,
		summary:  summary,
		duration: duration,
	}
}

func (m *SendDataWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case sendDataResultMsg:
		goToSelectAction := func() tea.Model {
			return NewSelectAction(m.manager, m.user)
		}

		var mode ResultViewMode
		var resultMsg string

		if msg.err != nil {
			mode = ResultViewError
			resultMsg = "Failed to create transaction: " + msg.err.Error()
		} else {
			mode = ResultViewSuccess
			resultMsg = fmt.Sprintf("Transaction created successfully in %d ms!", msg.duration.Milliseconds())
		}

		resultView := NewResultView(m.manager, resultMsg, mode, goToSelectAction, msg.summary)
		return resultView, resultView.Init()
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m *SendDataWaiting) View() string {
	return fmt.Sprintf("%s Creating action with data transaction...", m.spinner.View())
}
