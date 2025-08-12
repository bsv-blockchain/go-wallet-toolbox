package tui

import (
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	tea "github.com/charmbracelet/bubbletea"
)

type SendDataSummary struct {
	manager     ManagerInterface
	user        *fixtures.UserConfig
	data        string
	summaryView *SummaryView
}

func NewSendDataSummary(manager ManagerInterface, user *fixtures.UserConfig, data string) *SendDataSummary {
	summary := []string{
		fmt.Sprintf("User: %s", user.Name),
		fmt.Sprintf("Action: Send Data"),
		fmt.Sprintf("Data Sent: %s", data),
		"Status: Success (simulation)",
	}
	return &SendDataSummary{
		manager:     manager,
		user:        user,
		data:        data,
		summaryView: NewSummaryView(summary, true),
	}
}

func (m *SendDataSummary) Init() tea.Cmd {
	return nil
}

func (m *SendDataSummary) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.summaryView, cmd = m.summaryView.Update(msg)

	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.Type == tea.KeyEnter && m.summaryView.continueIsFocused {
			// Go back to the select action screen
			return NewSelectAction(m.manager, m.user), nil
		}
	}

	return m, cmd
}

func (m *SendDataSummary) View() string {
	return m.summaryView.View()
}

