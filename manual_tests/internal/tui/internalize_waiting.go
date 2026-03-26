package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type InternalizeWaiting struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	txInput  textinput.Model
	focus    *FocusManager
	selected internalizeData
}

func NewInternalizeWaiting(manager ManagerInterface, user *fixtures.UserConfig, selected internalizeData) *InternalizeWaiting {
	txInput := textinput.New()
	txInput.Placeholder = "Transaction ID to internalize"
	txInput.CharLimit = 64
	txInput.Width = 70
	txInput.Prompt = ""
	txInput.Validate = validateTxID

	form := &InternalizeWaiting{
		manager:  manager,
		user:     user,
		txInput:  txInput,
		selected: selected,
		focus:    NewFocusManager(),
	}

	form.focus.SetItems([]FocusItem{
		{Type: ElementInput, Index: 0, Label: "Transaction ID"},
		{Type: ElementButton, Index: ButtonContinue, Label: fixtures.ButtonContinue},
		{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack},
	})

	form.updateInputFocus()
	return form
}

func (m *InternalizeWaiting) updateInputFocus() {
	// Clear input focus
	m.txInput.Blur()

	// Set focus on input if applicable
	current := m.focus.CurrentItem()
	if current.Type == ElementInput {
		m.txInput.Focus()
	}
}

func (m *InternalizeWaiting) Init() tea.Cmd {
	return textinput.Blink
}

func (m *InternalizeWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	var inputCmd tea.Cmd
	m.txInput, inputCmd = m.txInput.Update(msg)
	return m, inputCmd
}

func (m *InternalizeWaiting) handleEnter() (tea.Model, tea.Cmd) {
	current := m.focus.CurrentItem()
	if current.Type == ElementButton {
		switch current.Index {
		case ButtonContinue:
			return m.submit()
		case ButtonBack:
			selectAction := NewSelectAction(m.manager, m.user)
			return selectAction, selectAction.Init()
		}
	}
	return m, nil
}

func (m *InternalizeWaiting) View() string {
	instructions := ""
	if m.manager.GetBSVNetwork() == defs.NetworkTestnet {
		instructions = RenderTestnetFaucetInstructions(m.selected.address)
	}

	return fmt.Sprintf(` %s

%s
%s

%s
%s
`,
		instructions,
		inputStyle.Width(30).Render("New Transaction ID"),
		m.txInput.View(),
		func() string {
			style := navStyle
			if m.focus.IsButtonFocused(ButtonContinue) {
				style = navStyleFocused
			}
			return style.Render(fixtures.ButtonContinue)
		}(),
		func() string {
			style := navStyle
			if m.focus.IsButtonFocused(ButtonBack) {
				style = navStyleFocused
			}
			return style.Render(fixtures.ButtonBack)
		}(),
	)
}

func (m *InternalizeWaiting) submit() (tea.Model, tea.Cmd) {
	stopChan := make(chan struct{})
	var internalizeErr error
	var summary fixtures.Summary
	go func() {
		keyID := brc29.KeyID{
			DerivationPrefix: m.selected.derivationPrefix,
			DerivationSuffix: m.selected.derivationSuffix,
		}
		summary, internalizeErr = m.manager.InternalizeTxID(m.txInput.Value(), *m.user, keyID, m.selected.address)

		stopChan <- struct{}{}
	}()

	goToResultView := func() tea.Model {
		mode := ResultViewSuccess
		resultMsg := "Transaction internalized successfully!"
		if internalizeErr != nil {
			mode = ResultViewError
			resultMsg = fmt.Sprintf("Failed to internalize transaction: %s", internalizeErr.Error())
		}

		goToSelectAction := func() tea.Model {
			return NewSelectAction(m.manager, m.user)
		}

		return NewResultView(m.manager, resultMsg, mode, goToSelectAction, summary)
	}

	spinner := NewModelSpinner("Internalizing transaction...", stopChan, goToResultView)
	return spinner, spinner.Init()
}

func validateTxID(input string) error {
	err := primitives.TXIDHexString(input).Validate()
	if err != nil {
		return fmt.Errorf("invalid transaction ID: %w", err)
	}
	return nil
}
