package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

func NewSelectAction(manager ManagerInterface, user *fixtures.UserConfig) tea.Model {
	actionsTypes := []fixtures.ActionType{
		fixtures.ActionInternalize,
		fixtures.ActionBalance,
		fixtures.ActionListOutputs,
		fixtures.ActionSend,
		fixtures.ActionNoSendSendWith,
		fixtures.ButtonBack,
	}

	title := fmt.Sprintf("Select action for %s:", user.Name)

	onSelect := func(actionType fixtures.ActionType) (tea.Model, tea.Cmd) {
		switch actionType {
		case fixtures.ActionInternalize:
			internalizeModel := NewInternalizeActionForm(manager, user)
			return internalizeModel, internalizeModel.Init()
		case fixtures.ActionBalance:
			balanceModel := NewBalanceView(manager, user)
			return balanceModel, balanceModel.Init()
		case fixtures.ActionListOutputs:
			listOutputsForm := NewListOutputsForm(manager, user)
			return listOutputsForm, listOutputsForm.Init()
		case fixtures.ActionSend:
			sendForm := NewSendForm(manager, user)
			return sendForm, sendForm.Init()
		case fixtures.ActionNoSendSendWith:
			noSendSendWithForm := NewNoSendSendWithForm(manager, user)
			return noSendSendWithForm, noSendSendWithForm.Init()
		case fixtures.ButtonBack:
			selectWalletModel := NewSelectWallet(manager)
			return selectWalletModel, selectWalletModel.Init()
		default:
			manager.Panic(nil, "Unsupported action type: "+string(actionType))
			return nil, nil
		}
	}

	return NewItemSelector(actionsTypes, title, onSelect)
}
