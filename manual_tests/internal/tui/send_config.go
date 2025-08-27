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
		focused: 0,
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

type SendForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	config   *SendConfig
	inputs   []textinput.Model
	step     SendFormStep
	focused  int
	errorMsg string
}

func (m *SendForm) Init() tea.Cmd {
	m.initCurrentStep()
	return textinput.Blink
}

func (m *SendForm) initCurrentStep() {
	switch m.step {
	case StepTransactionType, StepPeriodicChoice:
		m.inputs = make([]textinput.Model, 0)
		m.focused = 0

	case StepTransactionDetails:
		if m.config.transactionType == TransactionTypeData {
			// Data transaction: just one field
			m.inputs = make([]textinput.Model, 1)
			m.inputs[0] = textinput.New()
			m.inputs[0].Placeholder = "Data to send"
			m.inputs[0].Focus()
			m.inputs[0].CharLimit = 256
			m.inputs[0].Width = 50
			m.inputs[0].Prompt = ""
		} else {
			// P2PKH transaction: address and amount
			m.inputs = make([]textinput.Model, 2)

			m.inputs[0] = textinput.New()
			m.inputs[0].Placeholder = "Recipient address (P2PKH)"
			m.inputs[0].Focus()
			m.inputs[0].CharLimit = 128
			m.inputs[0].Width = 50
			m.inputs[0].Prompt = ""

			m.inputs[1] = textinput.New()
			m.inputs[1].Placeholder = "Satoshis to send"
			m.inputs[1].CharLimit = 20
			m.inputs[1].Width = 50
			m.inputs[1].Prompt = ""
		}
		m.focused = 0

	case StepPeriodConfig:
		m.inputs = make([]textinput.Model, 1)
		m.inputs[0] = textinput.New()
		m.inputs[0].Placeholder = "Time period (ms)"
		m.inputs[0].Focus()
		m.inputs[0].CharLimit = 10
		m.inputs[0].Width = 50
		m.inputs[0].Prompt = ""
		m.focused = 0
	}
}

func (m *SendForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m.handleEnter()
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
		if m.inputs[i].Focused() {
			m.focused = i
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *SendForm) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepTransactionType:
		if m.focused == 0 {
			m.config.transactionType = TransactionTypeData
		} else if m.focused == 1 {
			m.config.transactionType = TransactionTypeP2PKH
		}
		m.step = StepTransactionDetails
		m.initCurrentStep()
		m.errorMsg = ""
		return m, nil

	case StepTransactionDetails:
		if m.continueIsFocused() {
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
			return m, nil
		}
		m.nextInput()

	case StepPeriodicChoice:
		if m.focused == 0 {
			m.config.isPeriodic = false
			return m.executeAction()
		} else if m.focused == 1 {
			m.config.isPeriodic = true
			m.step = StepPeriodConfig
			m.initCurrentStep()
			m.errorMsg = ""
			return m, nil
		}

	case StepPeriodConfig:
		if m.continueIsFocused() {
			periodStr := strings.TrimSpace(m.inputs[0].Value())
			periodMs, err := strconv.Atoi(periodStr)
			if err != nil || periodMs <= 0 {
				m.errorMsg = "Invalid time period"
				return m, nil
			}
			m.config.period = time.Duration(periodMs) * time.Millisecond
			return m.executeAction()
		}
		m.nextInput()
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

		dataButton := &BlurredButton
		p2pkhButton := &BlurredButton

		if m.focused == 0 {
			dataButton = &FocusedButton
		} else if m.focused == 1 {
			p2pkhButton = &FocusedButton
		}

		b.WriteString(dataButton.Render("📄 Send Data") + "\n")
		b.WriteString(p2pkhButton.Render("💰 Send Payment (P2PKH)"))

	case StepTransactionDetails:
		if m.config.transactionType == TransactionTypeData {
			b.WriteString("Enter data to send:\n\n")
		} else {
			b.WriteString("Enter payment details:\n\n")
		}

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

	case StepPeriodicChoice:
		b.WriteString("Choose sending method:\n\n")

		if m.config.transactionType == TransactionTypeData {
			b.WriteString(fmt.Sprintf("Data: %s\n\n", m.config.data))
		} else {
			b.WriteString(fmt.Sprintf("Recipient: %s\n", m.config.address))
			b.WriteString(fmt.Sprintf("Amount: %d satoshis\n\n", m.config.amount))
		}

		sendOnceButton := &BlurredButton
		sendPeriodicButton := &BlurredButton

		if m.focused == 0 {
			sendOnceButton = &FocusedButton
		} else if m.focused == 1 {
			sendPeriodicButton = &FocusedButton
		}

		b.WriteString(sendOnceButton.Render("🎯 Send Once") + "\n")
		b.WriteString(sendPeriodicButton.Render("🔄 Send Periodically"))

	case StepPeriodConfig:
		b.WriteString("Configure periodic sending:\n\n")

		if m.config.transactionType == TransactionTypeData {
			b.WriteString(fmt.Sprintf("Data: %s\n\n", m.config.data))
		} else {
			b.WriteString(fmt.Sprintf("Recipient: %s\n", m.config.address))
			b.WriteString(fmt.Sprintf("Amount: %d satoshis\n\n", m.config.amount))
		}

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
		b.WriteString("\n\n" + continueButton.Render("🚀 Start Periodic Sending"))
	}

	if m.errorMsg != "" {
		b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.errorMsg))
	}

	return b.String()
}

func (m *SendForm) nextInput() {
	switch m.step {
	case StepTransactionType:
		m.focused = (m.focused + 1) % 2

	case StepTransactionDetails:
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

	case StepPeriodicChoice:
		m.focused = (m.focused + 1) % 2

	case StepPeriodConfig:
		m.focused = (m.focused + 1) % (len(m.inputs) + 1)
	}
}

func (m *SendForm) prevInput() {
	switch m.step {
	case StepTransactionType, StepPeriodicChoice:
		m.focused--
		if m.focused < 0 {
			m.focused = 1
		}

	case StepTransactionDetails:
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

	case StepPeriodConfig:
		m.focused--
		if m.focused < 0 {
			m.focused = len(m.inputs)
		}
	}
}

func (m *SendForm) continueIsFocused() bool {
	switch m.step {
	case StepTransactionType:
		return false
	case StepTransactionDetails:
		return m.focused == len(m.inputs)
	case StepPeriodicChoice:
		return false
	case StepPeriodConfig:
		return m.focused == len(m.inputs)
	}
	return false
}

// NewSendForm creates a new unified SendForm using the builder pattern
func NewSendForm(manager ManagerInterface, user *fixtures.UserConfig) *SendForm {
	return NewSendFormBuilder(manager, user).Build()
}
