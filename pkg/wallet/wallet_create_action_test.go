package wallet_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgerrors "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	internaltestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/asserttx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const testValueForFunding = 99904

func TestCreateActionOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.CreateActionResult, error) {
			args := fixtures.DefaultWalletCreateActionArgs(t)
			return w.CreateAction(ctx, args, originator)
		},
	)
}

func TestWalletCreateActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.CreateActionArgs
	}{
		"empty args": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				return sdk.CreateActionArgs{}
			},
		},
		"invalid description (too short)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Description = "a"
				return args
			},
		},
		"invalid description (too long)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Description = strings.Repeat("a", 2001)
				return args
			},
		},
		"too big output satoshis": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].Satoshis = primitives.MaxSatoshis + 1
				return args
			},
		},
		"too short output description": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].OutputDescription = "a"
				return args
			},
		},
		"too long output description": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].OutputDescription = strings.Repeat("a", 2001)
				return args
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
			action, err := aliceWallet.CreateAction(t.Context(), test.args(), test.originator)

			// then:
			then.Result(action).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}

func (s *WalletTestSuite) TestWalletCreateAction_SignableTx() {
	s.Run("return signable transaction when signAndProcess is false", func() {
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
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")
		require.NotNil(t, result.SignableTransaction, "Wallet result without sign&process contain signable transaction")
		assert.NotEmpty(t, result.SignableTransaction.Reference, "Signable transaction should have reference")

		// and:
		require.NotEmpty(t, result.SignableTransaction.Tx, "Signable transaction should have transaction bytes")

		thenTx := asserttx.RestoredFromBEEFBytes(t, result.SignableTransaction.Tx)

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
			WithoutTxID(). // NOTE: Signable transaction does not have txid in DB yet.
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee) //nolint:gosec // satoshi value fits in int64

		thenCreatedAction.OutputAtIndex(0).
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(true).
			WithBasket("")
	})

	s.Run("return signable transaction when input unlocking script is not provided", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txInput := given.InputForUser(testusers.Alice).WithNoUnlockingScript().WithSatoshis(topUpValue)
		given.Services().BHS().OnMerkleRootVerifyResponse(txInput.BlockHeight(), txInput.MerklePath().Hex(), "CONFIRMED")

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false), walletargs.WithInput(txInput))

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")
		require.NotNil(t, result.SignableTransaction, "Wallet result without sign&process contain signable transaction")
		assert.NotEmpty(t, result.SignableTransaction.Reference, "Signable transaction should have reference")

		// and:
		require.NotEmpty(t, result.SignableTransaction.Tx, "Signable transaction should have transaction bytes")

		thenTx := asserttx.RestoredFromBEEFBytes(t, result.SignableTransaction.Tx)

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
			HasActionsCount(1).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		const fee = 1
		thenCreatedAction := thenState.ActionAtIndex(0)
		thenCreatedAction.
			WithoutTxID(). // NOTE: Signable transaction does not have txid in DB yet.
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(topUpValue - int64(args.Outputs[0].Satoshis) - fee) //nolint:gosec // satoshi value fits in int64
	})
}

