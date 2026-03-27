package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

func NewSelectWallet(manager ManagerInterface) tea.Model {
	walletConfigs := manager.GetWalletConfigs()
	items := make([]string, len(walletConfigs))
	walletConfigLookup := make(map[string]fixtures.UserConfig)

	for i, wc := range walletConfigs {
		items[i] = wc.Name
		walletConfigLookup[wc.Name] = wc
	}

	onSelect := func(selectedWalletName string) (tea.Model, tea.Cmd) {
		user := walletConfigLookup[selectedWalletName]
		actionSelector := NewSelectAction(manager, &user)
		return actionSelector, actionSelector.Init()
	}

	onBack := func() (tea.Model, tea.Cmd) {
		storageSelector := NewSelectStorage(manager)
		return storageSelector, storageSelector.Init()
	}

	return NewItemSelectorWithBack(items, "Select wallet:", onSelect, onBack)
}
