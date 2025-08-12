package tui

import (
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type SendDataWaiting struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	data     string
	spinner  spinner.Model
	quitting bool
	err      error
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
	err     error
	summary fixtures.Summary
}

func (m *SendDataWaiting) sendData() tea.Msg {
	summary, err := m.manager.CreateActionWithData(*m.user, m.data)
	return sendDataResultMsg{
		err:     err,
		summary: summary,
	}
}

func (m *SendDataWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case sendDataResultMsg:
		goToSelectAction := func() tea.Model {
			return NewSelectAction(m.manager, m.user)
		}

		mode := ResultViewSuccess
		resultMsg := "Transaction created successfully!"

		if msg.err != nil {
			mode = ResultViewError
			resultMsg = "Failed to create transaction: " + msg.err.Error()
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
