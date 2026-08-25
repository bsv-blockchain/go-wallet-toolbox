package wallet_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/asserttx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestSignAction_ValidationError(t *testing.T) {
	// NOTE: SignAction cannot reuse RunOriginatorValidationErrorTests: a rejected sign
	// action leaves the action created by CreateAction parked as 'unsigned' in storage,
	// so the wallet compensates by aborting it, which is a storage interaction.
	originatorErrorTestCases := map[string]struct {
		originator string
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
		},
		"empty originator part": {
			originator: "a..c",
		},
	}

	for name, test := range originatorErrorTestCases {
		t.Run(name, func(t *testing.T) {
			// given: real storage, because the rejected sign action triggers a
			// compensating abort of the referenced action
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeSQLite)

			// when:
			result, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
				Reference: []byte(fixtures.Reference),
			}, test.originator)

			// then:
			then.Result(result).HasError(err)
			require.ErrorContains(t, err, "invalid originator")
		})
	}

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
		args := fixtures.DefaultWalletCreateActionArgs(
			t,
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
		args := fixtures.DefaultWalletCreateActionArgs(
			t,
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
		args := fixtures.DefaultWalletCreateActionArgs(
			t,
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

		// and check db state: the action could not be signed and was never processed,
		// so the wallet aborted it - it is gone from the active actions and the funding
		// output it had reserved is spendable again.
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(1).
			HasActionsCount(0, fixtures.CreateActionTestLabel)

		thenState.ActionAtIndex(0).
			WithTxID(txFromFaucet.ID().String()).
			WithSatoshis(topUpValue)

		outputs, err := aliceWallet.ListOutputs(t.Context(), fixtures.DefaultWalletListOutputsArgs(), fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.Len(t, outputs.Outputs, 1, "the funding output should be released by the abort")
		require.True(t, outputs.Outputs[0].Spendable)
		require.EqualValues(t, topUpValue, outputs.Outputs[0].Satoshis)
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
		args := fixtures.DefaultWalletCreateActionArgs(
			t,
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
			args := fixtures.DefaultWalletCreateActionArgs(
				t,
				append(
					test.createActionModifiers,
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
			args := fixtures.DefaultWalletCreateActionArgs(
				t,
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

// By the time SignAction resolves the txid-only stubs in its reply the
// transaction has already been broadcast, so a stub the wallet cannot resolve
// must degrade to the BEEF storage sent rather than fail the action - a caller
// that gets an error for a live transaction either rebuilds and double-spends
// its own inputs or writes off an operation that succeeded.
//
// Same fallback, and same reasoning, as TestCreateActionResolvesKnownTxidStubs'
// counterpart in CreateAction.
func (s *WalletTestSuite) TestSignActionFallsBackWhenAStubCannotBeResolved() {
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

	// and: the party holds the funding transaction as a bare txid under a proof,
	// which is what the known-txid list is built from but not what resolving a
	// reply accepts - so the wallet advertises a transaction it cannot produce.
	funding := txFromFaucet.ID()
	stranded := transaction.NewBeef()
	stranded.MergeTxidOnly(funding)
	proof := testutils.MockValidMerklePath(t, funding.String(), 800_000)
	stranded.MergeBump(&proof)
	strandedBytes, err := stranded.Bytes()
	require.NoError(t, err)
	require.NoError(t, aliceWallet.GetBeefParty().MergeBeefFromParty(t.Context(), storagePartyID, strandedBytes))

	// and:
	args := fixtures.DefaultWalletCreateActionArgs(
		t,
		walletargs.WithInput(input),
		walletargs.WithSignAndProcess(false),
	)

	createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
	require.NoError(t, err)

	// when:
	signActionResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
		Reference: createActionResult.SignableTransaction.Reference,
		Spends: map[uint32]sdk.SignActionSpend{
			0: {UnlockingScript: input.UnlockingScript().Bytes()},
		},
	}, fixtures.DefaultOriginator)

	// then: the action completes on the BEEF it already had, for a transaction
	// that is by now on the network
	require.NoError(t, err, "an unresolvable stub must not fail an already broadcast action")
	require.NotNil(t, signActionResult)
	require.NotEmpty(t, signActionResult.Tx, "the caller asked for the transaction, so it must still be returned")
}
