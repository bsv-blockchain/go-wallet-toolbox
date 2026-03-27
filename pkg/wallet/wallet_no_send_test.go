package wallet_test

import (
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func (s *WalletTestSuite) TestWalletCreateActionNoSendChain_HappyPath() {
	s.Run("second create action call should consume all no send changes from the previous create action result", func() {
		t := s.T()
		const inputValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		given.Faucet(aliceWallet).TopUp(inputValue)

		// given - 1st create action result call to obtain no send change funds for the 2nd create action call:
		firstCreateActionCallArgs := fixtures.DefaultWalletCreateActionArgs(t)
		firstCreateActionCallArgs.Outputs[0].Satoshis = 1
		firstCreateActionCallArgs.Options.NoSend = to.Ptr(true)

		// when:
		firstCreateActionResult, err := aliceWallet.CreateAction(t.Context(), firstCreateActionCallArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, firstCreateActionResult.NoSendChange)

		// given - 2nd create action call that should consume all no send changes from the previous call
		firstCreateActionTx := testabilities.CreateTxFromBEEF(t, firstCreateActionResult.Tx)

		var satoshisToSpent uint64
		for _, o := range firstCreateActionTx.Outputs {
			satoshisToSpent += o.Satoshis
		}

		const calculatedFee = 6 // fee_calculator.go -> Calculate

		secondCreateActionCallArgs := firstCreateActionCallArgs
		secondCreateActionCallArgs.Outputs[0].Satoshis = satoshisToSpent - calculatedFee
		secondCreateActionCallArgs.Options.NoSend = to.Ptr(true)
		secondCreateActionCallArgs.Options.NoSendChange = firstCreateActionResult.NoSendChange

		// when:
		secondCreateActionResult, err := aliceWallet.CreateAction(t.Context(), secondCreateActionCallArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, secondCreateActionResult)
		require.NotNil(t, secondCreateActionResult.Tx)

		secondCreateActionTx := testabilities.CreateTxFromBEEF(t, secondCreateActionResult.Tx)
		require.Len(t, secondCreateActionTx.Inputs, len(firstCreateActionResult.NoSendChange))

		for vin, input := range secondCreateActionTx.Inputs {
			require.Equal(t, firstCreateActionResult.NoSendChange[vin].Txid.String(), input.SourceTXID.String())
			require.Equal(t, firstCreateActionResult.NoSendChange[vin].Index, input.SourceTxOutIndex)
		}
	})

	s.Run("create twice noSend create actions, providing noSendChange to the second one", func() {
		t := s.T()
		const inputValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// given not empty storage:
		// NOTE: The purpose of this is to create many UTXOs in the wallet, so that we can test noSendChange.
		given.Faucet(aliceWallet).TopUp(inputValue)
		args := fixtures.DefaultWalletCreateActionArgs(t)
		args.Outputs[0].Satoshis = 1
		_, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)

		// when creating the first noSend create action:
		args.Options.NoSend = to.Ptr(true)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, result.NoSendChange) // TODO: Maybe add more assertions here
		firstNoSendChange := result.NoSendChange

		// when creating the second noSend create action, providing noSendChange from the first one:
		args.Options.NoSendChange = firstNoSendChange
		args.Options.NoSend = to.Ptr(true)
		args.Outputs[0].Satoshis = 1
		result, err = aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, result.NoSendChange) // NOTE: These are OTHER than the first noSendChange
		require.NotNil(t, result.Tx)

		tx := testabilities.CreateTxFromBEEF(t, result.Tx)
		for vin, input := range tx.Inputs {
			if vin < len(firstNoSendChange) {
				require.Equal(t, firstNoSendChange[vin].Txid.String(), input.SourceTXID.String())
				require.Equal(t, firstNoSendChange[vin].Index, input.SourceTxOutIndex)
			}
		}
	})

	s.Run("send with after creating three times noSend create actions using no send changes from the previous create action results", func() {
		t := s.T()
		t.Skip() // TODO: Remove this after handling noSend + sendWith

		const inputValue = testValueForFunding

		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given not empty storage:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		given.Faucet(aliceWallet).TopUp(inputValue)

		args := fixtures.DefaultWalletCreateActionArgs(t)
		args.Outputs[0].Satoshis = 1

		_, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)

		// given - 1st CreateAction with no send true and no send change outpoints:
		firstCreateActionsArgs := args
		firstCreateActionsArgs.Options.NoSend = to.Ptr(true)
		firstCreateActionResult, err := aliceWallet.CreateAction(t.Context(), firstCreateActionsArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, firstCreateActionResult.NoSendChange)

		// given - 2nd CreateAction with no send true, and no send change from the first create action result:
		secondCreateActionsArgs := firstCreateActionsArgs
		secondCreateActionsArgs.Options.NoSend = to.Ptr(true)
		secondCreateActionsArgs.Options.NoSendChange = firstCreateActionResult.NoSendChange
		secondCreateActionsArgs.Outputs[0].Satoshis = 1

		secondCreateActionResult, err := aliceWallet.CreateAction(t.Context(), secondCreateActionsArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, secondCreateActionResult.NoSendChange)

		// given - 3rd CreateAction with no send true, and no send change from the second create action result:
		thirdCreateActionArgs := secondCreateActionsArgs
		thirdCreateActionArgs.Options.NoSend = to.Ptr(true)
		thirdCreateActionArgs.Options.NoSendChange = secondCreateActionResult.NoSendChange
		thirdCreateActionArgs.Outputs[0].Satoshis = 1

		thirdCreateActionsResult, err := aliceWallet.CreateAction(t.Context(), thirdCreateActionArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, thirdCreateActionsResult.NoSendChange)

		// given - 4th - CreateAction with send with (tx1, tx2, tx3) - all transactions from the previous create action results
		forthCreateActionArgs := thirdCreateActionArgs
		forthCreateActionArgs.Options.NoSend = to.Ptr(false)
		forthCreateActionArgs.Options.NoSendChange = nil
		forthCreateActionArgs.Options.SendWith = append(forthCreateActionArgs.Options.SendWith,
			firstCreateActionResult.Txid,
			secondCreateActionResult.Txid,
			thirdCreateActionsResult.Txid,
		)

		forthCreateActionsResult, err := aliceWallet.CreateAction(t.Context(), forthCreateActionArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, forthCreateActionsResult.SendWithResults)
	})
}
