package tui

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"

	tea "github.com/charmbracelet/bubbletea"
)

func NewSelectNetwork(manager ManagerInterface) tea.Model {
	networkTypes := []defs.BSVNetwork{
		defs.NetworkMainnet,
		defs.NetworkTestnet,
	}

	onSelect := func(networkTypes defs.BSVNetwork) (tea.Model, tea.Cmd) {
		manager.SelectNetwork(networkTypes)
		spinner := NewModelSpinner("Initializing network...", Wait(manager.Ctx(), 1*time.Second), func() tea.Model {
			return NewSelectStorage(manager)
		})
		return spinner, spinner.Init()
	}

	return NewItemSelector(networkTypes, "Select network:", onSelect)
}
