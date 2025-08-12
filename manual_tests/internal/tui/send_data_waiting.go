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

type sendDataResultMsg struct{ err error }

func (m *SendDataWaiting) sendData() tea.Msg {
	// TODO: implement manager.SendData(m.user, m.data)
	// For now, we'll just simulate a successful operation
	return sendDataResultMsg{err: nil}
}

func (m *SendDataWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case sendDataResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil // Keep view and show error
		}
		summaryView := NewSendDataSummary(m.manager, m.user, m.data)
		return summaryView, summaryView.Init()
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m *SendDataWaiting) View() string {
	if m.quitting {
		return "Quitting..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	return fmt.Sprintf("%s Sending data...", m.spinner.View())
}
