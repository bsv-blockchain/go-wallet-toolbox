package tui

import (
	"encoding/base64"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

const (
	ButtonRegenerate = 10
)

const (
	derivationPrefixIndex = iota
	derivationSuffixIndex
	regenerateButtonIndex = 2
	continueButtonIndex   = 3
	backButtonIndex       = 4
)

type internalizeData struct {
	address          string
	derivationPrefix string
	derivationSuffix string
}

type InternalizeForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	inputs   []textinput.Model
	focus    *FocusManager
	selected internalizeData
	errorMsg string
}

func NewInternalizeActionForm(manager ManagerInterface, user *fixtures.UserConfig) *InternalizeForm {
	inputs := make([]textinput.Model, 2)
	i := derivationPrefixIndex
	inputs[i] = textinput.New()
	inputs[i].Placeholder = "Base64 DerivationPrefix string"
	inputs[i].CharLimit = 40
	inputs[i].Width = 40
	inputs[i].Prompt = ""
	inputs[i].Validate = validateCanonicalBase64
	inputs[i].SetValue(fixtures.DefaultDerivationPrefix)
	i = derivationSuffixIndex
	inputs[i] = textinput.New()
	inputs[i].Placeholder = "Base64 DerivationSuffix string"
	inputs[i].CharLimit = 40
	inputs[i].Width = 40
	inputs[i].Prompt = ""
	inputs[i].Validate = validateCanonicalBase64
	inputs[i].SetValue(fixtures.DefaultDerivationSuffix)

	form := &InternalizeForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focus:   NewFocusManager(),
	}

	// Set up focus items: DerivationPrefix, DerivationSuffix, Regenerate, Continue, Back
	form.focus.SetItems([]FocusItem{
		{Type: ElementInput, Index: 0, Label: "Derivation Prefix"},
		{Type: ElementInput, Index: 1, Label: "Derivation Suffix"},
		{Type: ElementButton, Index: ButtonRegenerate, Label: "Regenerate"},
		{Type: ElementButton, Index: ButtonContinue, Label: fixtures.ButtonContinue},
		{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack},
	})

	form.updateInputFocus()
	form.recalculateAddress()
	return form
}

func (m *InternalizeForm) updateInputFocus() {
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

func (m *InternalizeForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m *InternalizeForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *InternalizeForm) handleEnter() (tea.Model, tea.Cmd) {
	current := m.focus.CurrentItem()
	if current.Type == ElementButton {
		switch current.Index {
		case ButtonBack:
			selectAction := NewSelectAction(m.manager, m.user)
			return selectAction, selectAction.Init()
		case ButtonContinue:
			internalizeWaiting := NewInternalizeWaiting(m.manager, m.user, m.selected)
			return internalizeWaiting, internalizeWaiting.Init()
		case ButtonRegenerate:
			m.regenerateRandomDerivation()
			return m, nil
		}
	}
	return m, nil
}

func (m *InternalizeForm) View() string {
	m.recalculateAddress()

	return fmt.Sprintf(
		`Provide derivation prefix and suffix to calculte an address on which you can receive funds.

 %s
 %s

 %s
 %s

 %s
 %s

 %s
 %s
 %s
 %s
`,
		inputStyle.Width(30).Render("Derivation Prefix"),
		m.inputs[0].View(),
		inputStyle.Width(30).Render("Derivation Suffix"),
		m.inputs[1].View(),
		calculatedAddressStyle.Width(30).Render("Calculated Address"),
		lipgloss.NewStyle().Foreground(hotBlue).Render(m.selected.address),
		func() string {
			if m.errorMsg != "" {
				return errorStyle.Render("Error: " + m.errorMsg + "\n")
			}
			return ""
		}(),
		func() string {
			style := navStyle
			if m.focus.IsButtonFocused(ButtonRegenerate) {
				style = navStyleFocused
			}
			return style.Render("[ Regenerate Random Values ]")
		}(),
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

func (m *InternalizeForm) recalculateAddress() {
	errorMsg := ""
	if err := m.inputs[0].Err; err != nil {
		errorMsg = fmt.Sprintf("Error in Derivation Prefix: %v", err)
	}
	if err := m.inputs[1].Err; err != nil {
		errorMsg = fmt.Sprintf("Error in Derivation Suffix: %v", err)
	}
	m.selected.derivationPrefix = m.inputs[derivationPrefixIndex].Value()
	m.selected.derivationSuffix = m.inputs[derivationSuffixIndex].Value()

	addressString := "-------"
	var err error
	if errorMsg == "" {
		addressString, err = calculateAddressForInternalize(
			m.selected.derivationPrefix,
			m.selected.derivationSuffix,
			m.user,
			m.manager.GetBSVNetwork(),
		)
		if err != nil {
			errorMsg = fmt.Sprintf("Failed to calculate address: %v", err)
		}
	}

	m.selected.address = addressString
	m.errorMsg = errorMsg
}

func (m *InternalizeForm) regenerateRandomDerivation() {
	r := randomizer.New()
	const length = 10
	prefixValue, err := r.Base64(length)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Failed to generate random prefix: %v", err)
		return
	}

	suffixValue, err := r.Base64(length)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Failed to generate random suffix: %v", err)
		return
	}
	m.inputs[0].SetValue(prefixValue)
	m.inputs[1].SetValue(suffixValue)
}

func calculateAddressForInternalize(derivationPrefix, derivationSuffix string, user *fixtures.UserConfig, bsvNetwork defs.BSVNetwork) (string, error) {
	anyonePriv, _ := sdk.AnyoneKey()
	keyID := brc29.KeyID{
		DerivationPrefix: derivationPrefix,
		DerivationSuffix: derivationSuffix,
	}

	networkOption := to.IfThen(bsvNetwork == defs.NetworkMainnet, brc29.WithMainNet()).ElseThen(brc29.WithTestNet())
	address, err := brc29.AddressForCounterparty(anyonePriv, keyID, user.PublicKey(), networkOption)
	if err != nil {
		return "", fmt.Errorf("failed to calculate address: %w", err)
	}

	return address.AddressString, nil
}

func validateCanonicalBase64(input string) error {
	bin, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return fmt.Errorf("invalid base64 string: %w", err)
	}

	backToBase64Str := base64.StdEncoding.EncodeToString(bin)
	if backToBase64Str != input {
		return fmt.Errorf("input is not canonical base64: %s", input)
	}

	return nil
}
