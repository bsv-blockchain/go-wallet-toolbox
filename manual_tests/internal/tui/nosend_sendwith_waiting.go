package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

const (
	padding  = 2
	maxWidth = 80
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render

type NoSendSendWithWaiting struct {
	manager          ManagerInterface
	user             *fixtures.UserConfig
	transactionCount int
	dataPrefix       string
	progress         progress.Model
	phase            string
	completed        bool
	currentPhase     int
	expectedTxCount  int
}

type noSendSendWithProgressMsg struct {
	completed bool
	err       error
	result    *NoSendSendWithResult
}

type noSendSendWithTickMsg time.Time

func NewNoSendSendWithWaiting(manager ManagerInterface, user *fixtures.UserConfig, count int, prefix string) *NoSendSendWithWaiting {
	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = maxWidth

	return &NoSendSendWithWaiting{
		manager:          manager,
		user:             user,
		transactionCount: count,
		dataPrefix:       prefix,
		progress:         prog,
		phase:            fmt.Sprintf("Phase 1/2: Creating %d NoSend transactions...", count),
		currentPhase:     0,
		expectedTxCount:  count,
	}
}

func (m *NoSendSendWithWaiting) Init() tea.Cmd {
	return tea.Batch(
		m.executeNoSendSendWith(),
		m.tickCmd(),
	)
}

func (m *NoSendSendWithWaiting) executeNoSendSendWith() tea.Cmd {
	return func() tea.Msg {
		result, err := m.manager.ExecuteNoSendSendWith(*m.user, m.transactionCount, m.dataPrefix)
		return noSendSendWithProgressMsg{
			completed: true,
			err:       err,
			result:    result,
		}
	}
}

func (m *NoSendSendWithWaiting) tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return noSendSendWithTickMsg(t)
	})
}

func (m *NoSendSendWithWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case noSendSendWithTickMsg:
		if m.completed {
			return m, nil
		}

		if m.currentPhase == 0 {
			if m.progress.Percent() < 0.5 {
				cmd := m.progress.IncrPercent(0.02)
				return m, tea.Batch(m.tickCmd(), cmd)
			}
		} else {
			if m.progress.Percent() < 0.5 {
				m.progress.SetPercent(0.5)
			}
			return m, m.tickCmd()
		}

		if m.currentPhase == 0 && m.progress.Percent() >= 0.5 {
			m.currentPhase = 1
			m.phase = "Phase 2/2: Broadcasting with SendWith..."
		}

		return m, m.tickCmd()

	case noSendSendWithProgressMsg:
		m.completed = msg.completed

		if msg.err != nil {
			m.progress.SetPercent(1.0)
			goToSelectAction := func() tea.Model {
				return NewSelectAction(m.manager, m.user)
			}

			resultView := NewResultView(
				m.manager,
				"NoSend/SendWith test failed: "+msg.err.Error(),
				ResultViewError,
				goToSelectAction,
				fixtures.Summary{},
			)
			return resultView, resultView.Init()
		}

		m.progress.SetPercent(1.0)
		m.phase = "NoSend/SendWith test completed!"

		paginatedResults := NewPaginatedResultsView(m.manager, m.user, msg.result)
		return paginatedResults, paginatedResults.Init()

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m *NoSendSendWithWaiting) View() string {
	pad := strings.Repeat(" ", padding)

	var b strings.Builder
	b.Grow(512)

	b.WriteString("\n")
	b.WriteString(pad)
	b.WriteString(m.phase)
	b.WriteString("\n\n")

	b.WriteString(pad)
	b.WriteString(m.progress.View())
	b.WriteString("\n\n")

	if m.completed {
		b.WriteString(pad)
		b.WriteString(helpStyle("Press Ctrl+C to cancel"))
		return b.String()
	}

	now := time.Now().UnixNano()
	currentPercent := m.progress.Percent() * 100

	if m.currentPhase == 0 {
		fmt.Fprintf(&b, "%sCreating NoSend transactions: %.0f%%\n\n", pad, currentPercent)
	} else {
		dots := loadingDots(now)
		fmt.Fprintf(&b, "%sBroadcasting transaction%s\n\n", pad, dots)

		spinnerChar := spinnerHelper(now)
		statusText := fmt.Sprintf("%s Broadcasting SendWith transaction with %d transactions, please wait...",
			spinnerChar, m.expectedTxCount)
		b.WriteString(pad)
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(statusText))
		b.WriteString("\n\n")
	}

	b.WriteString(pad)
	b.WriteString(helpStyle("Press Ctrl+C to cancel"))

	return b.String()
}

func spinnerHelper(nano int64) string {
	s := []string{"|", "/", "-", "\\"}
	return s[(nano/2e8)%int64(len(s))]
}

func loadingDots(nano int64) string {
	return [4]string{"   ", ".  ", ".. ", "..."}[(nano/4e8)%4]
}
