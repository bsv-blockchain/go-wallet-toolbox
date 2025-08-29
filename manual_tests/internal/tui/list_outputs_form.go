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
	focus    *FocusManager
	errorMsg string
}

func NewListOutputsForm(manager ManagerInterface, user *fixtures.UserConfig) *ListOutputsForm {
	inputs := make([]textinput.Model, 4)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = ""
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

	form := &ListOutputsForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focus:   NewFocusManager(),
	}

	// Set up focus items: Back, all inputs, Continue
	items := []FocusItem{
		{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack},
	}
	for i := range inputs {
		items = append(items, FocusItem{
			Type:  ElementInput,
			Index: i,
			Label: inputs[i].Prompt,
		})
	}
	items = append(items, FocusItem{Type: ElementButton, Index: ButtonContinue, Label: fixtures.ButtonContinue})

	form.focus.SetItems(items)
	// Start with first input focused
	form.focus.current = 1
	form.updateInputFocus()

	return form
}

func (m *ListOutputsForm) updateInputFocus() {
	// Clear all input focus
	for i := range m.inputs {
		m.inputs[i].Blur()
	}

	// Set focus on current input if applicable
	current := m.focus.CurrentItem()
	if current.Type == ElementInput && current.Index < len(m.inputs) {
		m.inputs[current.Index].Focus()
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
			current := m.focus.CurrentItem()
			if current.Type == ElementButton {
				return m.handleEnter()
			} else {
				// For inputs, Enter moves to next field
				m.focus.Next()
				m.updateInputFocus()
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyShiftTab, tea.KeyCtrlP, tea.KeyUp:
			m.focus.Previous()
			m.updateInputFocus()
		case tea.KeyTab, tea.KeyCtrlN, tea.KeyDown:
			m.focus.Next()
			m.updateInputFocus()
		}
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *ListOutputsForm) handleEnter() (tea.Model, tea.Cmd) {
	current := m.focus.CurrentItem()
	if current.Type == ElementButton {
		switch current.Index {
		case ButtonBack:
			// Go back to action selection
			selectAction := NewSelectAction(m.manager, m.user)
			return selectAction, selectAction.Init()
		case ButtonContinue:
			return m.processContinue()
		}
	}
	return m, nil
}

func (m *ListOutputsForm) processContinue() (tea.Model, tea.Cmd) {
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
}

func (m *ListOutputsForm) View() string {
	var b strings.Builder

	b.WriteString("Configure output listing:\n")

	// Back button
	backStyle := &fixtures.BlurredButton
	if m.focus.IsButtonFocused(ButtonBack) {
		backStyle = &fixtures.FocusedButton
	}
	b.WriteString(backStyle.Render(fixtures.ButtonBack) + "\n")

	// Input fields
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}

	// Continue button
	continueStyle := &fixtures.BlurredButton
	if m.focus.IsButtonFocused(ButtonContinue) {
		continueStyle = &fixtures.FocusedButton
	}
	b.WriteString(continueStyle.Render(fixtures.ButtonContinue))

	if m.errorMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.errorMsg))
	}

	return b.String()
}
