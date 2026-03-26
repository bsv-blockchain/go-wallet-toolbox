package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type ExpandedTxView struct {
	manager    ManagerInterface
	user       *fixtures.UserConfig
	result     *NoSendSendWithResult
	txIndex    int
	parentView *PaginatedResultsView
}

func NewExpandedTxView(manager ManagerInterface, user *fixtures.UserConfig, result *NoSendSendWithResult, txIndex int, parent *PaginatedResultsView) *ExpandedTxView {
	return &ExpandedTxView{
		manager:    manager,
		user:       user,
		result:     result,
		txIndex:    txIndex,
		parentView: parent,
	}
}

func (m *ExpandedTxView) Init() tea.Cmd {
	return nil
}

func (m *ExpandedTxView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyCtrlC, tea.KeyEsc, tea.KeyEnter:
			return m.parentView, nil
		}
	}

	return m, nil
}

func (m *ExpandedTxView) View() string {
	if m.txIndex >= len(m.result.BroadcastedTxIds) {
		return "Invalid transaction index"
	}

	fullHash := m.result.BroadcastedTxIds[m.txIndex].String()

	var b strings.Builder

	fmt.Fprintf(&b, "Transaction %d Details\n\n", m.txIndex+1)

	b.WriteString("Full Transaction Hash:\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render(fullHash) + "\n\n")

	b.WriteString("Transaction successfully broadcast via SendWith operation.\n\n")

	b.WriteString("Press Enter or Esc to go back")

	return b.String()
}
