package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	FocusedButton = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	BlurredButton = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

const (
	ButtonData         = iota // Send Data button
	ButtonP2PKH               // Send P2PKH button
	ButtonBack                // Back button
	ButtonContinue            // Continue button
	ButtonSendOnce            // Send Once button
	ButtonSendPeriodic        // Send Periodically button
)

// TransactionType represents the type of transaction to send
type TransactionType int

const (
	TransactionTypeData TransactionType = iota
	TransactionTypeP2PKH
)

// SendConfig holds the unified configuration for sending transactions
type SendConfig struct {
	transactionType TransactionType
	// Data transaction fields
	data string
	// P2PKH transaction fields
	address string
	amount  uint64
	// Common fields
	isPeriodic bool
	period     time.Duration
}

// SendFormBuilder provides a builder pattern for creating SendForm
type SendFormBuilder struct {
	manager ManagerInterface
	user    *fixtures.UserConfig
}

// NewSendFormBuilder creates a new builder instance
func NewSendFormBuilder(manager ManagerInterface, user *fixtures.UserConfig) *SendFormBuilder {
	return &SendFormBuilder{
		manager: manager,
		user:    user,
	}
}

// Build creates the SendForm
func (b *SendFormBuilder) Build() *SendForm {
	return &SendForm{
		manager: b.manager,
		user:    b.user,
		config:  &SendConfig{},
		step:    StepTransactionType,
		focus:   NewFocusManager(),
	}
}

// SendFormStep represents the current step in the form
type SendFormStep int

const (
	StepTransactionType SendFormStep = iota
	StepTransactionDetails
	StepPeriodicChoice
	StepPeriodConfig
)

// FocusableElement represents different types of focusable elements
type FocusableElement int

const (
	ElementInput FocusableElement = iota
	ElementButton
)

// FocusItem represents a single focusable item
type FocusItem struct {
	Type  FocusableElement
	Index int
	Label string
}

// FocusManager handles all focus-related logic
type FocusManager struct {
	items   []FocusItem
	current int
}

func NewFocusManager() *FocusManager {
	return &FocusManager{
		items:   []FocusItem{},
		current: 0,
	}
}

func (fm *FocusManager) SetItems(items []FocusItem) {
	fm.items = items
	fm.current = 0
}

func (fm *FocusManager) Next() {
	if len(fm.items) > 0 {
		fm.current = (fm.current + 1) % len(fm.items)
	}
}

func (fm *FocusManager) Previous() {
	if len(fm.items) > 0 {
		fm.current--
		if fm.current < 0 {
			fm.current = len(fm.items) - 1
		}
	}
}

func (fm *FocusManager) CurrentItem() FocusItem {
	if len(fm.items) == 0 {
		return FocusItem{}
	}
	return fm.items[fm.current]
}

func (fm *FocusManager) IsButtonFocused(buttonType int) bool {
	current := fm.CurrentItem()
	return current.Type == ElementButton && current.Index == buttonType
}

func (fm *FocusManager) IsInputFocused(inputIndex int) bool {
	current := fm.CurrentItem()
	return current.Type == ElementInput && current.Index == inputIndex
}

type SendForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	config   *SendConfig
	inputs   []textinput.Model
	step     SendFormStep
	focus    *FocusManager
	errorMsg string
}

func (m *SendForm) Init() tea.Cmd {
	m.initCurrentStep()
	return textinput.Blink
}