func (s *WalletTestSuite) TestWalletCreateAction_SignableTxAndProvidedInput() {
	s.Run("return signable transaction with provided input when signAndProcess is false", func() {
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

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInput(input),
			walletargs.WithSignAndProcess(false),
		)

		given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")
		require.NotNil(t, result.SignableTransaction, "Wallet result without sign&process contain signable transaction")
		assert.NotEmpty(t, result.SignableTransaction.Reference, "Signable transaction should have reference")

		// and:
		require.NotEmpty(t, result.SignableTransaction.Tx, "Signable transaction should have transaction bytes")

		thenTx := asserttx.RestoredFromBEEFBytes(t, result.SignableTransaction.Tx)

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
			WithoutTxID(). // NOTE: Signable transaction does not have txid in DB yet.
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee + inputValue) //nolint:gosec // satoshi value fits in int64

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

func (s *WalletTestSuite) TestWalletCreateActionNewWithBroadcast() {
	s.Run("create new action", func() {
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
		args := fixtures.DefaultWalletCreateActionArgs(t)

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")

		// and:
		assert.NotEmpty(t, result.Txid, "Wallet result should have transaction id")
		assert.NotEmpty(t, result.Tx, "Wallet result should have transaction bytes")
		assert.Len(t, result.SendWithResults, 1, "Wallet result should have single send with results")
		assert.Equal(t, result.SendWithResults[0].Txid, result.Txid, "Wallet result should have same txid as the one from send with result")
		assert.Equal(t, sdk.ActionResultStatusUnproven, result.SendWithResults[0].Status, "Wallet send with result should have unproven status")

		// and check the state of wallet:
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
			WithTxID(result.Txid.String()).
			WithStatus(sdk.ActionStatusUnproven).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee) //nolint:gosec // satoshi value fits in int64 // Pay attention that this is negative value (user spends balance).

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

func (s *WalletTestSuite) TestWalletCreateActionNewWithDelayedBroadcast() {
	s.Run("delayed broadcast single transaction", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		given.Services().ARC().HoldBroadcasting()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		_, _ = given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithDelayedBroadcast())

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")

		// and:
		assert.NotEmpty(t, result.Txid, "Wallet result should have transaction id")
		assert.NotEmpty(t, result.Tx, "Wallet result should have transaction bytes")
		assert.Len(t, result.SendWithResults, 1, "Wallet result should have single send with results")
		assert.Equal(t, result.Txid, result.SendWithResults[0].Txid, "Wallet result should have same txid as the one from send with result")
		assert.Equal(t, sdk.ActionResultStatusSending, result.SendWithResults[0].Status, "Wallet send with result should have sending status")

		// and check the state of wallet:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithTxID(result.Txid.String()).
			WithStatus(sdk.ActionStatusSending)

		// when, we release broadcasting, the background task should process the action:
		given.Services().ARC().ReleaseBroadcasting()

		thenState.WaitForActionsWithStatusCount(2, sdk.ActionStatusUnproven, 5*time.Second)
	})

	s.Run("delayed broadcast multiple transactions", func() {
		t := s.T()
		const topUpValue = testValueForFunding
		const count = 10

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		given.Services().ARC().HoldBroadcasting()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		faucet := given.Faucet(aliceWallet)
		for range count {
			// NOTE: We need to create multiple UTXOs one for each transaction because they will be reserved until the broadcast is released.
			_, _ = faucet.TopUp(topUpValue)
		}

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithDelayedBroadcast(), walletargs.WithSatoshisAsFirstOutput(1))

		var err error
		results := make([]*sdk.CreateActionResult, count)
		for i := range count {
			results[i], err = aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
			require.NoError(t, err, "Failed to create action %d", i)
		}

		// and:
		for _, result := range results {
			require.NotNil(t, result, "Wallet should return result")

			assert.NotEmpty(t, result.Txid, "Wallet result should have transaction id")
			assert.NotEmpty(t, result.Tx, "Wallet result should have transaction bytes")
			assert.Len(t, result.SendWithResults, 1, "Wallet result should have single send with results")
			assert.Equal(t, result.Txid, result.SendWithResults[0].Txid, "Wallet result should have same txid as the one from send with result")
			assert.Equal(t, sdk.ActionResultStatusSending, result.SendWithResults[0].Status, "Wallet send with result should have sending status")
		}

		// and check the state of wallet:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2*count).
			HasActionsCount(count, fixtures.CreateActionTestLabel)

		thenState.HasActionsWithStatusCount(count, sdk.ActionStatusSending).
			HasActionsWithStatusCount(count, sdk.ActionStatusUnproven)

		// when, we release broadcasting, the background task should process the action:
		given.Services().ARC().ReleaseBroadcasting()

		thenState.WaitForActionsWithStatusCount(2*count, sdk.ActionStatusUnproven, 5*time.Second)
	})

	s.Run("delayed broadcast multiple transactions - check for double spending", func() {
		t := s.T()
		const topUpValue = testValueForFunding
		const count = 100
		const initialUTXOsCount = 50
		// NOTE: While new actions are being created, the background broadcaster sends the previously created ones and generates new UTXOs.
		// Using 50 (half of the total count) should provide enough time for new UTXOs to become available before they are needed.
		// This timing differs between machines, so this number should be used with caution.

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		faucet := given.Faucet(aliceWallet)

		for range initialUTXOsCount {
			_, _ = faucet.TopUp(topUpValue)
		}

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithDelayedBroadcast(), walletargs.WithSatoshisAsFirstOutput(1))

		usedUTXOs := make(map[wdk.OutPoint]bool)

		var err error
		results := make([]*sdk.CreateActionResult, count)
		for i := range count {
			results[i], err = aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
			require.NoError(t, err, "Failed to create action %d", i)

			tx, err := transaction.NewTransactionFromBEEF(results[i].Tx)
			require.NoError(t, err, "Failed to parse transaction for action %d", i)

			for _, input := range tx.Inputs {
				outpoint := wdk.OutPoint{
					TxID: input.SourceTXID.String(),
					Vout: input.SourceTxOutIndex,
				}

				found := usedUTXOs[outpoint]
				require.False(t, found, "Outpoint %s should not be used before", outpoint.String())

				usedUTXOs[outpoint] = true
			}
		}

		// and:
		for _, result := range results {
			require.NotNil(t, result, "Wallet should return result")

			assert.NotEmpty(t, result.Txid, "Wallet result should have transaction id")
			assert.NotEmpty(t, result.Tx, "Wallet result should have transaction bytes")
			assert.Len(t, result.SendWithResults, 1, "Wallet result should have single send with results")
			assert.Equal(t, result.Txid, result.SendWithResults[0].Txid, "Wallet result should have same txid as the one from send with result")
			assert.Equal(t, sdk.ActionResultStatusSending, result.SendWithResults[0].Status, "Wallet send with result should have sending status")
		}

		// and check the state of wallet:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.WaitForActionsWithStatusCount(count+initialUTXOsCount, sdk.ActionStatusUnproven, 5*time.Second)
	})
}

