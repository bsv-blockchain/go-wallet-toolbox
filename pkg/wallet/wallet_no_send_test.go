package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func (s *WalletTestSuite) TestWalletCreateActionNoSendChain() {
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
		require.NotEmpty(t, result.NoSendChange) //TODO: Maybe add more assertions here
		firstNoSendChange := result.NoSendChange

		//when creating the second noSend create action, providing noSendChange from the first one:
		args.Options.NoSendChange = firstNoSendChange
		args.Options.NoSend = to.Ptr(true)
		args.Outputs[0].Satoshis = 1
		result, err = aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotEmpty(t, result.NoSendChange) // NOTE: These are OTHER than the first noSendChange
		require.NotNil(t, result.Tx)

		tx, err := transaction.NewTransactionFromBEEF(result.Tx)
		require.NoError(t, err, "Failed to decode transaction from result")

		for vin, input := range tx.Inputs {
			if vin < len(firstNoSendChange) {
				require.Equal(t, firstNoSendChange[vin].Txid.String(), input.SourceTXID.String())
				require.Equal(t, firstNoSendChange[vin].Index, input.SourceTxOutIndex)
			}
		}
	})
}
