package tui

import (
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	FocusedButton = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	BlurredButton = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type SendDataForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	inputs   []textinput.Model
	focused  int
	errorMsg string
}

func NewSendDataForm(manager ManagerInterface, user *fixtures.UserConfig) *SendDataForm {
	inputs := make([]textinput.Model, 1)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Data to send"
	inputs[0].Focus()
	inputs[0].CharLimit = 256
	inputs[0].Width = 50
	inputs[0].Prompt = ""

	return &SendDataForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focused: 0,
	}
}

func (m *SendDataForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SendDataForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.continueIsFocused() {
				waitingView := NewSendDataWaiting(m.manager, m.user, m.inputs[0].Value())
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
	}

	return m, tea.Batch(cmds...)
}

func (m *SendDataForm) View() string {
	var b strings.Builder

	b.WriteString("Enter data to send:\n\n")

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

func (m *SendDataForm) nextInput() {
	m.focused = (m.focused + 1) % (len(m.inputs) + 1)
}

func (m *SendDataForm) prevInput() {
	m.focused--
	if m.focused < 0 {
		m.focused = len(m.inputs)
	}
}

func (m *SendDataForm) continueIsFocused() bool {
	return m.focused == len(m.inputs)
}
