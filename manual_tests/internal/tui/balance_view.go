package tui

import (
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var balanceStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#00ff00")).
	SetString("Balance:")

type balanceResultMsg struct {
	balance string
	err     error
}

type balanceView struct {
	manager ManagerInterface
	user    *fixtures.UserConfig
	balance string
	spinner spinner.Model
	loading bool
	err     error
}

func NewBalanceView(manager ManagerInterface, user *fixtures.UserConfig) tea.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return balanceView{
		manager: manager,
		user:    user,
		spinner: s,
		loading: true,
	}
}

func (m balanceView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.calculateBalance())
}

func (m balanceView) calculateBalance() tea.Cmd {
	return func() tea.Msg {
		balance, err := m.manager.Balance(*m.user)
		if err != nil {
			return balanceResultMsg{balance: "", err: fmt.Errorf("failed to calculate balance for %s: %w", m.user.Name, err)}
		}

		return balanceResultMsg{
			balance: fmt.Sprintf("%d sats", balance),
			err:     nil,
		}
	}
}

func (m balanceView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		}
	case balanceResultMsg:
		m.loading = false
		m.balance = msg.balance
		m.err = msg.err
		return m, nil

	default:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m balanceView) View() string {
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
		return fmt.Sprintf(
			"Error calculating balance for %s\n\n%s\n\n(press 'q' to quit)",
			m.user.Name,
			errorStyle.Render(m.err.Error()),
		)
	}
	if m.loading {
		return fmt.Sprintf(
			"Calculating balance for %s\n\n   %s Please wait...\n\n(press 'q' to quit)",
			m.user.Name,
			m.spinner.View(),
		)
	}
	balanceValue := lipgloss.NewStyle().Bold(true).Render(m.balance)
	return fmt.Sprintf(
		"Balance for %s\n\n%s %s\n\n(press 'q' to quit)",
		m.user.Name,
		balanceStyle.Render(),
		balanceValue,
	)
}
