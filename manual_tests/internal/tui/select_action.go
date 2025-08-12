package tui

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	tea "github.com/charmbracelet/bubbletea"
)

func NewSelectAction(manager ManagerInterface, user *fixtures.UserConfig) tea.Model {
	actionsTypes := []fixtures.ActionType{
		fixtures.ActionInternalize,
		fixtures.ActionBalance,
		fixtures.ActionSendData,
		fixtures.ActionSendDataPeriodically,
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
		case fixtures.ActionSendData:
			sendDataModel := NewSendDataForm(manager, user)
			return sendDataModel, sendDataModel.Init()
		case fixtures.ActionSendDataPeriodically:
			sendDataModel := NewSendDataPeriodicallyForm(manager, user)
			return sendDataModel, sendDataModel.Init()
		default:
			manager.Panic(nil, "Unsupported action type: "+string(actionType))
			return nil, nil
		}
	}

	return NewItemSelector(actionsTypes, title, onSelect)
}
