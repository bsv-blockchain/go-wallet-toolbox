package tui

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type SendP2pkhWaiting struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	address  string
	amount   uint64
	spinner  spinner.Model
	quitting bool
	err      error
}

func NewSendP2pkhWaiting(manager ManagerInterface, user *fixtures.UserConfig, address string, amount uint64) *SendP2pkhWaiting {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &SendP2pkhWaiting{
		manager: manager,
		user:    user,
		address: address,
		amount:  amount,
		spinner: s,
	}
}

func (m *SendP2pkhWaiting) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.sendPayment)
}

type sendP2pkhResultMsg struct {
	err     error
	summary fixtures.Summary
}

func (m *SendP2pkhWaiting) sendPayment() tea.Msg {
	_, summary, err := m.manager.CreateActionWithP2pkh(*m.user, m.address, m.amount)
	return sendP2pkhResultMsg{
		err:     err,
		summary: summary,
	}
}

func (m *SendP2pkhWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case sendP2pkhResultMsg:
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

func (m *SendP2pkhWaiting) View() string {
	return fmt.Sprintf("%s Creating P2PKH payment transaction...", m.spinner.View())
}