func (s *WalletTestSuite) TestWalletCreateActionNewWithBroadcastAndTXIDOnly() {
	s.Run("create new action with return TXID only", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)
		args.Options.ReturnTXIDOnly = to.Ptr(true)

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result, "Wallet should return result")

		// and:
		require.Empty(t, result.Tx, "Wallet result should not have transaction bytes")
	})
}

func (s *WalletTestSuite) TestWalletCreateActionNewWithBroadcastAndProvidedInput() {
	s.Run("create new action with all funds from provided input", func() {
		t := s.T()
		const inputValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		input := given.InputForUser(testusers.Alice).WithSatoshis(inputValue)

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithInput(input))

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")

		// and:
		assert.NotEmpty(t, result.Txid, "Wallet result should have transaction id")
		assert.NotEmpty(t, result.Tx, "Wallet result should have transaction bytes")
		assert.Len(t, result.SendWithResults, 1, "Wallet result should have single send with results")
		assert.Equal(t, result.SendWithResults[0].Txid, result.Txid, "Wallet result should have same txid as the one from send with result")
		assert.Equal(t, sdk.ActionResultStatusUnproven, result.SendWithResults[0].Status, "Wallet send with result should have unproven status")

		// and check the state of wallet:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(1).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		const fee = 1
		thenCreatedAction := thenState.ActionAtIndex(0)
		thenCreatedAction.
			WithTxID(result.Txid.String()).
			WithStatus(sdk.ActionStatusUnproven).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(inputValue - int64(args.Outputs[0].Satoshis) - fee) //nolint:gosec // satoshi value fits in int64 // Pay attention that this is positive value, because provided input must be higher than output to fund the transaction.
	})
}

func (s *WalletTestSuite) TestWalletCreateActionNewNotEnoughFundsError() {
	s.Run("return error when user have not enough funds", func() {
		t := s.T()
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
	})

	s.Run("return error when user have not enough funds and when sign&process is false", func() {
		t := s.T()
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)
		args.Options.SignAndProcess = to.Ptr(false)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, result)
	})
}

