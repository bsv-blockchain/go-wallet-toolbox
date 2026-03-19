// manual_tests/internal/tui/send_form_steps.go
package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

// TransactionTypeStep handles transaction type selection
type TransactionTypeStep struct {
	BaseStep
}

func NewTransactionTypeStep(form *SendForm) *TransactionTypeStep {
	return &TransactionTypeStep{BaseStep{form}}
}

func (s *TransactionTypeStep) Init() tea.Cmd {
	s.form.inputs = make([]textinput.Model, 0)
	s.form.focus.SetItems([]FocusItem{
		{Type: ElementButton, Index: ButtonData, Label: "Send Data"},
		{Type: ElementButton, Index: ButtonP2PKH, Label: "Send P2PKH"},
		{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack},
	})
	return textinput.Blink
}

func (s *TransactionTypeStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, model, cmd := s.handleCommonKeys(msg); handled {
		return model, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyEnter:
			current := s.form.focus.CurrentItem()
			if current.Type == ElementButton {
				return s.HandleEnter()
			}
		}
	}

	return s.form, nil
}

func (s *TransactionTypeStep) View() string {
	var b strings.Builder
	b.WriteString("What would you like to send?\n\n")

	buttons := []struct {
		buttonType int
		text       string
	}{
		{ButtonData, "Send Data (OP_RETURN)"},
		{ButtonP2PKH, "Send Payment (P2PKH)"},
		{ButtonBack, fixtures.ButtonBack},
	}

	for _, btn := range buttons {
		style := &fixtures.BlurredButton
		if s.form.focus.IsButtonFocused(btn.buttonType) {
			style = &fixtures.FocusedButton
		}
		b.WriteString(style.Render(btn.text) + "\n")
	}

	return b.String()
}

func (s *TransactionTypeStep) HandleEnter() (tea.Model, tea.Cmd) {
	current := s.form.focus.CurrentItem()
	if current.Type == ElementButton {
		switch current.Index {
		case ButtonData:
			s.form.config.transactionType = TransactionTypeData
			s.form.setState(NewTransactionDetailsStep(s.form))
			return s.form, s.form.state.Init()
		case ButtonP2PKH:
			s.form.config.transactionType = TransactionTypeP2PKH
			s.form.setState(NewTransactionDetailsStep(s.form))
			return s.form, s.form.state.Init()
		case ButtonBack:
			selectAction := NewSelectAction(s.form.manager, s.form.user)
			return selectAction, selectAction.Init()
		}
	}
	return s.form, nil
}

// TransactionDetailsStep handles transaction details input
type TransactionDetailsStep struct {
	BaseStep
}

func NewTransactionDetailsStep(form *SendForm) *TransactionDetailsStep {
	return &TransactionDetailsStep{BaseStep{form}}
}

func (s *TransactionDetailsStep) Init() tea.Cmd {
	if s.form.config.transactionType == TransactionTypeData {
		s.form.inputs = make([]textinput.Model, 1)
		s.form.inputs[0] = textinput.New()
		s.form.inputs[0].Placeholder = "Data to send"
		s.form.inputs[0].CharLimit = 256
		s.form.inputs[0].Width = 50
		s.form.inputs[0].Prompt = ""
	} else {
		s.form.inputs = make([]textinput.Model, 2)

		s.form.inputs[0] = textinput.New()
		s.form.inputs[0].Placeholder = "Recipient address (P2PKH)"
		s.form.inputs[0].CharLimit = 128
		s.form.inputs[0].Width = 50
		s.form.inputs[0].Prompt = ""

		s.form.inputs[1] = textinput.New()
		s.form.inputs[1].Placeholder = "Satoshis to send"
		s.form.inputs[1].CharLimit = 20
		s.form.inputs[1].Width = 50
		s.form.inputs[1].Prompt = ""
	}

	items := make([]FocusItem, 0, 1+len(s.form.inputs)+1)
	items = append(items, FocusItem{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack})
	for i := range s.form.inputs {
		items = append(items, FocusItem{
			Type:  ElementInput,
			Index: i,
			Label: fmt.Sprintf("Input %d", i),
		})
	}
	items = append(items, FocusItem{Type: ElementButton, Index: ButtonContinue, Label: fixtures.ButtonContinue})
	s.form.focus.SetItems(items)

	s.form.focus.current = 1
	s.updateInputFocus()

	return textinput.Blink
}

func (s *TransactionDetailsStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, model, cmd := s.handleCommonKeys(msg); handled {
		return model, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyEnter:
			current := s.form.focus.CurrentItem()
			if current.Type == ElementButton {
				return s.HandleEnter()
			}
		}
	}

	cmds := make([]tea.Cmd, len(s.form.inputs))
	for i := range s.form.inputs {
		s.form.inputs[i], cmds[i] = s.form.inputs[i].Update(msg)
	}

	return s.form, tea.Batch(cmds...)
}