func (m *SendForm) initCurrentStep() {
	switch m.step {
	case StepTransactionType:
		m.inputs = make([]textinput.Model, 0)
		m.focus.SetItems([]FocusItem{
			{Type: ElementButton, Index: ButtonData, Label: "Send Data"},
			{Type: ElementButton, Index: ButtonP2PKH, Label: "Send P2PKH"},
			{Type: ElementButton, Index: ButtonBack, Label: "Back"},
		})

	case StepTransactionDetails:
		if m.config.transactionType == TransactionTypeData {
			m.inputs = make([]textinput.Model, 1)
			m.inputs[0] = textinput.New()
			m.inputs[0].Placeholder = "Data to send"
			m.inputs[0].CharLimit = 256
			m.inputs[0].Width = 50
			m.inputs[0].Prompt = ""
		} else {
			m.inputs = make([]textinput.Model, 2)

			m.inputs[0] = textinput.New()
			m.inputs[0].Placeholder = "Recipient address (P2PKH)"
			m.inputs[0].CharLimit = 128
			m.inputs[0].Width = 50
			m.inputs[0].Prompt = ""

			m.inputs[1] = textinput.New()
			m.inputs[1].Placeholder = "Satoshis to send"
			m.inputs[1].CharLimit = 20
			m.inputs[1].Width = 50
			m.inputs[1].Prompt = ""
		}

		items := []FocusItem{}
		for i := range m.inputs {
			items = append(items, FocusItem{
				Type:  ElementInput,
				Index: i,
				Label: fmt.Sprintf("Input %d", i),
			})
		}
		items = append(items, FocusItem{Type: ElementButton, Index: ButtonContinue, Label: "Continue"})
		items = append(items, FocusItem{Type: ElementButton, Index: ButtonBack, Label: "Back"})
		m.focus.SetItems(items)

	case StepPeriodicChoice:
		m.inputs = make([]textinput.Model, 0)
		m.focus.SetItems([]FocusItem{
			{Type: ElementButton, Index: ButtonSendOnce, Label: "Send Once"},
			{Type: ElementButton, Index: ButtonSendPeriodic, Label: "Send Periodically"},
			{Type: ElementButton, Index: ButtonBack, Label: "Back"},
		})

	case StepPeriodConfig:
		m.inputs = make([]textinput.Model, 1)
		m.inputs[0] = textinput.New()
		m.inputs[0].Placeholder = "Time period (ms)"
		m.inputs[0].CharLimit = 10
		m.inputs[0].Width = 50
		m.inputs[0].Prompt = ""

		m.focus.SetItems([]FocusItem{
			{Type: ElementInput, Index: 0, Label: "Period Input"},
			{Type: ElementButton, Index: ButtonContinue, Label: "Continue"},
			{Type: ElementButton, Index: ButtonBack, Label: "Back"},
		})
	}
	m.updateInputFocus()
}

func (m *SendForm) updateInputFocus() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}

	current := m.focus.CurrentItem()
	if current.Type == ElementInput && current.Index < len(m.inputs) {
		m.inputs[current.Index].Focus()
	}
}