func (s *WalletTestSuite) TestWalletCreateActionWithAllServicesDown() {
	s.Run("return error when want non delayed broadcast and all services are down", func() {
		t := s.T()
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		input := given.InputForUser(testusers.Alice).WithSatoshis(testValueForFunding)

		// and:
		given.BeefVerifier().WillReturnBool(true) // because all services are down, we cannot verify beef, so we assume it's valid

		// and:
		given.Services().AllDown()

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithInput(input))
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.Nil(t, result, "Wallet shouldn't return result when all services are down")
		require.Error(t, err, "Wallet should return error when not delayed broadcast failed")

		// and:
		assert.ErrorIs(t, err, &pkgerrors.CreateActionError{})
	})

	s.Run("return signable transaction when all services are down, but sign and process is false", func() {
		t := s.T()
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		input := given.InputForUser(testusers.Alice).WithSatoshis(testValueForFunding)

		given.BeefVerifier().WillReturnBool(true) // because all services are down, we cannot verify beef, so we assume it's valid

		// and:
		given.Services().AllDown()

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInput(input),
			walletargs.WithSignAndProcess(false),
		)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err, "Wallet should not fail for signable transaction when all services are down")
		require.NotNil(t, result, "Wallet should return signable transaction when all services are down")
	})
}

func (s *WalletTestSuite) TestWalletCreateAction_NoSend_SendWith() {
	s.Run("createAction with 'noSend' then createAction with noSendChange outputs provided, then process with 'sendWith'", func() {
		t := s.T()
		const topUpValue = fixtures.DefaultCreateActionOutputSatoshis + 1

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		_, _ = given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithNoSend(true), walletargs.WithSatoshisAsFirstOutput(1))

		firstResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		assert.NotEmpty(t, firstResult.Txid, "Wallet result should have transaction id")
		assert.NotEmpty(t, firstResult.Tx, "Wallet result should have transaction bytes")
		assert.Empty(t, firstResult.SendWithResults, "Wallet result should have no send with results")

		// when:
		args = fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithNoSendChangeOutputs(firstResult.NoSendChange...),
			walletargs.WithSendWith(firstResult.Txid),
			walletargs.WithSatoshisAsFirstOutput(1),
		)

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.Nil(t, result)
		require.Error(t, err, "send action is not supported when noSendChange outputs are provided")
	})

	s.Run("createAction two 'noSend' actions then createAction with 'sendWith' to broadcast both txs", func() {
		t := s.T()
		const topUpValue = fixtures.DefaultCreateActionOutputSatoshis + 1

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		_, _ = given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithNoSend(true), walletargs.WithSatoshisAsFirstOutput(1))

		firstResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// when:
		args = fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithNoSendChangeOutputs(firstResult.NoSendChange...),
			walletargs.WithNoSend(true),
			walletargs.WithSatoshisAsFirstOutput(1),
		)

		secondResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// when:
		args = fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithoutProvidedOutputs(),
			walletargs.WithSendWith(firstResult.Txid, secondResult.Txid),
		)

		thirdResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		testabilities.SendWithResultsAsserter(thirdResult.SendWithResults).ContainsTxsWithStatus(t, sdk.ActionResultStatusUnproven,
			firstResult.Txid.String(),
			secondResult.Txid.String(),
		)

		// and check db state:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2, fixtures.CreateActionTestLabel)

		thenState.ActionAtIndex(1).
			WithTxID(firstResult.Txid.String()).
			WithStatus(sdk.ActionStatusUnproven)

		thenState.ActionAtIndex(2).
			WithTxID(secondResult.Txid.String()).
			WithStatus(sdk.ActionStatusUnproven)
	})
}

func (s *WalletTestSuite) TestWalletCreateAction_NoSend_SendWith_BroadcastErrorForOne() {
	s.Run("createAction with 'noSend' then attempt to createAction with 'sendWith', but ARC responds double spend for one of the transaction", func() {
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
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithNoSend(true))

		firstResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// and:
		assert.NotEmpty(t, firstResult.Txid, "Wallet result should have transaction id")
		assert.NotEmpty(t, firstResult.Tx, "Wallet result should have transaction bytes")
		assert.Empty(t, firstResult.SendWithResults, "Wallet result should have no send with results")

		// given:
		given.Services().ARC().WhenQueryingTx(firstResult.Txid.String()).WillReturnDoubleSpending()

		// when:
		args = fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSendWith(firstResult.Txid))

		_, err = aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
	})
}

