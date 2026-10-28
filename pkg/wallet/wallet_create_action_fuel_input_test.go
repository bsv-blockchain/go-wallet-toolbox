package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

// TestWalletCreateAction_FuelInputCoversFee checks the FuelKeeper consumer shape:
// an external, mined, puzzle-locked input with its unlocking script supplied up
// front ("fuel") must pay the fee on its own, so the wallet takes nothing from
// its change basket even though the basket is funded.
func (s *WalletTestSuite) TestWalletCreateAction_FuelInputCoversFee() {
	const fuelSats = 50
	const tokenSats = 1

	tokenOutput := []sdk.CreateActionOutput{{
		LockingScript:     []byte{script.Op3, script.OpEQUAL},
		Satoshis:          tokenSats,
		OutputDescription: "token output",
	}}

	requireOnlyInputs := func(t require.TestingT, beef []byte, expected ...transaction.Outpoint) {
		tx, err := transaction.NewTransactionFromBEEF(beef)
		require.NoError(t, err)
		require.Len(t, tx.Inputs, len(expected), "wallet must not add change-basket inputs")
		for _, in := range tx.Inputs {
			require.Contains(t, expected, transaction.Outpoint{Txid: *in.SourceTXID, Index: in.SourceTxOutIndex})
		}
	}

	s.Run("mint shape: fuel input plus one token output", func() {
		t := s.T()
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		_, _ = given.Faucet(aliceWallet).TopUp(testValueForFunding)

		fuel := given.InputForUser(testusers.Alice).WithSatoshis(fuelSats)
		given.Services().BHS().OnMerkleRootVerifyResponse(fuel.BlockHeight(), fuel.MerklePath().Hex(), "CONFIRMED")

		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithInput(fuel))
		args.Outputs = tokenOutput

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		require.NoError(t, err)
		requireOnlyInputs(t, result.Tx, fuel.CreateActionInput().Outpoint)
	})

	s.Run("burn shape: fuel input, no outputs", func() {
		t := s.T()
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		_, _ = given.Faucet(aliceWallet).TopUp(testValueForFunding)

		fuel := given.InputForUser(testusers.Alice).WithSatoshis(fuelSats)
		given.Services().BHS().OnMerkleRootVerifyResponse(fuel.BlockHeight(), fuel.MerklePath().Hex(), "CONFIRMED")

		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithInput(fuel), walletargs.WithNoOutputs())

		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)

		require.NoError(t, err)
		requireOnlyInputs(t, result.Tx, fuel.CreateActionInput().Outpoint)
	})

	s.Run("transfer shape: fuel input plus a token input signed later", func() {
		t := s.T()
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)
		_, _ = given.Faucet(aliceWallet).TopUp(testValueForFunding)

		// The token is the wallet's own output, as in EW, where a transfer spends
		// a pushdrop from the wallet's basket and signs it after CreateAction.
		mintArgs := fixtures.DefaultWalletCreateActionArgs(t)
		mintArgs.Outputs = tokenOutput
		minted, err := aliceWallet.CreateAction(t.Context(), mintArgs, fixtures.DefaultOriginator)
		require.NoError(t, err)
		tokenOutpoint := transaction.Outpoint{Txid: minted.Txid, Index: 0}

		fuel := given.InputForUser(testusers.Alice).WithSatoshis(fuelSats)
		given.Services().BHS().OnMerkleRootVerifyResponse(fuel.BlockHeight(), fuel.MerklePath().Hex(), "CONFIRMED")

		beef, err := transaction.NewBeefFromBytes(fuel.InputBEEFBytes())
		require.NoError(t, err)
		require.NoError(t, beef.MergeBeefBytes(minted.Tx))
		inputBEEF, err := beef.Bytes()
		require.NoError(t, err)

		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithInputs([]sdk.CreateActionInput{
				fuel.CreateActionInput(),
				{Outpoint: tokenOutpoint, InputDescription: "token input", UnlockingScriptLength: 1},
			}),
			walletargs.WithInputBEEF(inputBEEF),
			walletargs.WithSignAndProcess(false),
		)
		args.Outputs = tokenOutput

		created, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, created.SignableTransaction)
		requireOnlyInputs(t, created.SignableTransaction.Tx, fuel.CreateActionInput().Outpoint, tokenOutpoint)

		signed, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
			Reference: created.SignableTransaction.Reference,
			Spends:    map[uint32]sdk.SignActionSpend{1: {UnlockingScript: []byte{script.Op3}}},
		}, fixtures.DefaultOriginator)

		require.NoError(t, err)
		requireOnlyInputs(t, signed.Tx, fuel.CreateActionInput().Outpoint, tokenOutpoint)
	})
}
