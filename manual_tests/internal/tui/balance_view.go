package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

var balanceStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#00ff00")).
	SetString("Balance:")

type balanceResultMsg struct {
	balance string
	stats   map[string]int
	err     error
}

type balanceView struct {
	manager ManagerInterface
	user    *fixtures.UserConfig
	balance string
	stats   map[string]int
	spinner spinner.Model
	loading bool
	err     error
	focus   bool
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
		focus:   false,
	}
}

func (m balanceView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.calculateBalance())
}

func (m balanceView) calculateBalance() tea.Cmd {
	return func() tea.Msg {
		balance, err := m.manager.Balance(*m.user)
		if err != nil {
			return balanceResultMsg{err: fmt.Errorf("failed to calculate balance for %s: %w", m.user.Name, err)}
		}

		stats, err := m.manager.ActionsStats(*m.user)
		if err != nil {
			return balanceResultMsg{err: fmt.Errorf("failed to get actions stats for %s: %w", m.user.Name, err)}
		}

		return balanceResultMsg{
			balance: fmt.Sprintf("%d sats", balance),
			stats:   stats,
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
		case "enter":
			if m.focus {
				return NewSelectAction(m.manager, m.user), nil
			}
		}
	case balanceResultMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.balance = msg.balance
			m.stats = msg.stats
			m.focus = true
		}
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

	// Build stats section
	var statsSection string
	if len(m.stats) > 0 {
		keys := make([]string, 0, len(m.stats))
		for k := range m.stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		lines := make([]string, 0, len(keys))
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("- %s: %d", k, m.stats[k]))
		}
		statsHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ff00")).Render("Tx Stats:")
		statsSection = fmt.Sprintf("%s\n%s", statsHeader, strings.Join(lines, "\n"))
	}

	var buttons string
	if m.focus {
		buttons = navStyleFocused.Render(fixtures.ButtonBack)
	} else {
		buttons = navStyle.Render(fixtures.ButtonBack)
	}

	if statsSection != "" {
		return fmt.Sprintf(
			"Balance for %s\n\n%s %s\n\n%s\n\n%s\n\n(press 'q' to quit)",
			m.user.Name,
			balanceStyle.Render(),
			balanceValue,
			statsSection,
			buttons,
		)
	}

	return fmt.Sprintf(
		"Balance for %s\n\n%s %s\n\n%s\n\n(press 'q' to quit)",
		m.user.Name,
		balanceStyle.Render(),
		balanceValue,
		buttons,
	)
}