func (m *SendForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
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

func (m *SendForm) handleEnter() (tea.Model, tea.Cmd) {
	current := m.focus.CurrentItem()

	switch m.step {
	case StepTransactionType:
		if current.Type == ElementButton {
			switch current.Index {
			case ButtonData:
				m.config.transactionType = TransactionTypeData
			case ButtonP2PKH:
				m.config.transactionType = TransactionTypeP2PKH
			case ButtonBack:
				selectAction := NewSelectAction(m.manager, m.user)
				return selectAction, selectAction.Init()
			}
			if current.Index != ButtonBack {
				m.step = StepTransactionDetails
				m.initCurrentStep()
				m.errorMsg = ""
			}
		}

	case StepTransactionDetails:
		if current.Type == ElementButton {
			switch current.Index {
			case ButtonBack:
				m.step = StepTransactionType
				m.initCurrentStep()
				m.errorMsg = ""
			case ButtonContinue:
				if m.config.transactionType == TransactionTypeData {
					data := strings.TrimSpace(m.inputs[0].Value())
					if data == "" {
						m.errorMsg = "Data is required"
						return m, nil
					}
					m.config.data = data
				} else {
					addr := strings.TrimSpace(m.inputs[0].Value())
					if addr == "" {
						m.errorMsg = "Recipient address is required"
						return m, nil
					}

					amountStr := strings.TrimSpace(m.inputs[1].Value())
					if amountStr == "" {
						m.errorMsg = "Satoshis amount is required"
						return m, nil
					}

					amt, err := strconv.ParseUint(amountStr, 10, 64)
					if err != nil || amt == 0 {
						m.errorMsg = "Invalid satoshis amount"
						return m, nil
					}

					m.config.address = addr
					m.config.amount = amt
				}

				m.step = StepPeriodicChoice
				m.initCurrentStep()
				m.errorMsg = ""
			}
		}

	case StepPeriodicChoice:
		if current.Type == ElementButton {
			switch current.Index {
			case ButtonBack:
				m.step = StepTransactionDetails
				m.initCurrentStep()
				m.errorMsg = ""
			case ButtonSendOnce:
				m.config.isPeriodic = false
				return m.executeAction()
			case ButtonSendPeriodic:
				m.config.isPeriodic = true
				m.step = StepPeriodConfig
				m.initCurrentStep()
				m.errorMsg = ""
			}
		}

	case StepPeriodConfig:
		if current.Type == ElementButton {
			switch current.Index {
			case ButtonBack:
				m.step = StepPeriodicChoice
				m.initCurrentStep()
				m.errorMsg = ""
			case ButtonContinue:
				periodStr := strings.TrimSpace(m.inputs[0].Value())
				periodMs, err := strconv.Atoi(periodStr)
				if err != nil || periodMs <= 0 {
					m.errorMsg = "Invalid time period"
					return m, nil
				}
				m.config.period = time.Duration(periodMs) * time.Millisecond
				return m.executeAction()
			}
		}
	}

	return m, nil
}

func (m *SendForm) executeAction() (tea.Model, tea.Cmd) {
	if m.config.transactionType == TransactionTypeData {
		if m.config.isPeriodic {
			waitingView := NewSendDataPeriodicallyWaiting(m.manager, m.user, m.config.data, m.config.period)
			return waitingView, waitingView.Init()
		} else {
			waitingView := NewSendDataWaiting(m.manager, m.user, m.config.data)
			return waitingView, waitingView.Init()
		}
	} else {
		if m.config.isPeriodic {
			waitingView := NewSendP2pkhPeriodicallyWaiting(m.manager, m.user, m.config.address, m.config.amount, m.config.period)
			return waitingView, waitingView.Init()
		} else {
			waitingView := NewSendP2pkhWaiting(m.manager, m.user, m.config.address, m.config.amount)
			return waitingView, waitingView.Init()
		}
	}
}
func (m *SendForm) View() string {
	var b strings.Builder

	switch m.step {
	case StepTransactionType:
		b.WriteString("What would you like to send?\n\n")

		buttons := []struct {
			buttonType int
			text       string
		}{
			{ButtonData, "📄 Send Data"},
			{ButtonP2PKH, "💰 Send Payment (P2PKH)"},
			{ButtonBack, "← Back to Menu"},
		}

		for _, btn := range buttons {
			style := &BlurredButton
			if m.focus.IsButtonFocused(btn.buttonType) {
				style = &FocusedButton
			}
			b.WriteString(style.Render(btn.text) + "\n")
		}

	case StepTransactionDetails:
		if m.config.transactionType == TransactionTypeData {
			b.WriteString("Enter data to send:\n")
		} else {
			b.WriteString("Enter payment details:\n")
		}

		backStyle := &BlurredButton
		if m.focus.IsButtonFocused(ButtonBack) {
			backStyle = &FocusedButton
		}
		b.WriteString(backStyle.Render("← Back") + "\n")

		for i := range m.inputs {
			b.WriteString(m.inputs[i].View())
			if i < len(m.inputs)-1 {
				b.WriteRune('\n')
			}
		}

		continueStyle := &BlurredButton
		if m.focus.IsButtonFocused(ButtonContinue) {
			continueStyle = &FocusedButton
		}
		b.WriteString("\n" + continueStyle.Render("Continue →"))

	case StepPeriodicChoice:
		b.WriteString("Choose sending method:\n\n")

		if m.config.transactionType == TransactionTypeData {
			b.WriteString(fmt.Sprintf("Data: %s\n\n", m.config.data))
		} else {
			b.WriteString(fmt.Sprintf("Recipient: %s\n", m.config.address))
			b.WriteString(fmt.Sprintf("Amount: %d satoshis\n\n", m.config.amount))
		}

		buttons := []struct {
			buttonType int
			text       string
		}{
			{ButtonSendOnce, "🎯 Send Once"},
			{ButtonSendPeriodic, "🔄 Send Periodically"},
			{ButtonBack, "← Back"},
		}

		for _, btn := range buttons {
			style := &BlurredButton
			if m.focus.IsButtonFocused(btn.buttonType) {
				style = &FocusedButton
			}
			b.WriteString(style.Render(btn.text) + "\n")
		}

	case StepPeriodConfig:
		b.WriteString("Configure periodic sending:\n")

		if m.config.transactionType == TransactionTypeData {
			b.WriteString(fmt.Sprintf("Data: %s\n", m.config.data))
		} else {
			b.WriteString(fmt.Sprintf("Recipient: %s\n", m.config.address))
			b.WriteString(fmt.Sprintf("Amount: %d satoshis\n", m.config.amount))
		}

		backStyle := &BlurredButton
		if m.focus.IsButtonFocused(ButtonBack) {
			backStyle = &FocusedButton
		}
		b.WriteString(backStyle.Render("← Back") + "\n")

		for i := range m.inputs {
			b.WriteString(m.inputs[i].View())
			if i < len(m.inputs)-1 {
				b.WriteRune('\n')
			}
		}

		continueStyle := &BlurredButton
		if m.focus.IsButtonFocused(ButtonContinue) {
			continueStyle = &FocusedButton
		}
		b.WriteString("\n" + continueStyle.Render("🚀 Start Periodic Sending"))
	}

	if m.errorMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.errorMsg))
	}

	return b.String()
}

// NewSendForm creates a new unified SendForm using the builder pattern
func NewSendForm(manager ManagerInterface, user *fixtures.UserConfig) *SendForm {
	return NewSendFormBuilder(manager, user).Build()
}
