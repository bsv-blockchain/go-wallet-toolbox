package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type NoSendSendWithForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	inputs   []textinput.Model
	focus    *FocusManager
	errorMsg string
}

func NewNoSendSendWithForm(manager ManagerInterface, user *fixtures.UserConfig) *NoSendSendWithForm {
	return &NoSendSendWithForm{
		manager: manager,
		user:    user,
		focus:   NewFocusManager(),
	}
}

func (m *NoSendSendWithForm) Init() tea.Cmd {
	m.inputs = make([]textinput.Model, 2)

	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "Number of transactions"
	m.inputs[0].CharLimit = 10
	m.inputs[0].Width = 50
	m.inputs[0].Prompt = ""

	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "Data prefix for OP_RETURN"
	m.inputs[1].CharLimit = 50
	m.inputs[1].Width = 50
	m.inputs[1].Prompt = ""

	items := make([]FocusItem, 0, 1+len(m.inputs)+1)
	items = append(items, FocusItem{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack})
	for i := range m.inputs {
		items = append(items, FocusItem{
			Type:  ElementInput,
			Index: i,
			Label: fmt.Sprintf("Input %d", i),
		})
	}
	items = append(items, FocusItem{Type: ElementButton, Index: ButtonContinue, Label: fixtures.ButtonContinue})
	m.focus.SetItems(items)

	m.focus.current = 1
	m.updateInputFocus()

	return textinput.Blink
}

func (m *NoSendSendWithForm) updateInputFocus() {
	for i := range m.inputs {
		if m.focus.CurrentItem().Type == ElementInput && m.focus.CurrentItem().Index == i {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *NoSendSendWithForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab, tea.KeyDown:
			m.focus.Next()
			m.updateInputFocus()
			return m, nil
		case tea.KeyShiftTab, tea.KeyUp:
			m.focus.Previous()
			m.updateInputFocus()
			return m, nil
		case tea.KeyEnter:
			current := m.focus.CurrentItem()
			if current.Type == ElementButton {
				return m.handleEnter()
			} else {
				m.focus.Next()
				m.updateInputFocus()
			}
		}
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *NoSendSendWithForm) handleEnter() (tea.Model, tea.Cmd) {
	current := m.focus.CurrentItem()
	if current.Type != ElementButton {
		return m, nil
	}

	switch current.Index {
	case ButtonBack:
		selectAction := NewSelectAction(m.manager, m.user)
		return selectAction, selectAction.Init()
	case ButtonContinue:
		return m.handleSubmit()
	}

	return m, nil
}

func (m *NoSendSendWithForm) handleSubmit() (tea.Model, tea.Cmd) {
	countStr := strings.TrimSpace(m.inputs[0].Value())
	if countStr == "" {
		m.errorMsg = "Number of transactions is required"
		return m, nil
	}

	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 {
		m.errorMsg = "Number of transactions must be at least 1"
		return m, nil
	}

	prefix := strings.TrimSpace(m.inputs[1].Value())
	if prefix == "" {
		m.errorMsg = "Data prefix is required"
		return m, nil
	}

	waitingModel := NewNoSendSendWithWaiting(m.manager, m.user, count, prefix)
	return waitingModel, waitingModel.Init()
}

func (m *NoSendSendWithForm) View() string {
	var b strings.Builder

	b.WriteString("NoSend/SendWith Transaction Test\n")
	b.WriteString("This will create multiple noSend transactions and then broadcast them with sendWith.\n\n")

	backStyle := &fixtures.BlurredButton
	if m.focus.IsButtonFocused(ButtonBack) {
		backStyle = &fixtures.FocusedButton
	}
	b.WriteString(backStyle.Render(fixtures.ButtonBack) + "\n\n")

	labels := []string{
		"Number of transactions (min 1):",
		"Data prefix for OP_RETURN:",
	}

	for i := range m.inputs {
		b.WriteString(labels[i] + "\n")
		fmt.Fprintf(&b, "%s\n\n", m.inputs[i].View())
	}

	if m.errorMsg != "" {
		b.WriteString(errorStyle.Render(m.errorMsg) + "\n\n")
	}

	continueStyle := &fixtures.BlurredButton
	if m.focus.IsButtonFocused(ButtonContinue) {
		continueStyle = &fixtures.FocusedButton
	}
	b.WriteString(continueStyle.Render(fixtures.ButtonContinue))

	return b.String()
}
