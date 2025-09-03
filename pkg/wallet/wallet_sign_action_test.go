package wallet_test

import (
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/asserttx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

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

		const fee = 2
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithTxID(signActionResult.Txid.String()).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) - fee)

		thenCreatedAction.OutputAtIndex(0).
			WithSatoshis(args.Outputs[0].Satoshis).
			WithLockingScript(args.Outputs[0].LockingScript).
			WithOutputIndex(0).
			WithTags(fixtures.CreateActionTestTag).
			WithCustomInstructions(fixtures.CreateActionTestCustomInstructions).
			WithSpendable(false).
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

		const fee = 2
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithTxID(signActionResult.Txid.String()).
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) + inputValue - fee)

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

		const fee = 2
		thenCreatedAction := thenState.ActionAtIndex(1)
		thenCreatedAction.
			WithoutTxID().
			WithDescription(args.Description).
			WithLabels(fixtures.CreateActionTestLabel).
			WithSatoshis(-int64(args.Outputs[0].Satoshis) + inputValue - fee)

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
