package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type sendP2pkhTickMsg struct{}

type SendP2pkhPeriodicallyWaiting struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	address  string
	amount   uint64
	period   time.Duration
	spinner  spinner.Model
	events   []string
	sentTx   int
	quitting bool
}

func NewSendP2pkhPeriodicallyWaiting(manager ManagerInterface, user *fixtures.UserConfig, address string, amount uint64, period time.Duration) *SendP2pkhPeriodicallyWaiting {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &SendP2pkhPeriodicallyWaiting{
		manager: manager,
		user:    user,
		address: address,
		amount:  amount,
		period:  period,
		spinner: s,
		events:  make([]string, 0, 10),
	}
}

func (m *SendP2pkhPeriodicallyWaiting) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.sendPaymentPeriodically())
}

type p2pkhSentResultMsg struct {
	txID     string
	err      error
	duration time.Duration
}

func (m *SendP2pkhPeriodicallyWaiting) sendPaymentPeriodically() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		txID, _, err := m.manager.CreateActionWithP2pkh(*m.user, m.address, m.amount)
		dur := time.Since(start)
		return p2pkhSentResultMsg{txID: txID, err: err, duration: dur}
	}
}

func (m *SendP2pkhPeriodicallyWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "s":
			m.quitting = true
			goToSelect := func() tea.Model { return NewSelectAction(m.manager, m.user) }
			res := NewResultView(m.manager, "Stopping periodic P2PKH sending", ResultViewSuccess, goToSelect, append(m.events, fmt.Sprintf("Sent %d transactions", m.sentTx)))
			return res, res.Init()
		}
	case p2pkhSentResultMsg:
		if msg.err != nil {
			m.events = append(m.events, fmt.Sprintf("Error: %v", msg.err))
		} else {
			m.events = append(m.events, fmt.Sprintf("Transaction %d sent: %s in %d ms", m.sentTx, msg.txID, msg.duration.Milliseconds()))
			m.sentTx++
		}
		if len(m.events) > 10 {
			m.events = m.events[1:]
		}
		if !m.quitting {
			return m, tea.Tick(m.period, func(t time.Time) tea.Msg { return sendP2pkhTickMsg{} })
		}
	case sendP2pkhTickMsg:
		return m, m.sendPaymentPeriodically()
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *SendP2pkhPeriodicallyWaiting) View() string {
	if m.quitting {
		return ""
	}
	out := fmt.Sprintf("%s Sending P2PKH periodically...\n\n", m.spinner.View())
	for i := len(m.events) - 1; i >= 0; i-- {
		out += m.events[i] + "\n"
	}
	out += "\nPress 's' to stop."
	return out
}
