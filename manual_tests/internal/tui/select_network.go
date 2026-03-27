package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func NewSelectNetwork(manager ManagerInterface) tea.Model {
	networkTypes := []defs.BSVNetwork{
		defs.NetworkTestnet,
		defs.NetworkMainnet,
	}

	onSelect := func(networkTypes defs.BSVNetwork) (tea.Model, tea.Cmd) {
		manager.SelectNetwork(networkTypes)
		return NewSelectStorage(manager), nil
	}

	return NewItemSelector(networkTypes, "Select network:", onSelect)
}
