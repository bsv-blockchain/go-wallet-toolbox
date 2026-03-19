package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
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
	items := make([]FocusItem, 0, 1+len(inputs)+1)
	items = append(items, FocusItem{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack})
	for i := range inputs {
		items = append(items, FocusItem{
			Type:  ElementInput,
			Index: i,
			Label: inputs[i].Prompt,
		})
	}
	items = append(items, FocusItem{Type: ElementButton, Index: ButtonContinue, Label: fixtures.ButtonContinue})

	form.focus.SetItems(items)
	form.focus.current = 1
	form.updateInputFocus()

	return form
}

func (m *ListOutputsForm) updateInputFocus() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}

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
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyEnter:
			current := m.focus.CurrentItem()
			if current.Type == ElementButton {
				return m.handleEnter()
			} else {
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
			selectAction := NewSelectAction(m.manager, m.user)
			return selectAction, selectAction.Init()
		case ButtonContinue:
			return m.processContinue()
		}
	}
	return m, nil
}

func (m *ListOutputsForm) processContinue() (tea.Model, tea.Cmd) {
	config, err := m.validateAndParseInputs()
	if err != nil {
		m.errorMsg = err.Error()
		return m, nil
	}

	waiting := NewListOutputsWaiting(m.manager, m.user, config.limit, config.offset, config.includeLabels, config.basket)
	return waiting, waiting.Init()
}

type outputsConfig struct {
	limit         uint32
	offset        uint32
	basket        string
	includeLabels bool
}

func (m *ListOutputsForm) validateAndParseInputs() (*outputsConfig, error) {
	config := &outputsConfig{
		limit:         100,
		offset:        0,
		basket:        "default",
		includeLabels: true,
	}

	if err := m.parseLimit(config); err != nil {
		return nil, err
	}

	if err := m.parseOffset(config); err != nil {
		return nil, err
	}

	m.parseBasket(config)

	if err := m.parseIncludeLabels(config); err != nil {
		return nil, err
	}

	return config, nil
}

func (m *ListOutputsForm) parseLimit(config *outputsConfig) error {
	v := strings.TrimSpace(m.inputs[0].Value())
	if v == "" {
		return nil // Use default
	}

	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid limit")
	}

	config.limit = uint32(n)
	return nil
}

func (m *ListOutputsForm) parseOffset(config *outputsConfig) error {
	v := strings.TrimSpace(m.inputs[1].Value())
	if v == "" {
		return nil // Use default
	}

	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid offset")
	}

	config.offset = uint32(n)
	return nil
}

func (m *ListOutputsForm) parseBasket(config *outputsConfig) {
	v := strings.TrimSpace(m.inputs[2].Value())
	if v != "" {
		config.basket = v
	}
}

func (m *ListOutputsForm) parseIncludeLabels(config *outputsConfig) error {
	v := strings.TrimSpace(strings.ToLower(m.inputs[3].Value()))
	if v == "" {
		return nil
	}

	switch v {
	case "true", "t", "y", "yes":
		config.includeLabels = true
	case "false", "f", "n", "no":
		config.includeLabels = false
	default:
		return fmt.Errorf("invalid include labels (true/false)")
	}

	return nil
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