func (s *TransactionDetailsStep) View() string {
	var b strings.Builder

	if s.form.config.transactionType == TransactionTypeData {
		b.WriteString("Enter data to send:\n")
	} else {
		b.WriteString("Enter payment details:\n")
	}

	backStyle := &fixtures.BlurredButton
	if s.form.focus.IsButtonFocused(ButtonBack) {
		backStyle = &fixtures.FocusedButton
	}
	b.WriteString(backStyle.Render(fixtures.ButtonBack) + "\n")

	for i := range s.form.inputs {
		fmt.Fprintf(&b, "%s\n", s.form.inputs[i].View())
	}

	continueStyle := &fixtures.BlurredButton
	if s.form.focus.IsButtonFocused(ButtonContinue) {
		continueStyle = &fixtures.FocusedButton
	}
	b.WriteString(continueStyle.Render(fixtures.ButtonContinue))

	return b.String()
}

func (s *TransactionDetailsStep) HandleEnter() (tea.Model, tea.Cmd) {
	current := s.form.focus.CurrentItem()
	if current.Type != ElementButton {
		return s.form, nil
	}

	switch current.Index {
	case ButtonBack:
		return s.handleBackButton()
	case ButtonContinue:
		return s.handleContinueButton()
	}

	return s.form, nil
}

func (s *TransactionDetailsStep) handleBackButton() (tea.Model, tea.Cmd) {
	s.form.setState(NewTransactionTypeStep(s.form))
	return s.form, s.form.state.Init()
}

func (s *TransactionDetailsStep) handleContinueButton() (tea.Model, tea.Cmd) {
	if s.form.config.transactionType == TransactionTypeData {
		return s.validateAndProceedWithData()
	}
	return s.validateAndProceedWithP2PKH()
}

func (s *TransactionDetailsStep) validateAndProceedWithData() (tea.Model, tea.Cmd) {
	data := strings.TrimSpace(s.form.inputs[0].Value())
	if data == "" {
		s.form.errorMsg = "Data is required"
		return s.form, nil
	}

	s.form.config.data = data
	return s.proceedToNextStep()
}

func (s *TransactionDetailsStep) validateAndProceedWithP2PKH() (tea.Model, tea.Cmd) {
	if err := s.validateP2PKHInputs(); err != nil {
		s.form.errorMsg = err.Error()
		return s.form, nil
	}

	return s.proceedToNextStep()
}

func (s *TransactionDetailsStep) validateP2PKHInputs() error {
	addr := strings.TrimSpace(s.form.inputs[0].Value())
	if addr == "" {
		return fmt.Errorf("recipient address is required")
	}

	amountStr := strings.TrimSpace(s.form.inputs[1].Value())
	if amountStr == "" {
		return fmt.Errorf("satoshis amount is required")
	}

	amt, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil || amt == 0 {
		return fmt.Errorf("invalid satoshis amount")
	}

	// Store validated values
	s.form.config.address = addr
	s.form.config.amount = amt
	return nil
}

func (s *TransactionDetailsStep) proceedToNextStep() (tea.Model, tea.Cmd) {
	s.form.setState(NewPeriodicChoiceStep(s.form))
	s.form.errorMsg = ""
	return s.form, s.form.state.Init()
}

// PeriodicChoiceStep handles periodic choice selection
type PeriodicChoiceStep struct {
	BaseStep
}

func NewPeriodicChoiceStep(form *SendForm) *PeriodicChoiceStep {
	return &PeriodicChoiceStep{BaseStep{form}}
}

func (s *PeriodicChoiceStep) Init() tea.Cmd {
	s.form.inputs = make([]textinput.Model, 0)
	s.form.focus.SetItems([]FocusItem{
		{Type: ElementButton, Index: ButtonSendOnce, Label: "Send Once"},
		{Type: ElementButton, Index: ButtonSendPeriodic, Label: "Send Periodically"},
		{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack},
	})
	return nil
}

func (s *PeriodicChoiceStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, model, cmd := s.handleCommonKeys(msg); handled {
		return model, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyEnter:
			current := s.form.focus.CurrentItem()
			if current.Type == ElementButton {
				return s.HandleEnter()
			}
		}
	}

	return s.form, nil
}

