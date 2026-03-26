package wallet_test

import (
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type WalletTestSuite struct {
	suite.Suite

	StorageType testabilities.StorageType
}

func TestWalletWithSQLiteStorage(t *testing.T) {
	suite.Run(t, &WalletTestSuite{
		StorageType: testabilities.StorageTypeSQLite,
	})
}

func TestWalletWithRemoteStorage(t *testing.T) {
	suite.Run(t, &WalletTestSuite{
		StorageType: testabilities.StorageTypeRemote,
	})
}

func TestNewWalletArgumentValidation(t *testing.T) {
	validChain := defs.NetworkMainnet
	validKeyDeriver := sdk.NewKeyDeriver(testusers.Alice.PrivateKey(t))
	validStorage := mocks.NewMockWalletStorageProvider(nil)

	tests := map[string]struct {
		chain      defs.BSVNetwork
		keyDeriver *sdk.KeyDeriver
		storage    wdk.WalletStorageProvider
	}{
		"return error on invalid chain": {
			chain:      "unknown",
			keyDeriver: validKeyDeriver,
			storage:    validStorage,
		},
		"return error on nil key deriver": {
			chain:      validChain,
			keyDeriver: nil,
			storage:    validStorage,
		},
		"return error on nil storage": {
			chain:      validChain,
			keyDeriver: validKeyDeriver,
			storage:    nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			w, err := wallet.New(test.chain, test.keyDeriver, test.storage)
			assert.Nil(t, w)
			require.Error(t, err)
		})
	}
}
