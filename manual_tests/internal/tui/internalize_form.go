package tui

import (
	"encoding/base64"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-softwarelab/common/pkg/to"
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
	focused  int
	selected internalizeData
	errorMsg string
}

func NewInternalizeActionForm(manager ManagerInterface, user *fixtures.UserConfig) *InternalizeForm {
	inputs := make([]textinput.Model, 2)
	i := derivationPrefixIndex
	inputs[i] = textinput.New()
	inputs[i].Placeholder = "Base64 DerivationPrefix string"
	inputs[i].Focus()
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

	model := &InternalizeForm{
		manager: manager,
		user:    user,
		inputs:  inputs,
		focused: 0,
	}

	model.recalculateAddress()
	return model
}

func (m *InternalizeForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m *InternalizeForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			switch {
			case m.backIsFocused():
				selectAction := NewSelectAction(m.manager, m.user)
				return selectAction, selectAction.Init()
			case m.continueIsFocused():
				internalizeWaiting := NewInternalizeWaiting(m.manager, m.user, m.selected)
				return internalizeWaiting, internalizeWaiting.Init()
			case m.regenerateIsFocused():
				// Regenerate random derivation prefix and suffix
				m.regenerateRandomDerivation()
				return m, nil
			default:
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

		m.controlInputsFocus()
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
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
		m.inputs[derivationPrefixIndex].View(),
		inputStyle.Width(30).Render("Derivation Suffix"),
		m.inputs[derivationSuffixIndex].View(),
		calculatedAddressStyle.Width(30).Render("Calculated Address"),
		lipgloss.NewStyle().Foreground(hotBlue).Render(m.selected.address),
		to.If(m.errorMsg != "", func() string {
			return errorStyle.Render("Error: " + m.errorMsg + "\n")
		}).ElseThen(""),
		to.IfThen(m.regenerateIsFocused(), navStyleFocused).ElseThen(navStyle).
			Render("[ Regenerate Random Values ]"),
		to.IfThen(m.continueIsFocused(), navStyleFocused).ElseThen(navStyle).
			Render("Continue ->"),
		to.IfThen(m.backIsFocused(), navStyleFocused).ElseThen(navStyle).
			Render("<- Back"),
	)
}

func (m *InternalizeForm) nextInput() {
	m.focused = (m.focused + 1) % (len(m.inputs) + 3)
}

// prevInput focuses the previous input field
func (m *InternalizeForm) prevInput() {
	m.focused--
	if m.focused < 0 {
		m.focused = len(m.inputs) + 2
	}
}

func (m *InternalizeForm) controlInputsFocus() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	if m.focused < len(m.inputs) {
		m.inputs[m.focused].Focus()
	}
}

func (m *InternalizeForm) continueIsFocused() bool {
	return m.focused == continueButtonIndex
}

func (m *InternalizeForm) backIsFocused() bool {
	return m.focused == backButtonIndex
}

// regenerateIsFocused checks if the regenerate button is focused
func (m *InternalizeForm) regenerateIsFocused() bool {
	return m.focused == regenerateButtonIndex
}

func (m *InternalizeForm) recalculateAddress() {
	errorMsg := ""
	if err := m.inputs[derivationPrefixIndex].Err; err != nil {
		errorMsg = fmt.Sprintf("Error in Derivation Prefix: %v", err)
	}
	if err := m.inputs[derivationSuffixIndex].Err; err != nil {
		errorMsg = fmt.Sprintf("%s Error in Derivation Suffix: %v", errorMsg, err)
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

	m.inputs[derivationPrefixIndex].SetValue(prefixValue)
	m.inputs[derivationSuffixIndex].SetValue(suffixValue)

	m.recalculateAddress()
}
