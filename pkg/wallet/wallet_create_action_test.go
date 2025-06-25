package wallet_test

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/asserttx"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testValueForFunding = 99904

func TestWalletCreateActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.CreateActionArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: func() sdk.CreateActionArgs {
				return fixtures.DefaultWalletCreateActionArgs(t)
			},
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: func() sdk.CreateActionArgs {
				return fixtures.DefaultWalletCreateActionArgs(t)
			},
		},
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

func (s *WalletTestSuite) TestWalletCreateActionNewWithNoSend() {
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
		assert.NoError(t, err)

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
			WithTxID(txFromFaucet.ID()).
			WithSatoshis(topUpValue)

		const fee = 2
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithoutTxID(). // NOTE: Signable transaction does not have txid in DB yet.
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee)

		thenCreatedAction.OutputAtIndex(0).
			ListActionsAlignsListOutputs().
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(false).
			WithBasket("")
	})
}

func (s *WalletTestSuite) TestWalletCreateActionNewWithNoSendAndProvidedInput() {
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

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)

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
			WithTxID(txFromFaucet.ID()).
			WithSatoshis(topUpValue)

		const fee = 2
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithoutTxID(). // NOTE: Signable transaction does not have txid in DB yet.
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee + inputValue)

		thenCreatedAction.OutputAtIndex(0).
			ListActionsAlignsListOutputs().
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(false).
			WithBasket("")
	})
}

func (s *WalletTestSuite) TestWalletCreateActionNewWithSend() {
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
		assert.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")

		// and:
		assert.NotEmpty(t, result.Txid, "Wallet result should have transaction id")
		assert.NotEmpty(t, result.Tx, "Wallet result should have transaction bytes")
		assert.Len(t, result.SendWithResults, 1, "Wallet result should have single send with results")
		assert.Equal(t, result.SendWithResults[0].Txid, result.Txid, "Wallet result should have same txid as the one from send with result")
		assert.Equal(t, result.SendWithResults[0].Status, sdk.ActionResultStatusUnproven, "Wallet send with result should have unproven status")

		// and check the state of wallet:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(2).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		thenState.ActionAtIndex(0).
			WithTxID(txFromFaucet.ID()).
			WithSatoshis(topUpValue)

		const fee = 2
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithTxID(result.Txid.String()).
			WithStatus(sdk.ActionStatusUnproven).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee) // Pay attention that this is negative value (user spends balance).

		thenCreatedAction.OutputAtIndex(0).
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(false).
			WithBasket("")
	})
}

func (s *WalletTestSuite) TestWalletCreateActionNewWithSendAndTXIDOnly() {
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

func (s *WalletTestSuite) TestWalletCreateActionNewWithSendAndProvidedInput() {
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

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithInput(input))

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")

		// and:
		assert.NotEmpty(t, result.Txid, "Wallet result should have transaction id")
		assert.NotEmpty(t, result.Tx, "Wallet result should have transaction bytes")
		assert.Len(t, result.SendWithResults, 1, "Wallet result should have single send with results")
		assert.Equal(t, result.SendWithResults[0].Txid, result.Txid, "Wallet result should have same txid as the one from send with result")
		assert.Equal(t, result.SendWithResults[0].Status, sdk.ActionResultStatusUnproven, "Wallet send with result should have unproven status")

		// and check the state of wallet:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(1).
			HasActionsCount(1, fixtures.CreateActionTestLabel)

		const fee = 2
		thenCreatedAction := thenState.ActionAtIndex(0)
		thenCreatedAction.
			WithTxID(result.Txid.String()).
			WithStatus(sdk.ActionStatusUnproven).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(inputValue - int64(args.Outputs[0].Satoshis) - fee) // Pay attention that this is positive value, because provided input must be higher than output to fund the transaction.
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
		assert.Error(t, err)
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
		assert.Error(t, err)
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
		given.Services().AllDown()

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithInput(input))
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.Nil(t, result, "Wallet shouldn't return result when all services are down")
		require.Error(t, err, "Wallet should return error when not delayed broadcast failed")

		// and:
		// TODO: replacace with better assertions for error - when we will have custom type for it
		assert.ErrorContains(t, err, "undelayed result require review")
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

		// and:
		given.Services().AllDown()

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInput(input),
			walletargs.WithSignAndProcess(false),
		)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err, "Wallet should not fail for signable transaction when all services are down")
		require.NotNil(t, result, "Wallet should return signable transaction when all services are down")

	})
}