func (s *PeriodicChoiceStep) View() string {
	var b strings.Builder
	b.WriteString("Choose sending method:\n\n")

	if s.form.config.transactionType == TransactionTypeData {
		fmt.Fprintf(&b, "Data: %s\n\n", s.form.config.data)
	} else {
		fmt.Fprintf(&b, "Recipient: %s\n", s.form.config.address)
		fmt.Fprintf(&b, "Amount: %d satoshis\n\n", s.form.config.amount)
	}

	buttons := []struct {
		buttonType int
		text       string
	}{
		{ButtonSendOnce, "Send Once"},
		{ButtonSendPeriodic, "Send Periodically"},
		{ButtonBack, fixtures.ButtonBack},
	}

	for _, btn := range buttons {
		style := &fixtures.BlurredButton
		if s.form.focus.IsButtonFocused(btn.buttonType) {
			style = &fixtures.FocusedButton
		}
		b.WriteString(style.Render(btn.text) + "\n")
	}

	return b.String()
}

func (s *PeriodicChoiceStep) HandleEnter() (tea.Model, tea.Cmd) {
	current := s.form.focus.CurrentItem()
	if current.Type == ElementButton {
		switch current.Index {
		case ButtonBack:
			s.form.setState(NewTransactionDetailsStep(s.form))
			return s.form, s.form.state.Init()
		case ButtonSendOnce:
			s.form.config.isPeriodic = false
			return s.form.executeAction()
		case ButtonSendPeriodic:
			s.form.config.isPeriodic = true
			s.form.setState(NewPeriodConfigStep(s.form))
			return s.form, s.form.state.Init()
		}
	}
	return s.form, nil
}

// PeriodConfigStep handles period configuration
type PeriodConfigStep struct {
	BaseStep
}

func NewPeriodConfigStep(form *SendForm) *PeriodConfigStep {
	return &PeriodConfigStep{BaseStep{form}}
}

func (s *PeriodConfigStep) Init() tea.Cmd {
	s.form.inputs = make([]textinput.Model, 1)
	s.form.inputs[0] = textinput.New()
	s.form.inputs[0].Placeholder = "Time period (ms)"
	s.form.inputs[0].CharLimit = 10
	s.form.inputs[0].Width = 50
	s.form.inputs[0].Prompt = ""

	s.form.focus.SetItems([]FocusItem{
		{Type: ElementButton, Index: ButtonBack, Label: fixtures.ButtonBack},
		{Type: ElementInput, Index: 0, Label: "Period Input"},
		{Type: ElementButton, Index: ButtonContinue, Label: fixtures.ButtonContinue},
	})

	s.form.focus.current = 1
	s.updateInputFocus()

	return textinput.Blink
}

func (s *PeriodConfigStep) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, model, cmd := s.handleCommonKeys(msg); handled {
		return model, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { //nolint:exhaustive // only specific keys handled, others ignored
		case tea.KeyEnter:
			current := s.form.focus.CurrentItem()
			if current.Type == ElementButton {
				return s.HandleEnter()
			}
		}
	}

	cmds := make([]tea.Cmd, len(s.form.inputs))
	for i := range s.form.inputs {
		s.form.inputs[i], cmds[i] = s.form.inputs[i].Update(msg)
	}

	return s.form, tea.Batch(cmds...)
}

func (s *PeriodConfigStep) View() string {
	var b strings.Builder
	b.WriteString("Configure periodic sending:\n")

	if s.form.config.transactionType == TransactionTypeData {
		fmt.Fprintf(&b, "Data: %s\n", s.form.config.data)
	} else {
		fmt.Fprintf(&b, "Recipient: %s\n", s.form.config.address)
		fmt.Fprintf(&b, "Amount: %d satoshis\n", s.form.config.amount)
	}

	backStyle := &fixtures.BlurredButton
	if s.form.focus.IsButtonFocused(ButtonBack) {
		backStyle = &fixtures.FocusedButton
	}
	b.WriteString(backStyle.Render(fixtures.ButtonBack) + "\n")

	for i := range s.form.inputs {
		b.WriteString(s.form.inputs[i].View())
		if i < len(s.form.inputs)-1 {
			b.WriteRune('\n')
		}
	}

	continueStyle := &fixtures.BlurredButton
	if s.form.focus.IsButtonFocused(ButtonContinue) {
		continueStyle = &fixtures.FocusedButton
	}
	b.WriteString("\n" + continueStyle.Render("Start Periodic Sending"))

	return b.String()
}

func (s *PeriodConfigStep) HandleEnter() (tea.Model, tea.Cmd) {
	current := s.form.focus.CurrentItem()
	if current.Type == ElementButton {
		switch current.Index {
		case ButtonBack:
			s.form.setState(NewPeriodicChoiceStep(s.form))
			return s.form, s.form.state.Init()
		case ButtonContinue:
			periodStr := strings.TrimSpace(s.form.inputs[0].Value())
			periodMs, err := strconv.Atoi(periodStr)
			if err != nil || periodMs <= 0 {
				s.form.errorMsg = "Invalid time period"
				return s.form, nil
			}
			s.form.config.period = time.Duration(periodMs) * time.Millisecond
			return s.form.executeAction()
		}
	}
	return s.form, nil
}
