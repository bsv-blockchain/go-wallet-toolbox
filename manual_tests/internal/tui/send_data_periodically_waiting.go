package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type sendDataTickMsg struct{}

type SendDataPeriodicallyWaiting struct {
	manager     ManagerInterface
	user        *fixtures.UserConfig
	dataPrefix  string
	period      time.Duration
	spinner     spinner.Model
	events      []string
	sentTxCount int
	quitting    bool
}

func NewSendDataPeriodicallyWaiting(manager ManagerInterface, user *fixtures.UserConfig, dataPrefix string, period time.Duration) *SendDataPeriodicallyWaiting {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &SendDataPeriodicallyWaiting{
		manager:    manager,
		user:       user,
		dataPrefix: dataPrefix,
		period:     period,
		spinner:    s,
		events:     make([]string, 0, 10),
	}
}

func (m *SendDataPeriodicallyWaiting) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.sendDataPeriodically())
}

type dataSentResultMsg struct {
	txID     string
	err      error
	duration time.Duration
}

func (m *SendDataPeriodicallyWaiting) sendDataPeriodically() tea.Cmd {
	return func() tea.Msg {
		data := fmt.Sprintf("%s %d %s", m.dataPrefix, m.sentTxCount, time.Now().Format(time.RFC3339Nano))
		startTime := time.Now()
		txID, _, err := m.manager.CreateActionWithData(*m.user, data)
		duration := time.Since(startTime)
		return dataSentResultMsg{
			txID:     txID,
			err:      err,
			duration: duration,
		}
	}
}

func (m *SendDataPeriodicallyWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "s":
			m.quitting = true
			goToSelectAction := func() tea.Model {
				return NewSelectAction(m.manager, m.user)
			}
			resView := NewResultView(
				m.manager,
				"Stopping periodic data sending",
				ResultViewSuccess,
				goToSelectAction,
				append(m.events, fmt.Sprintf("Sent %d transactions", m.sentTxCount)),
			)
			return resView, resView.Init()
		}
	case dataSentResultMsg:
		if msg.err != nil {
			m.events = append(m.events, fmt.Sprintf("Error: %v", msg.err))
		} else {
			m.events = append(m.events, fmt.Sprintf("Transaction %d sent: %s in %d ms", m.sentTxCount, msg.txID, msg.duration.Milliseconds()))
			m.sentTxCount++
		}
		if len(m.events) > 10 {
			m.events = m.events[1:]
		}
		if !m.quitting {
			return m, tea.Tick(m.period, func(t time.Time) tea.Msg {
				return sendDataTickMsg{}
			})
		}
	case sendDataTickMsg:
		return m, m.sendDataPeriodically()
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)

	return m, cmd
}

func (m *SendDataPeriodicallyWaiting) View() string {
	if m.quitting {
		return ""
	}
	s := fmt.Sprintf("%s Sending data periodically...\n\n", m.spinner.View())
	for i := len(m.events) - 1; i >= 0; i-- {
		s += m.events[i] + "\n"
	}
	s += "\nPress 's' to stop."
	return s
}
