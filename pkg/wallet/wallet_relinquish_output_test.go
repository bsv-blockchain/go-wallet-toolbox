package wallet_test

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletRelinquishOutputArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       sdk.RelinquishOutputArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: sdk.RelinquishOutputArgs{
				Basket: "test-basket",
				Output: *testutils.SdkOutpoint(t, fixtures.MockOutpoint),
			},
		},

		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: sdk.RelinquishOutputArgs{
				Basket: "test-basket",
				Output: *testutils.SdkOutpoint(t, fixtures.MockOutpoint),
			},
		},

		"basket too long": {
			originator: fixtures.DefaultOriginator,
			args: sdk.RelinquishOutputArgs{
				Basket: strings.Repeat("a", 301),
				Output: *testutils.SdkOutpoint(t, fixtures.MockOutpoint),
			},
		},
	}

	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

			// when:
			result, err := aliceWallet.RelinquishOutput(t.Context(), test.args, test.originator)

			// then:
			then.Result(result).HasError(err)
			then.Storage().HadNoInteraction()
		})
	}
}

func (s *WalletTestSuite) TestWalletRelinquishOutputErrorPaths() {
	s.Run("output not found", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// when:
		args := sdk.RelinquishOutputArgs{
			Basket: "test-basket",
			Output: *testutils.SdkOutpoint(t, "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6.1"),
		}

		result, err := aliceWallet.RelinquishOutput(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
		assert.Contains(t, err.Error(), "no output found")
	})

	s.Run("output not in specified basket", func() {
		t := s.T()
		const topUpValue = 100_000

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := sdk.RelinquishOutputArgs{
			Basket: "wrong-basket",
			Output: transaction.Outpoint{
				Txid:  *txFromFaucet.TX().TxID(),
				Index: 0,
			},
		}

		result, err := aliceWallet.RelinquishOutput(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
		assert.Contains(t, err.Error(), "no output found")
		assert.Contains(t, err.Error(), "wrong-basket")
	})

	s.Run("wrong output index", func() {
		t := s.T()
		const topUpValue = 100_000

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := sdk.RelinquishOutputArgs{
			Basket: "",
			Output: transaction.Outpoint{
				Txid:  *txFromFaucet.TX().TxID(),
				Index: 1,
			},
		}

		result, err := aliceWallet.RelinquishOutput(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
		assert.Contains(t, err.Error(), "no output found")
		assert.Contains(t, err.Error(), "vout 1")
	})

	s.Run("output from different user", func() {
		t := s.T()
		const topUpValue = 100_000

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		bobWallet := given.BobWalletWithStorage(s.StorageType)

		// and:
		txFromBobFaucet, _ := given.Faucet(bobWallet).TopUp(topUpValue)

		// when:
		args := sdk.RelinquishOutputArgs{
			Basket: "",
			Output: transaction.Outpoint{
				Txid:  *txFromBobFaucet.TX().TxID(),
				Index: 0,
			},
		}

		result, err := aliceWallet.RelinquishOutput(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
		assert.Contains(t, err.Error(), "no output found")
	})
}

func (s *WalletTestSuite) TestWalletRelinquishOutputSuccess() {
	s.Run("successfully relinquish output", func() {
		t := s.T()
		const topUpValue = 100_000

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := sdk.RelinquishOutputArgs{
			Basket: "",
			Output: transaction.Outpoint{
				Txid:  *txFromFaucet.TX().TxID(),
				Index: 0,
			},
		}

		result, err := aliceWallet.RelinquishOutput(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Relinquished)
	})

	s.Run("relinquish one output out of two", func() {
		t := s.T()
		const topUpValue1 = 50_000
		const topUpValue2 = 100_000

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		tx1, _ := given.Faucet(aliceWallet).TopUp(topUpValue1)

		given.Faucet(aliceWallet).TopUp(topUpValue2)

		// when:
		args := sdk.RelinquishOutputArgs{
			Basket: "",
			Output: transaction.Outpoint{
				Txid:  *tx1.TX().TxID(),
				Index: 0,
			},
		}

		result, err := aliceWallet.RelinquishOutput(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Relinquished)

	})
}
