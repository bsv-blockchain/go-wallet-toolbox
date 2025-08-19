package tui

import (
	"strconv"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListOutputsForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	inputs   []textinput.Model
	focused  int
	errorMsg string
}

func NewListOutputsForm(manager ManagerInterface, user *fixtures.UserConfig) *ListOutputsForm {
	inputs := make([]textinput.Model, 4)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = ""
	inputs[0].Focus()
	inputs[0].CharLimit = 10
	inputs[0].Width = 30
	inputs[0].Prompt = "Limit: "
	inputs[0].SetValue("100")

	inputs[1] = textinput.New()
	inputs[1].Placeholder = ""
	inputs[1].CharLimit = 10
	inputs[1].Width = 30
	inputs[1].Prompt = "Offset: "
	inputs[1].SetValue("0")

	inputs[2] = textinput.New()
	inputs[2].Placeholder = ""
	inputs[2].CharLimit = 64
	inputs[2].Width = 30
	inputs[2].Prompt = "Basket: "
	inputs[2].SetValue("default")

	inputs[3] = textinput.New()
	inputs[3].Placeholder = ""
	inputs[3].CharLimit = 5
	inputs[3].Width = 30
	inputs[3].Prompt = "Include labels: "
	inputs[3].SetValue("true")

	return &ListOutputsForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focused: 0,
	}
}

func (m *ListOutputsForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ListOutputsForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.continueIsFocused() {
				limit := uint32(100)
				offset := uint32(0)
				basket := "default"
				includeLabels := true

				if v := strings.TrimSpace(m.inputs[0].Value()); v != "" {
					if n, err := strconv.ParseUint(v, 10, 32); err == nil {
						limit = uint32(n)
					} else {
						m.errorMsg = "Invalid limit"
						return m, nil
					}
				}

				if v := strings.TrimSpace(m.inputs[1].Value()); v != "" {
					if n, err := strconv.ParseUint(v, 10, 32); err == nil {
						offset = uint32(n)
					} else {
						m.errorMsg = "Invalid offset"
						return m, nil
					}
				}

				if v := strings.TrimSpace(m.inputs[2].Value()); v != "" {
					basket = v
				}

				if v := strings.TrimSpace(strings.ToLower(m.inputs[3].Value())); v != "" {
					if v == "true" || v == "t" || v == "y" || v == "yes" {
						includeLabels = true
					} else if v == "false" || v == "f" || v == "n" || v == "no" {
						includeLabels = false
					} else {
						m.errorMsg = "Invalid include labels (true/false)"
						return m, nil
					}
				}

				waiting := NewListOutputsWaiting(m.manager, m.user, limit, offset, includeLabels, basket)
				return waiting, waiting.Init()
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

func (m *ListOutputsForm) View() string {
	var b strings.Builder

	// Input prompts include labels and defaults inline

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

func (m *ListOutputsForm) nextInput() {
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

func (m *ListOutputsForm) prevInput() {
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

func (m *ListOutputsForm) continueIsFocused() bool {
	return m.focused == len(m.inputs)
}
