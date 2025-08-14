package tui

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type ManagerInterface interface {
	SelectStorageType(storageType fixtures.StorageType) error
	Panic(err error, msg string)
	Ctx() context.Context
	GetWalletConfigs() []fixtures.UserConfig
	GetBSVNetwork() defs.BSVNetwork
	InternalizeTxID(txID string, user fixtures.UserConfig, keyID brc29.KeyID, address string) (fixtures.Summary, error)
	Balance(user fixtures.UserConfig) (uint64, error)
	CreateActionWithData(user fixtures.UserConfig, data string) (string, fixtures.Summary, error)
}
