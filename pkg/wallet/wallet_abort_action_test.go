package wallet_test

import (
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletAbortActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.AbortActionArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: func() sdk.AbortActionArgs {
				return fixtures.DefaultWalletAbortActionArgs()
			},
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: func() sdk.AbortActionArgs {
				return fixtures.DefaultWalletAbortActionArgs()
			},
		},
		"empty reference": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.AbortActionArgs {
				return sdk.AbortActionArgs{
					Reference: []byte(""),
				}
			},
		},
		"nil args": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.AbortActionArgs {
				return sdk.AbortActionArgs{}
			},
		},
	}

	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

			// when:
			result, err := aliceWallet.AbortAction(t.Context(), test.args(), test.originator)

			// then:
			then.Result(result).HasError(err)
			then.Storage().HadNoInteraction()
		})
	}
}

func (s *WalletTestSuite) TestWalletAbortActionSuccess() {
	s.Run("successful abort of created transaction", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		given.Faucet(aliceWallet).TopUp(100_000)

		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
		createResult, _ := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(string(createResult.SignableTransaction.Reference))
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Aborted, "Action should be successfully aborted")

		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.HasActionsCount(1)
	})

	s.Run("successful abort with transaction ID as reference", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		given.Faucet(aliceWallet).TopUp(100_000)

		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
		createResult, _ := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)

		reference := createResult.SignableTransaction.Reference
		require.NotEmpty(t, reference, "Should have reference")

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(string(reference))
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Aborted, "Action should be successfully aborted")

		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.HasActionsCount(1)
	})

	s.Run("successful spending after abort", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		given.Faucet(aliceWallet).TopUp(100_000)

		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
		createResult, _ := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)

		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(string(createResult.SignableTransaction.Reference))
		abortResult, _ := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		require.True(t, abortResult.Aborted, "Action should be aborted")

		// when:
		newCreateArgs := fixtures.DefaultWalletCreateActionArgs(t)
		newCreateResult, err := aliceWallet.CreateAction(t.Context(), newCreateArgs, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err, "Should be able to create new action after abort")
		require.NotNil(t, newCreateResult, "New create result should not be nil")
		assert.NotEmpty(t, newCreateResult.Txid, "New action should have a TxID")

		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.HasActionsCount(2)
	})
}

func (s *WalletTestSuite) TestWalletAbortActionErrorCases() {
	s.Run("transaction not found by reference", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference("bm9uLWV4aXN0ZW50LXJlZg==")
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no transaction found with reference or txid")
	})

	s.Run("transaction not found by TxID", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference("1234567890123456789012345678901234567890123456789012345678901234")
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no transaction found with reference or txid")
	})

	s.Run("transaction not abortable - incoming transaction", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		faucetTx, _ := given.Faucet(aliceWallet).TopUp(100_000)

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(faucetTx.ID().String())
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "must be an outgoing transaction")
	})

	s.Run("transaction not abortable - already failed", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		given.Faucet(aliceWallet).TopUp(100_000)

		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
		createResult, _ := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)
		reference := createResult.SignableTransaction.Reference

		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(string(reference))
		aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// when:
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "action with status failed cannot be aborted")
	})
}
