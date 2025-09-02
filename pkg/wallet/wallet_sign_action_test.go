package wallet_test

import (
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func (s *WalletTestSuite) TestWalletSignAction_SingleOutputProvided() {
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
		_, _ = given.Faucet(aliceWallet).TopUp(topUpValue)

		// when:
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInput(input),
			walletargs.WithSignAndProcess(false),
		)

		given.Services().BHS().OnMerkleRootVerifyResponse(input.BlockHeight(), input.MerklePath().Hex(), "CONFIRMED")

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
	})
}
