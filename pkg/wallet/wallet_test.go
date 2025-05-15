package wallet_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWalletArgumentValidation(t *testing.T) {
	validChain := defs.NetworkMainnet
	validKeyDeriver := sdk.NewKeyDeriver(testusers.Alice.PrivateKey(t))
	validStorage := mocks.NewMockWalletStorageWriter(nil)

	tests := map[string]struct {
		chain      defs.BSVNetwork
		keyDeriver *sdk.KeyDeriver
		storage    wdk.WalletStorageWriter
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
