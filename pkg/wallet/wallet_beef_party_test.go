package wallet_test

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const storagePartyID = "storage 0320bbfb879bbd6761ecd2962badbb41ba9d60ca88327d78b07ae7141af6b6c810"

func (s *WalletTestSuite) TestWalletBeefParty() {
	s.Run("create new action add tx to beef party", func() {
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

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)

		// and:
		require.NotNil(t, result, "Wallet should return result")

		// and:
		txs, err := aliceWallet.GetBeefParty().GetKnownTxIDsForParty(storagePartyID)
		assert.NoError(t, err, "Should get known tx ids for party without error")
		assert.NotEmpty(t, aliceWallet.GetBeefParty().Transactions, "New transactions should be added to beef party")
		assert.Len(t, txs, 3, "There should be 3 known transactions in beef party for storage party") // Faucet tx + created action tx + change tx
	})
}

func (s *WalletTestSuite) TestWalletBeefPartyGetTrimmedBeef() {
	s.Run("create new action add tx to beef party", func() {
		t := s.T()
		const topUpValue = testValueForFunding

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		txFromFaucet, _ := given.Faucet(aliceWallet).TopUp(topUpValue)

		aliceWalletParty := aliceWallet.GetBeefParty()
		err := aliceWalletParty.AddKnownTxIDsForParty(storagePartyID, txFromFaucet.ID().String())

		trimmed, _ := aliceWallet.GetBeefParty().GetTrimmedBeefForParty(storagePartyID)

		// then:
		assert.NoError(t, err)

		// and:
		require.NotNil(t, trimmed, "Wallet should return result")
		assert.Len(t, trimmed.Transactions, 0, "There should be no new transactions")
	})
}
