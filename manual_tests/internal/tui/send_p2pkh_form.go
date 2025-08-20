package tui

import (
	"strconv"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SendP2pkhForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	inputs   []textinput.Model
	focused  int
	errorMsg string
}

func NewSendP2pkhForm(manager ManagerInterface, user *fixtures.UserConfig) *SendP2pkhForm {
	inputs := make([]textinput.Model, 2)

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

	return &SendP2pkhForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focused: 0,
	}
}

func (m *SendP2pkhForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SendP2pkhForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				if amountStr == "" {
					m.errorMsg = "Satoshis amount is required"
					return m, nil
				}
				amt, err := strconv.ParseUint(amountStr, 10, 64)
				if err != nil || amt == 0 {
					m.errorMsg = "Invalid satoshis amount"
					return m, nil
				}
				waitingView := NewSendP2pkhWaiting(m.manager, m.user, addr, amt)
				return waitingView, waitingView.Init()
			} else {
				m.nextInput()
			}
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

func (m *SendP2pkhForm) View() string {
	var b strings.Builder

	b.WriteString("Enter recipient address and satoshis to send:\n\n")

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

func (m *SendP2pkhForm) nextInput() {
	currentFocus := -1
	for i, input := range m.inputs {
		if input.Focused() {
			currentFocus = i
			break
		}
	}

	if currentFocus != -1 {
		m.inputs[currentFocus].Blur()
	}

	nextFocus := currentFocus + 1
	if nextFocus > len(m.inputs) {
		nextFocus = 0
	}

	if nextFocus < len(m.inputs) {
		m.inputs[nextFocus].Focus()
	}
	m.focused = nextFocus
}

func (m *SendP2pkhForm) prevInput() {
	currentFocus := -1
	for i, input := range m.inputs {
		if input.Focused() {
			currentFocus = i
			break
		}
	}

	if currentFocus != -1 {
		m.inputs[currentFocus].Blur()
	}

	nextFocus := currentFocus - 1
	if nextFocus < 0 {
		nextFocus = len(m.inputs)
	}

	if nextFocus < len(m.inputs) {
		m.inputs[nextFocus].Focus()
	}
	m.focused = nextFocus
}

func (m *SendP2pkhForm) continueIsFocused() bool {
	return m.focused == len(m.inputs)
}