func (s *WalletTestSuite) TestWalletCreateAction_SendWithAsRetryOfProcessAction() {
	s.Run("sendWith create action as retry of broadcasting when process action failed with unprocessed status", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		// and:
		given.ScriptsVerifier().WillReturnError(fmt.Errorf("mock scripts verifier error"))

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t)

		createActionResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		txError := &pkgerrors.TransactionError{}
		require.ErrorAs(t, err, &txError)
		assert.False(t, txError.WrongHash)
		assert.Nil(t, createActionResult)

		// when:
		txIDToRetry := txError.TxID
		given.ScriptsVerifier().DefaultBehavior()
		createActionResult, err = aliceWallet.CreateAction(t.Context(), sdk.CreateActionArgs{
			Options: &sdk.CreateActionOptions{
				SendWith: []chainhash.Hash{txIDToRetry},
			},
			Description: "retry using sendWith",
		}, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, createActionResult)

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
			WithTxID(txIDToRetry.String()).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee) //nolint:gosec // satoshi value fits in int64

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

func (s *WalletTestSuite) TestWalletCreateActionByBobBasedOnAliceCreateAction() {
	s.Run("Alice and Bob use different storages", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.Wallet().WithOwnStorage().ForUser(testusers.Alice)

		// and:
		bobWallet := given.BobWalletWithStorage(s.StorageType)

		// and:
		_, _ = given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		trivialLockingScript := script.Script{
			script.Op3,
			script.OpEQUAL,
		}
		aliceArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithLockingScript(trivialLockingScript))

		firstResult, err := aliceWallet.CreateAction(t.Context(), aliceArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		assert.NotEmpty(t, firstResult.Tx, "Alice wallet should return transaction BEEF bytes")
		assert.NotEmpty(t, firstResult.Txid, "Alice wallet should return transaction ID")

		// when:
		bobsArgs := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInputs([]sdk.CreateActionInput{
				{
					Outpoint:         transaction.Outpoint{Txid: firstResult.Txid, Index: 0},
					InputDescription: "got from alice",
					UnlockingScript:  script.Script{script.Op3},
				},
			}),
			walletargs.WithNoOutputs(),
			walletargs.WithInputBEEF(firstResult.Tx),
		)

		bobsResult, err := bobWallet.CreateAction(t.Context(), bobsArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		internaltestabilities.AssertAtomicBEEFState(t, bobsResult.Tx[:], internaltestabilities.ExpectedBeefTransactionState{
			ID:         firstResult.Txid.String(),
			DataFormat: to.Ptr(transaction.RawTx),
		})

		// and check db state:
		thenState := testabilities.ThenWalletState(t, bobWallet)
		thenState.ActionAtIndex(0).
			WithDescription("test transaction").
			WithSatoshis(41999).
			WithStatus(sdk.ActionStatusUnproven).
			WithNotEmptyTxID()
	})

	s.Run("alice and bob uses the same storage", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		bobWallet := given.BobWalletWithStorage(s.StorageType)

		// and:
		_, _ = given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		aliceArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithLockingScript(script.Script{
			script.Op3,
			script.OpEQUAL,
		}))

		firstResult, err := aliceWallet.CreateAction(t.Context(), aliceArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, firstResult.Tx, "Alice wallet should return transaction BEEF bytes")
		require.NotEmpty(t, firstResult.Txid, "Alice wallet should return transaction ID")

		// when:
		bobsArgs := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInputs([]sdk.CreateActionInput{
				{
					Outpoint:         transaction.Outpoint{Txid: firstResult.Txid, Index: 0},
					InputDescription: "got from alice",
					UnlockingScript:  script.Script{script.Op3},
				},
			}),
			walletargs.WithNoOutputs(),
			walletargs.WithInputBEEF(firstResult.Tx),
		)

		secondResult, err := bobWallet.CreateAction(t.Context(), bobsArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		internaltestabilities.AssertAtomicBEEFState(t, secondResult.Tx[:], internaltestabilities.ExpectedBeefTransactionState{
			ID:         firstResult.Txid.String(),
			DataFormat: to.Ptr(transaction.RawTx),
		})

		// and check db state:
		thenState := testabilities.ThenWalletState(t, bobWallet)
		thenState.ActionAtIndex(0).
			WithDescription("test transaction").
			WithSatoshis(41999).
			WithStatus(sdk.ActionStatusUnproven).
			WithNotEmptyTxID()
	})
}
