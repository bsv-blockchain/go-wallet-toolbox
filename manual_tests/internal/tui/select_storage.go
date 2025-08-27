package tui

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	tea "github.com/charmbracelet/bubbletea"
)

func NewSelectStorage(manager ManagerInterface) tea.Model {
	storageTypes := []fixtures.StorageType{
		fixtures.StorageTypeLocalSQLite,
		fixtures.StorageTypeLocalPostgres,
		fixtures.StorageTypeRemoteSQLite,
		fixtures.StorageTypeRemotePostgres,
	}

	onSelect := func(storageType fixtures.StorageType) (tea.Model, tea.Cmd) {
		err := manager.SelectStorageType(storageType)
		if err != nil {
			manager.Panic(err, "failed to select storage type")
			return nil, nil
		}
		spinner := NewModelSpinner("Initializing storage...", Wait(manager.Ctx(), 1*time.Second), func() tea.Model {
			return NewSelectWallet(manager)
		})
		return spinner, spinner.Init()
	}

	onBack := func() (tea.Model, tea.Cmd) {
		networkSelector := NewSelectNetwork(manager)
		return networkSelector, networkSelector.Init()
	}

	return NewItemSelectorWithBack(storageTypes, "Select storage type:", onSelect, onBack)
}
