package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type NoSendSendWithResult struct {
	NoSendTimes      []time.Duration
	SendWithTime     time.Duration
	MinNoSendTime    time.Duration
	MaxNoSendTime    time.Duration
	AvgNoSendTime    time.Duration
	TotalTxCount     int
	BroadcastedTxIds []chainhash.Hash
}

type PaginatedResultsView struct {
	manager        ManagerInterface
	user           *fixtures.UserConfig
	result         *NoSendSendWithResult
	currentPage    int
	itemsPerPage   int
	cursor         int
	showingResults bool
}

func NewPaginatedResultsView(manager ManagerInterface, user *fixtures.UserConfig, result *NoSendSendWithResult) *PaginatedResultsView {
	return &PaginatedResultsView{
		manager:        manager,
		user:           user,
		result:         result,
		currentPage:    0,
		itemsPerPage:   10,
		cursor:         0,
		showingResults: true,
	}
}

func (m *PaginatedResultsView) Init() tea.Cmd {
	return nil
}

func (m *PaginatedResultsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyTab:
			m.showingResults = !m.showingResults
			m.cursor = 0
			if !m.showingResults {
				m.currentPage = 0
			}
			return m, nil

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyDown:
			if m.showingResults {
				if m.cursor < 1 {
					m.cursor++
				}
			} else {
				maxCursor := m.getVisibleItemCount() + 2
				if m.cursor < maxCursor {
					m.cursor++
				}
			}

		case tea.KeyLeft:
			if !m.showingResults && m.currentPage > 0 {
				m.currentPage--
				m.cursor = 0
			}

		case tea.KeyRight:
			if !m.showingResults {
				maxPage := m.getMaxPage()
				if m.currentPage < maxPage {
					m.currentPage++
					m.cursor = 0
				}
			}

		case tea.KeyEnter:
			return m.handleEnter()
		}
	}

	return m, nil
}

func (m *PaginatedResultsView) handleEnter() (tea.Model, tea.Cmd) {
	if m.showingResults {
		switch m.cursor {
		case 0:
			selectAction := NewSelectAction(m.manager, m.user)
			return selectAction, selectAction.Init()
		case 1:
			m.showingResults = false
			m.cursor = 0
			return m, nil
		}
	} else {
		visibleCount := m.getVisibleItemCount()

		if m.cursor < visibleCount {
			actualTxIndex := m.currentPage*m.itemsPerPage + m.cursor
			if actualTxIndex < len(m.result.BroadcastedTxIds) {
				expandedView := NewExpandedTxView(m.manager, m.user, m.result, actualTxIndex, m)
				return expandedView, expandedView.Init()
			}
		} else if m.cursor == visibleCount {
			m.showingResults = true
			m.cursor = 0
			return m, nil
		}
	}

	return m, nil
}

func (m *PaginatedResultsView) getMaxPage() int {
	totalTx := len(m.result.BroadcastedTxIds)
	if totalTx == 0 {
		return 0
	}
	return (totalTx - 1) / m.itemsPerPage
}

func (m *PaginatedResultsView) getVisibleItemCount() int {
	totalTx := len(m.result.BroadcastedTxIds)
	startIdx := m.currentPage * m.itemsPerPage
	endIdx := startIdx + m.itemsPerPage

	if endIdx > totalTx {
		endIdx = totalTx
	}

	return endIdx - startIdx
}

func (m *PaginatedResultsView) View() string {
	if m.showingResults {
		return m.viewSummary()
	}
	return m.viewTxList()
}

func (m *PaginatedResultsView) viewSummary() string {
	var b strings.Builder

	b.WriteString("NoSend/SendWith Test Results\n\n")

	fmt.Fprintf(&b, "✓ Created %d NoSend transactions and broadcast them via SendWith\n\n", m.result.TotalTxCount)

	b.WriteString("NoSend Creation Performance:\n")
	fmt.Fprintf(&b, "  Min time: %v\n", m.result.MinNoSendTime)
	fmt.Fprintf(&b, "  Max time: %v\n", m.result.MaxNoSendTime)
	fmt.Fprintf(&b, "  Avg time: %v\n\n", m.result.AvgNoSendTime)

	fmt.Fprintf(&b, "SendWith broadcast operation: %v\n\n", m.result.SendWithTime)

	options := []string{
		"Continue to menu",
		"View transaction list",
	}

	for i, option := range options {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		line := cursor + option
		if m.cursor == i {
			highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true)
			line = highlightStyle.Render(line)
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\nPress Tab to switch views, ↑/↓ to navigate, Enter to select")

	return b.String()
}

func (m *PaginatedResultsView) viewTxList() string {
	var b strings.Builder

	totalTx := len(m.result.BroadcastedTxIds)
	maxPage := m.getMaxPage()

	fmt.Fprintf(&b, "Transaction List (Page %d/%d)\n\n", m.currentPage+1, maxPage+1)

	startIdx := m.currentPage * m.itemsPerPage
	endIdx := startIdx + m.itemsPerPage
	if endIdx > totalTx {
		endIdx = totalTx
	}

	for i := startIdx; i < endIdx; i++ {
		localIdx := i - startIdx
		cursor := "  "
		if m.cursor == localIdx {
			cursor = "> "
		}

		fullHash := m.result.BroadcastedTxIds[i].String()
		shortHash := fullHash[:8] + "..." + fullHash[len(fullHash)-8:]

		line := fmt.Sprintf("%s%d. %s", cursor, i+1, shortHash)

		if m.cursor == localIdx {
			highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true)
			line = highlightStyle.Render(line)
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n")

	backCursor := "  "
	visibleCount := m.getVisibleItemCount()
	if m.cursor == visibleCount {
		backCursor = "> "
	}

	backLine := backCursor + "Back to summary"
	if m.cursor == visibleCount {
		highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true)
		backLine = highlightStyle.Render(backLine)
	}
	b.WriteString(backLine + "\n\n")

	fmt.Fprintf(&b, "Showing %d-%d of %d transactions\n", startIdx+1, endIdx, totalTx)

	instructions := "↑/↓ Navigate, Enter to view full hash, ←/→ Change page, Tab for summary"
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(instructions))

	return b.String()
}
