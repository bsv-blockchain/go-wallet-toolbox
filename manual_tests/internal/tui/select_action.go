package tui

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	tea "github.com/charmbracelet/bubbletea"
)

func NewSelectAction(manager ManagerInterface, user *fixtures.UserConfig) tea.Model {
	const backOption = "<- Back"
	actionsTypes := []fixtures.ActionType{
		fixtures.ActionInternalize,
		fixtures.ActionBalance,
		fixtures.ActionListOutputs,
		fixtures.ActionSendData,
		fixtures.ActionSendDataPeriodically,
		fixtures.ActionSendP2PKH,
		fixtures.ActionSendP2PKHPeriodically,
		backOption,
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
		case fixtures.ActionSendData:
			sendDataModel := NewSendDataForm(manager, user)
			return sendDataModel, sendDataModel.Init()
		case fixtures.ActionSendDataPeriodically:
			sendDataModel := NewSendDataPeriodicallyForm(manager, user)
			return sendDataModel, sendDataModel.Init()
		case fixtures.ActionSendP2PKH:
			p2pkhForm := NewSendP2pkhForm(manager, user)
			return p2pkhForm, p2pkhForm.Init()
		case fixtures.ActionSendP2PKHPeriodically:
			p2pkhPeriodicForm := NewSendP2pkhPeriodicallyForm(manager, user)
			return p2pkhPeriodicForm, p2pkhPeriodicForm.Init()
		case backOption:
			// Return to wallet selection view when Back is selected
			selectWalletModel := NewSelectWallet(manager)
			return selectWalletModel, selectWalletModel.Init()
		default:
			manager.Panic(nil, "Unsupported action type: "+string(actionType))
			return nil, nil
		}
	}

	return NewItemSelector(actionsTypes, title, onSelect)
}
