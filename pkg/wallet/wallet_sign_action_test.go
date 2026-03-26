package wallet_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/asserttx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestSignAction_ValidationError(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.SignActionResult, error) {
			args := sdk.SignActionArgs{
				Reference: []byte(fixtures.Reference),
			}
			return w.SignAction(ctx, args, originator)
		},
	)

	t.Run("empty args", func(t *testing.T) {
		given, _, cleanup := testabilities.New(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

		_, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{}, fixtures.DefaultOriginator)

		require.Error(t, err)
	})
}

func (s *WalletTestSuite) TestWalletSignAction_SignIsNotNecessary() {
	s.Run("sign action of tx with no inputs provided", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithSignAndProcess(false),
		)

		createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// when:
		signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
			Reference: createActionResult.SignableTransaction.Reference,
		}, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, signActionResult)

		thenTx := asserttx.RestoredFromBEEFBytes(t, signActionResult.Tx)

		thenTx.HasInputsThatFundsOutputs().HasMinimalFee()

		thenTx.Inputs().AllHaveUnlockingScript().HasTotalInputValue(topUpValue)

		thenTx.Outputs().AllHaveLockingScript()

		thenTx.Output(0).
			HasLockingScript(args.Outputs[0].LockingScript).
			HasSatoshis(args.Outputs[0].Satoshis).
			IsNotChange()

		// and check db state:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		thenState.ActionAtIndex(0).
			WithTxID(txFromFaucet.ID().String()).
			WithSatoshis(topUpValue)

		const fee = 1
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithTxID(signActionResult.Txid.String()).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee) //nolint:gosec // safe: satoshis fit in int64

		thenCreatedAction.OutputAtIndex(0).
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(true).
			WithBasket("")
	})

	s.Run("sign action of tx with input with unlocking script provided during create action", func() {
		t := s.T()
		const topUpValue = testValueForFunding
		const inputValue = 100

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		input := given.InputForUser(testusers.Alice).WithSatoshis(inputValue)

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// and:
		given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInput(input),
			walletargs.WithSignAndProcess(false),
		)

		createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// when:
		signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
			Reference: createActionResult.SignableTransaction.Reference,
		}, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, signActionResult)

		thenTx := asserttx.RestoredFromBEEFBytes(t, signActionResult.Tx)

		thenTx.HasInputsThatFundsOutputs().HasMinimalFee()

		thenTx.Inputs().AllHaveUnlockingScript().HasTotalInputValue(topUpValue + inputValue)

		thenTx.Outputs().AllHaveLockingScript()

		thenTx.Output(0).
			HasLockingScript(args.Outputs[0].LockingScript).
			HasSatoshis(args.Outputs[0].Satoshis).
			IsNotChange()

		// and check db state:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		thenState.ActionAtIndex(0).
			WithTxID(txFromFaucet.ID().String()).
			WithSatoshis(topUpValue)

		const fee = 1
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithTxID(signActionResult.Txid.String()).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) + inputValue - fee) //nolint:gosec // safe: satoshis fit in int64

		thenCreatedAction.OutputAtIndex(0).
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(true).
			WithBasket("")
	})
}

func (s *WalletTestSuite) TestWalletSignAction_SignSingleInput() {
	s.Run("attempt to sign action of tx with provided unlocking script length only, without client-side sign", func() {
		t := s.T()
		const topUpValue = testValueForFunding
		const inputValue = 100

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		input := given.InputForUser(testusers.Alice).WithSatoshis(inputValue).WithNoUnlockingScript()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// and:
		given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInput(input),
			walletargs.WithSignAndProcess(false),
		)

		createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// when:
		signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
			Reference: createActionResult.SignableTransaction.Reference,
		}, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, signActionResult)

		// and check db state:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		thenState.ActionAtIndex(0).
			WithTxID(txFromFaucet.ID().String()).
			WithSatoshis(topUpValue)

		const fee = 1
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithoutTxID().
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) + inputValue - fee) //nolint:gosec // safe: satoshis fit in int64

		thenCreatedAction.OutputAtIndex(0).
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(true).
			WithBasket("")
	})

	s.Run("sign action of tx with provided unlocking script length only, with client-side sign", func() {
		t := s.T()
		const topUpValue = testValueForFunding
		const inputValue = 100

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		input := given.InputForUser(testusers.Alice).WithSatoshis(inputValue).WithNoUnlockingScript()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// and:
		given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInput(input),
			walletargs.WithSignAndProcess(false),
		)

		createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// when:
		signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
			Reference: createActionResult.SignableTransaction.Reference,
			Spends: map[uint32]sdk.SignActionSpend{
				0: {
					UnlockingScript: input.UnlockingScript().Bytes(),
				},
			},
		}, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, signActionResult)

		thenTx := asserttx.RestoredFromBEEFBytes(t, signActionResult.Tx)

		thenTx.HasInputsThatFundsOutputs().HasMinimalFee()

		thenTx.Inputs().AllHaveUnlockingScript().HasTotalInputValue(topUpValue + inputValue)

		thenTx.Outputs().AllHaveLockingScript()

		thenTx.Output(0).
			HasLockingScript(args.Outputs[0].LockingScript).
			HasSatoshis(args.Outputs[0].Satoshis).
			IsNotChange()

		// and check db state:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		thenState.ActionAtIndex(0).
			WithTxID(txFromFaucet.ID().String()).
			WithSatoshis(topUpValue)

		const fee = 1
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithTxID(signActionResult.Txid.String()).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) + inputValue - fee) //nolint:gosec // safe: satoshis fit in int64

		thenCreatedAction.OutputAtIndex(0).
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(true).
			WithBasket("")
	})
}

