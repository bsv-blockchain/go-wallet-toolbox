package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type SendP2pkhWaiting struct {
	manager ManagerInterface
	user    *fixtures.UserConfig
	address string
	amount  uint64
	spinner spinner.Model
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
	err      error
	summary  fixtures.Summary
	duration time.Duration
}

func (m *SendP2pkhWaiting) sendPayment() tea.Msg {
	start := time.Now()
	_, summary, err := m.manager.CreateActionWithP2pkh(*m.user, m.address, m.amount)
	dur := time.Since(start)

	return sendP2pkhResultMsg{
		err:      err,
		summary:  summary,
		duration: dur,
	}
}

func (m *SendP2pkhWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case sendP2pkhResultMsg:
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

func (m *SendP2pkhWaiting) View() string {
	return fmt.Sprintf("%s Creating P2PKH payment transaction...", m.spinner.View())
}
