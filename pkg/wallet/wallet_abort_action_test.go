package wallet_test

import (
	"context"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestAbortActionOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.AbortActionResult, error) {
			args := fixtures.DefaultWalletAbortActionArgs()
			return w.AbortAction(ctx, args, originator)
		},
	)
}

func TestWalletAbortActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.AbortActionArgs
	}{
		"empty reference": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.AbortActionArgs {
				return sdk.AbortActionArgs{
					Reference: []byte(""),
				}
			},
		},
		"empty args": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.AbortActionArgs {
				return sdk.AbortActionArgs{}
			},
		},
		"invalid reference": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.AbortActionArgs {
				return sdk.AbortActionArgs{
					Reference: []byte("this is invalid reference"),
				}
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

		// and:
		given.Faucet(aliceWallet).TopUp(100_000)

		// and:
		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
		createResult, err := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "cannot create action, invalid test setup?")

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(createResult.SignableTransaction.Reference)
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Aborted, "Action should be successfully aborted")

		// and:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.HasActionsCount(1)
	})

	s.Run("successful abort no send action with transaction ID as reference", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		given.Faucet(aliceWallet).TopUp(100_000)

		// and:
		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithNoSend(true))
		createResult, err := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "cannot create action, invalid test setup?")

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(createResult.Txid.String())
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Aborted, "Action should be successfully aborted")

		// and:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.HasActionsCount(1)
	})

	s.Run("successful spending after abort", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		given.Faucet(aliceWallet).TopUp(fixtures.DefaultCreateActionOutputSatoshis + 1)

		// when: we're sending all the funds from top-up
		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
		createResult, err := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "cannot create action, invalid test setup?")

		// and: we want to spend some more - we shouldn't have enough funds.
		_, err = aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)
		require.Error(t, err, "should be unable to create action, because of not enough funds")

		// and: now we're aborting the created transaction
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(createResult.SignableTransaction.Reference)
		abortResult, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err, "should be able to abort created transaction")
		assert.True(t, abortResult.Aborted, "Action should be aborted")

		// when: we want to spend some more funds again
		newCreateResult, err := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err, "Should be able to create new action after abort")
		require.NotNil(t, newCreateResult, "New create result should not be nil")
		assert.NotEmpty(t, newCreateResult.Txid, "New action should have a TxID")

		// and:
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
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no transaction found with reference or txid")
	})

	s.Run("transaction not found by TxID", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference("1234567890123456789012345678901234567890123456789012345678901234")
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no transaction found with reference or txid")
	})

	s.Run("transaction not abortable - incoming transaction", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		faucetTx, _ := given.Faucet(aliceWallet).TopUp(100_000)

		// when:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(faucetTx.ID().String())
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "must be an outgoing transaction")
	})

	s.Run("transaction not abortable - already aborted", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		given.Faucet(aliceWallet).TopUp(100_000)

		// and:
		createArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
		createResult, err := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "cannot create action, invalid test setup?")

		// and:
		abortArgs := fixtures.DefaultWalletAbortActionArgsWithReference(createResult.SignableTransaction.Reference)

		// when
		_, err = aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "cannot abort action, invalid test setup?")

		// and:
		result, err := aliceWallet.AbortAction(t.Context(), abortArgs, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "action with status failed cannot be aborted")
	})
}
