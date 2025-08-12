package tui

import (
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-softwarelab/common/pkg/to"
)

type InternalizeWaiting struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	txInput  textinput.Model
	focused  int
	selected internalizeData
}

func NewInternalizeWaiting(manager ManagerInterface, user *fixtures.UserConfig, selected internalizeData) *InternalizeWaiting {
	txInput := textinput.New()
	txInput.Placeholder = "Transaction ID to internalize"
	txInput.CharLimit = 64
	txInput.Width = 70
	txInput.Prompt = ""
	txInput.Validate = validateTxID
	txInput.Focus()

	model := &InternalizeWaiting{
		manager:  manager,
		user:     user,
		txInput:  txInput,
		selected: selected,
	}

	return model
}

func (m *InternalizeWaiting) Init() tea.Cmd {
	return textinput.Blink
}

func (m *InternalizeWaiting) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.continueIsFocused() {
				return m.submit()
			} else {
				m.nextFocus()
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyShiftTab, tea.KeyCtrlP:
			m.prevFocus()
		case tea.KeyTab, tea.KeyCtrlN:
			m.nextFocus()
		case tea.KeyDown:
			m.nextFocus()
		case tea.KeyUp:
			m.prevFocus()
		}

		if m.focused == 0 {
			m.txInput.Focus()
		} else {
			m.txInput.Blur()
		}
	}

	var inputCmd tea.Cmd
	m.txInput, inputCmd = m.txInput.Update(msg)
	return m, inputCmd
}

func (m *InternalizeWaiting) View() string {
	// TODO: Handle invalid transaction ID input

	instructions := ""
	if m.manager.GetBSVNetwork() == defs.NetworkTestnet {
		instructions = RenderTestnetFaucetInstructions(m.selected.address)
	}

	return fmt.Sprintf(` %s

%s
%s

%s
`,
		instructions,
		inputStyle.Width(30).Render("New Transaction ID"),
		m.txInput.View(),
		to.IfThen(m.continueIsFocused(), continueStyleFocused).ElseThen(continueStyle).
			Render("Continue ->"),
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

// nextFocus focuses the next input field
func (m *InternalizeWaiting) nextFocus() {
	m.focused = (m.focused + 1) % 2
}

// prevFocus focuses the previous input field
func (m *InternalizeWaiting) prevFocus() {
	m.focused--
	if m.focused < 0 {
		m.focused = 1
	}
}

func (m *InternalizeWaiting) continueIsFocused() bool {
	return m.focused == 1
}

func validateTxID(input string) error {
	err := primitives.TXIDHexString(input).Validate()
	if err != nil {
		return fmt.Errorf("invalid transaction ID: %w", err)
	}

	return nil
}
