package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SendP2pkhPeriodicallyForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	inputs   []textinput.Model
	focused  int
	errorMsg string
}

func NewSendP2pkhPeriodicallyForm(manager ManagerInterface, user *fixtures.UserConfig) *SendP2pkhPeriodicallyForm {
	inputs := make([]textinput.Model, 3)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Recipient address (P2PKH)"
	inputs[0].Focus()
	inputs[0].CharLimit = 128
	inputs[0].Width = 50
	inputs[0].Prompt = ""

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Satoshis to send"
	inputs[1].CharLimit = 20
	inputs[1].Width = 50
	inputs[1].Prompt = ""

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Time period (ms)"
	inputs[2].CharLimit = 10
	inputs[2].Width = 50
	inputs[2].Prompt = ""

	return &SendP2pkhPeriodicallyForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focused: 0,
	}
}

func (m *SendP2pkhPeriodicallyForm) Init() tea.Cmd { return textinput.Blink }

func (m *SendP2pkhPeriodicallyForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.continueIsFocused() {
				addr := strings.TrimSpace(m.inputs[0].Value())
				if addr == "" {
					m.errorMsg = "Recipient address is required"
					return m, nil
				}
				amountStr := strings.TrimSpace(m.inputs[1].Value())
				amt, err := strconv.ParseUint(amountStr, 10, 64)
				if err != nil || amt == 0 {
					m.errorMsg = "Invalid satoshis amount"
					return m, nil
				}
				periodStr := strings.TrimSpace(m.inputs[2].Value())
				periodMs, err := strconv.Atoi(periodStr)
				if err != nil || periodMs <= 0 {
					m.errorMsg = "Invalid time period"
					return m, nil
				}
				waiting := NewSendP2pkhPeriodicallyWaiting(m.manager, m.user, addr, amt, time.Duration(periodMs)*time.Millisecond)
				return waiting, waiting.Init()
			}
			m.nextInput()
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyShiftTab, tea.KeyCtrlP:
			m.prevInput()
		case tea.KeyTab, tea.KeyCtrlN:
			m.nextInput()
		case tea.KeyDown:
			m.nextInput()
		case tea.KeyUp:
			m.prevInput()
		}
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
		if m.inputs[i].Focused() {
			m.focused = i
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *SendP2pkhPeriodicallyForm) View() string {
	var b strings.Builder
	b.WriteString("Enter recipient, satoshis, and time period:\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
	}
	continueButton := &BlurredButton
	if m.continueIsFocused() {
		continueButton = &FocusedButton
	}
	b.WriteString("\n\n" + continueButton.Render("Continue"))
	if m.errorMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.errorMsg))
	}
	return b.String()
}

func (m *SendP2pkhPeriodicallyForm) nextInput() {
	current := -1
	for i, in := range m.inputs {
		if in.Focused() {
			current = i
			break
		}
	}
	if current != -1 {
		m.inputs[current].Blur()
	}
	next := current + 1
	if next > len(m.inputs) {
		next = 0
	}
	if next < len(m.inputs) {
		m.inputs[next].Focus()
	}
	m.focused = next
}

func (m *SendP2pkhPeriodicallyForm) prevInput() {
	current := -1
	for i, in := range m.inputs {
		if in.Focused() {
			current = i
			break
		}
	}
	if current != -1 {
		m.inputs[current].Blur()
	}
	prev := current - 1
	if prev < 0 {
		prev = len(m.inputs)
	}
	if prev < len(m.inputs) {
		m.inputs[prev].Focus()
	}
	m.focused = prev
}

func (m *SendP2pkhPeriodicallyForm) continueIsFocused() bool { return m.focused == len(m.inputs) }
