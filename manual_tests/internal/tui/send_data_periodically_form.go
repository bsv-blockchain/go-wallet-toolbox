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

type SendDataPeriodicallyForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	inputs   []textinput.Model
	focused  int
	errorMsg string
}

func NewSendDataPeriodicallyForm(manager ManagerInterface, user *fixtures.UserConfig) *SendDataPeriodicallyForm {
	inputs := make([]textinput.Model, 2)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Data prefix"
	inputs[0].Focus()
	inputs[0].CharLimit = 256
	inputs[0].Width = 50
	inputs[0].Prompt = ""

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Time period (ms)"
	inputs[1].CharLimit = 10
	inputs[1].Width = 50
	inputs[1].Prompt = ""

	return &SendDataPeriodicallyForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focused: 0,
	}
}

func (m *SendDataPeriodicallyForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SendDataPeriodicallyForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.continueIsFocused() {
				period, err := strconv.Atoi(m.inputs[1].Value())
				if err != nil {
					m.errorMsg = "Invalid time period"
					return m, nil
				}
				waitingView := NewSendDataPeriodicallyWaiting(m.manager, m.user, m.inputs[0].Value(), time.Duration(period)*time.Millisecond)
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

func (m *SendDataPeriodicallyForm) View() string {
	var b strings.Builder

	b.WriteString("Enter data prefix and time period:\n\n")

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

func (m *SendDataPeriodicallyForm) nextInput() {
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

func (m *SendDataPeriodicallyForm) prevInput() {
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

func (m *SendDataPeriodicallyForm) continueIsFocused() bool {
	return m.focused == len(m.inputs)
}
