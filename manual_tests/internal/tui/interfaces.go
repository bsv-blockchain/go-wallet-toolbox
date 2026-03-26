package tui

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type ManagerInterface interface {
	SelectNetwork(network defs.BSVNetwork)
	SelectStorageType(storageType fixtures.StorageType) error
	Panic(err error, msg string)
	Ctx() context.Context
	GetWalletConfigs() []fixtures.UserConfig
	GetBSVNetwork() defs.BSVNetwork
	InternalizeTxID(txID string, user fixtures.UserConfig, keyID brc29.KeyID, address string) (fixtures.Summary, error)
	Balance(user fixtures.UserConfig) (uint64, error)
	CreateActionWithData(user fixtures.UserConfig, data string) (string, fixtures.Summary, error)
	CreateActionWithP2pkh(user fixtures.UserConfig, recipientAddress string, satoshis uint64) (string, fixtures.Summary, error)
	ListOutputs(user fixtures.UserConfig, limit, offset uint32, includeLabels bool, basket string) (fixtures.Summary, error)
	ActionsStats(user fixtures.UserConfig) (map[string]int, error)
	ExecuteNoSendSendWith(user fixtures.UserConfig, txCount int, dataPrefix string) (*NoSendSendWithResult, error)
}