func (s *WalletTestSuite) TestWalletSignAction_MergeOptions() {
	tests := map[string]struct {
		createActionModifiers []func(args *sdk.CreateActionArgs)
		signActionOptions     sdk.SignActionOptions
		then                  func(*testing.T, *sdk.SignActionResult)
	}{
		"accept delayed broadcast": {
			createActionModifiers: []func(args *sdk.CreateActionArgs){
				walletargs.WithDelayedBroadcast(),
			},
			signActionOptions: sdk.SignActionOptions{
				AcceptDelayedBroadcast: to.Ptr(false),
			},
			then: func(t *testing.T, result *sdk.SignActionResult) {
				allSent := seq.Every(seq.FromSlice(result.SendWithResults), func(it sdk.SendWithResult) bool {
					return it.Status == sdk.ActionResultStatusUnproven
				})

				require.True(t, allSent)
			},
		},
		"return tx id only": {
			signActionOptions: sdk.SignActionOptions{
				ReturnTXIDOnly: to.Ptr(true),
			},
			then: func(t *testing.T, result *sdk.SignActionResult) {
				require.Empty(t, result.Tx)
				require.NotEmpty(t, result.Txid)
			},
		},
		"no send": {
			signActionOptions: sdk.SignActionOptions{
				NoSend: to.Ptr(true),
			},
			then: func(t *testing.T, result *sdk.SignActionResult) {
				require.Empty(t, result.SendWithResults)
			},
		},
		"send with": {
			createActionModifiers: []func(args *sdk.CreateActionArgs){
				walletargs.WithSendWith(chainhash.Hash{}),
			},
			signActionOptions: sdk.SignActionOptions{
				SendWith: []chainhash.Hash{},
			},
			then: func(t *testing.T, result *sdk.SignActionResult) {
				require.Len(t, result.SendWithResults, 1)
			},
		},
	}
	for name, test := range tests {
		s.Run(name, func() {
			t := s.T()

			const topUpValue = testValueForFunding
			const inputValue = 100

			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			// and:
			input := given.InputForUser(testusers.Alice).WithSatoshis(inputValue)

			// and:
			aliceWallet := given.AliceWalletWithStorage(s.StorageType)

			// and:
			given.Faucet(aliceWallet).TopUp(topUpValue)

			// and:
			given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

			// when:
			args := fixtures.DefaultWalletCreateActionArgs(t,
				append(test.createActionModifiers,
					walletargs.WithInput(input),
					walletargs.WithSignAndProcess(false),
				)...,
			)

			createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

			// then:
			require.NoError(t, err)

			// when:
			signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
				Reference: createActionResult.SignableTransaction.Reference,
				Options:   &test.signActionOptions,
			}, fixtures.DefaultOriginator)

			// then:
			require.NoError(t, err)
			require.NotNil(t, signActionResult)

			// and:
			test.then(t, signActionResult)
		})
	}
}

func (s *WalletTestSuite) TestWalletSignAction_PendingSignActions_CacheErrors() {
	mockErr := fmt.Errorf("some error")

	tests := map[string]struct {
		setup             func(cache *testabilities.MockPendingSignActionRepo)
		errOnCreateAction bool
		errOnSignAction   bool
	}{
		"error on set": {
			setup: func(cache *testabilities.MockPendingSignActionRepo) {
				cache.ErrOnSet = mockErr
			},
			errOnCreateAction: true,
		},
		"error on get": {
			setup: func(cache *testabilities.MockPendingSignActionRepo) {
				cache.ErrOnGet = mockErr
			},
			errOnSignAction: true,
		},
		"error on delete": {
			setup: func(cache *testabilities.MockPendingSignActionRepo) {
				cache.ErrOnDelete = mockErr
			},
			errOnSignAction: false, // NOTE: delete error is only logged, not returned
		},
	}
	for name, test := range tests {
		s.Run(name, func() {
			t := s.T()

			const topUpValue = testValueForFunding
			const inputValue = 100

			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			// and:
			input := given.InputForUser(testusers.Alice).WithSatoshis(inputValue)

			// and:
			mockCache := testabilities.NewMockPendingSignActionCache()

			// and:
			aliceWallet := given.Wallet().
				WithActiveStorage(s.StorageType).
				WithServices().
				WithWalletOpts(wallet.WithPendingSignActionsRepository(mockCache)).
				ForUser(testusers.Alice)

			// and:
			test.setup(mockCache)

			// and:
			given.Faucet(aliceWallet).TopUp(topUpValue)

			// and:
			given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

			// when:
			args := fixtures.DefaultWalletCreateActionArgs(t,
				walletargs.WithInput(input),
				walletargs.WithSignAndProcess(false),
			)

			createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

			// then:
			if test.errOnCreateAction {
				require.ErrorIs(t, err, mockErr)
				return
			}
			require.NoError(t, err)

			// when:
			signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
				Reference: createActionResult.SignableTransaction.Reference,
			}, fixtures.DefaultOriginator)

			// then:
			if test.errOnSignAction {
				require.ErrorIs(t, err, mockErr)
				require.Nil(t, signActionResult)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, signActionResult)
		})
	}
}

func (s *WalletTestSuite) TestWalletSignAction_SigningNotExistingAction() {
	s.Run("attempt to sign an action that doesn't exist", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		_, _ = given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		const nonExistingReference = "non-existing-reference"
		signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
			Reference: []byte(nonExistingReference),
		}, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, signActionResult)
	})
}
