// manual_tests/internal/tui/send_form.go
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
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
	form := &SendForm{
		manager: b.manager,
		user:    b.user,
		config:  &SendConfig{},
		focus:   NewFocusManager(),
	}
	form.setState(NewTransactionTypeStep(form))
	return form
}

// FocusableElement represents different types of focusable elements
type FocusableElement int

const (
	ElementInput FocusableElement = iota
	ElementButton
)

// FocusItem represents a single focusable item
type FocusItem struct {
	Type  FocusableElement
	Index int    // For inputs: input index, For buttons: button type
	Label string // For display/debugging
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

// Button types
const (
	ButtonBack = iota
	ButtonContinue
	ButtonSendOnce
	ButtonSendPeriodic
	ButtonData
	ButtonP2PKH
)

// FormStepState interface defines the behavior for each step
type FormStepState interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() string
	HandleEnter() (tea.Model, tea.Cmd)
}

// BaseStep provides common functionality for all steps
type BaseStep struct {
	form *SendForm
}

func (b *BaseStep) updateInputFocus() {
	// Clear all input focus
	for i := range b.form.inputs {
		b.form.inputs[i].Blur()
	}

	// Set focus on current input if applicable
	current := b.form.focus.CurrentItem()
	if current.Type == ElementInput && current.Index < len(b.form.inputs) {
		b.form.inputs[current.Index].Focus()
	}
}

func (b *BaseStep) handleCommonKeys(msg tea.Msg) (handled bool, model tea.Model, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyEnter:
			current := b.form.focus.CurrentItem()
			if current.Type == ElementButton {
				return false, b.form, nil
			} else {
				b.form.focus.Next()
				b.updateInputFocus()
				return true, b.form, nil
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return true, b.form, tea.Quit
		case tea.KeyShiftTab, tea.KeyCtrlP, tea.KeyUp:
			b.form.focus.Previous()
			b.updateInputFocus()
			return true, b.form, nil
		case tea.KeyTab, tea.KeyCtrlN, tea.KeyDown:
			b.form.focus.Next()
			b.updateInputFocus()
			return true, b.form, nil
		}
	}
	return false, b.form, nil
}

// SendForm main struct
type SendForm struct {
	manager  ManagerInterface
	user     *fixtures.UserConfig
	config   *SendConfig
	inputs   []textinput.Model
	focus    *FocusManager
	errorMsg string
	state    FormStepState
}

func (m *SendForm) setState(state FormStepState) {
	m.state = state
	m.errorMsg = ""
}

func (m *SendForm) Init() tea.Cmd {
	return m.state.Init()
}

func (m *SendForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.state.Update(msg)
}

func (m *SendForm) View() string {
	view := m.state.View()
	if m.errorMsg != "" {
		view += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.errorMsg)
	}
	return view
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

// NewSendForm creates a new unified SendForm using the builder pattern
func NewSendForm(manager ManagerInterface, user *fixtures.UserConfig) *SendForm {
	return NewSendFormBuilder(manager, user).Build()
}
