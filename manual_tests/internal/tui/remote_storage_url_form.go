package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const Placeholder = "http://localhost:8100"

type RemoteStorageURLForm struct {
	manager   ManagerInterface
	textInput textinput.Model
	focus     *FocusManager
	err       error
}

func NewRemoteStorageURLForm(manager ManagerInterface) *RemoteStorageURLForm {
	ti := textinput.New()
	ti.Placeholder = Placeholder
	ti.CharLimit = 256
	ti.Width = 50

	form := &RemoteStorageURLForm{
		manager:   manager,
		textInput: ti,
		focus:     NewFocusManager(),
		err:       nil,
	}

	// Set up focus items: URL input, Continue, Back
	form.focus.SetItems([]FocusItem{
		{Type: ElementInput, Index: 0, Label: "Server URL"},
		{Type: ElementButton, Index: ButtonContinue, Label: "Connect"},
		{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack},
	})

	form.updateInputFocus()
	return form
}

func (m *RemoteStorageURLForm) updateInputFocus() {
	m.textInput.Blur()

	current := m.focus.CurrentItem()
	if current.Type == ElementInput {
		m.textInput.Focus()
	}
}

func (m *RemoteStorageURLForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m *RemoteStorageURLForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			return m.handleEnter()

		case tea.KeyShiftTab, tea.KeyCtrlP, tea.KeyUp:
			m.focus.Previous()
			m.updateInputFocus()
		case tea.KeyTab, tea.KeyCtrlN, tea.KeyDown:
			m.focus.Next()
			m.updateInputFocus()
		}
	}

	current := m.focus.CurrentItem()
	var cmd tea.Cmd
	if current.Type == ElementInput {
		m.textInput, cmd = m.textInput.Update(msg)
	} else {
		cmd = nil
	}

	return m, cmd
}

func (m *RemoteStorageURLForm) handleEnter() (tea.Model, tea.Cmd) {
	current := m.focus.CurrentItem()
	if current.Type == ElementButton {
		switch current.Index {
		case ButtonContinue:
			return m.connectToRemote()
		case ButtonBack:
			selectAction := NewSelectStorage(m.manager)
			return selectAction, selectAction.Init()
		}
	} else {
		url := m.textInput.Value()
		if url == "" {
			url = Placeholder
			m.textInput.SetValue(url)
			return m, nil
		}
		m.focus.Next()
		m.updateInputFocus()
	}
	return m, nil
}

func (m *RemoteStorageURLForm) connectToRemote() (tea.Model, tea.Cmd) {
	url := m.textInput.Value()
	if managerWithURL, ok := m.manager.(interface{ SetRemoteStorageURL(string) error }); ok {
		if err := managerWithURL.SetRemoteStorageURL(url); err != nil {
			m.err = err
			return m, nil
		}
	} else {
		m.err = fmt.Errorf("manager doesn't support remote storage URL setting")
		return m, nil
	}

	spinner := NewModelSpinner("✅ Remote storage connected successfully! Loading wallets...", Wait(m.manager.Ctx(), 2*time.Second), func() tea.Model {
		return NewSelectWallet(m.manager)
	})
	return spinner, spinner.Init()
}

func (m *RemoteStorageURLForm) View() string {
	var b strings.Builder

	b.WriteString("Enter Remote Storage URL:\n\n")
	b.WriteString(m.textInput.View() + "\n\n")

	connectStyle := &fixtures.BlurredButton
	if m.focus.IsButtonFocused(ButtonContinue) {
		connectStyle = &fixtures.FocusedButton
	}
	b.WriteString(connectStyle.Render("Connect") + "\n")

	backStyle := &fixtures.BlurredButton
	if m.focus.IsButtonFocused(ButtonBack) {
		backStyle = &fixtures.FocusedButton
	}
	b.WriteString(backStyle.Render(fixtures.ButtonBack))

	if m.err != nil {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("❌ Error: "+m.err.Error()))
	}

	return b.String()
}
