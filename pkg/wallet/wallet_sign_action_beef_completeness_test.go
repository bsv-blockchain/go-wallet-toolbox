package wallet_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

// The signed transaction a sign action returns is what a caller hands to a
// counterparty, for example to internalize, so it has to stand on its own: every
// input's ancestry down to a proven transaction, with no txid-only stubs the
// receiver cannot resolve.
func TestSignAction_ReturnedBEEFIsComplete(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeSQLite)
	given.Faucet(aliceWallet).TopUp(testValueForFunding)

	createResult, err := aliceWallet.CreateAction(t.Context(),
		fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false)),
		fixtures.DefaultOriginator)
	require.NoError(t, err)

	signResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
		Reference: createResult.SignableTransaction.Reference,
	}, fixtures.DefaultOriginator)
	require.NoError(t, err)

	beef, subjectTxID, err := transaction.NewBeefFromAtomicBytes(signResult.Tx)
	require.NoError(t, err)
	subject := beef.FindTransactionByHash(subjectTxID)
	require.NotNil(t, subject)

	for _, input := range subject.Inputs {
		entry, ok := beef.Transactions[*input.SourceTXID]
		require.True(t, ok, "input source %s missing from the BEEF", input.SourceTXID)
		assert.NotEqual(t, transaction.TxIDOnly, entry.DataFormat, "input source %s is a bare txid stub", input.SourceTXID)
	}
	assert.True(t, beef.IsValid(false), "signed BEEF must be valid without txid-only entries")
}

// A caller that trusts storage and discards the signed transaction opts out of
// that guarantee, and storage skips the ancestry walk for it. The sign action
// must still complete and broadcast: broadcast rebuilds its own BEEF.
func TestSignAction_TrustSelfReturnTXIDOnly_SignsAndBroadcasts(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeSQLite)
	given.Faucet(aliceWallet).TopUp(testValueForFunding)

	args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false))
	args.Options.TrustSelf = sdk.TrustSelfKnown
	args.Options.ReturnTXIDOnly = new(true)

	createResult, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
	require.NoError(t, err)

	// and: the signable transaction still lets the caller compute each input's
	// sighash - which needs the source output of every input
	require.NotNil(t, createResult.SignableTransaction)
	signableBeef, signableTxID, err := transaction.NewBeefFromAtomicBytes(createResult.SignableTransaction.Tx)
	require.NoError(t, err)
	signable := signableBeef.FindAtomicTransactionByHash(signableTxID)
	require.NotNil(t, signable)
	require.NotEmpty(t, signable.Inputs)
	for vin, input := range signable.Inputs {
		require.NotNil(t, input.SourceTxOutput(), "input %d has no source output to sign against", vin)
		_, err = signable.CalcInputSignatureHash(uint32(vin), sighash.AllForkID) //nolint:gosec // test-sized index
		require.NoError(t, err, "input %d sighash", vin)
	}

	signResult, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
		Reference: createResult.SignableTransaction.Reference,
	}, fixtures.DefaultOriginator)
	require.NoError(t, err)

	assert.False(t, signResult.Txid.IsEqual(nil), "signed txid must be returned")
	assert.Empty(t, signResult.Tx, "ReturnTXIDOnly: no transaction data is returned")
	for _, sendWith := range signResult.SendWithResults {
		assert.NotEqual(t, sdk.ActionResultStatusFailed, sendWith.Status)
	}

	testabilities.ThenWalletState(t, aliceWallet).HasActionsCount(2)
}

// A wallet spending an output its own storage holds does not need to ship that
// output's BEEF: the wallet trusts its own storage (TrustSelf=known is the
// wallet default), so storage resolves the input itself. This is the
// shape of a token burn or local transfer - look the output up, spend it as a
// caller-supplied input - and the chain runs unconfirmed for longer than the
// ancestry hydration bound, so a path that walked it generation by generation
// would fail part-way.
func TestSignAction_SpendsOwnOutputWithoutInputBEEF_AcrossUnconfirmedChain(t *testing.T) {
	const (
		generations = 12
		tokenSats   = 1000
		basket      = "token"
	)

	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeSQLite)
	given.Faucet(aliceWallet).TopUp(testValueForFunding)

	tokenScript := script.Script{script.Op3, script.OpEQUAL}
	tokenOutput := []sdk.CreateActionOutput{{
		LockingScript:     tokenScript.Bytes(),
		Satoshis:          tokenSats,
		OutputDescription: "token",
		Basket:            basket,
	}}

	// given: a token output in the wallet's own basket (the mint)
	// Delayed broadcast: each spend can arrive before the transaction it spends
	// has been sent.
	mintArgs := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithDelayedBroadcast())
	mintArgs.Outputs = tokenOutput
	mintArgs.Options.TrustSelf = sdk.TrustSelfKnown
	mintArgs.Options.ReturnTXIDOnly = new(true)
	mint, err := aliceWallet.CreateAction(t.Context(), mintArgs, fixtures.DefaultOriginator)
	require.NoError(t, err)
	prev := mint.Txid

	for gen := 1; gen <= generations; gen++ {
		// when: the previous token output is spent with no inputBEEF at all
		args := fixtures.DefaultWalletCreateActionArgs(t, walletargs.WithSignAndProcess(false), walletargs.WithDelayedBroadcast())
		args.Inputs = []sdk.CreateActionInput{{
			Outpoint:              transaction.Outpoint{Txid: prev, Index: 0},
			InputDescription:      "token input",
			UnlockingScriptLength: 1,
		}}
		args.Outputs = tokenOutput
		args.Options.TrustSelf = sdk.TrustSelfKnown
		args.Options.ReturnTXIDOnly = new(true)

		created, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err, "generation %d createAction", gen)
		require.NotNil(t, created.SignableTransaction)

		// then: the signable transaction carries the token input's source output,
		// which is what the caller signs against
		signableBeef, signableTxID, err := transaction.NewBeefFromAtomicBytes(created.SignableTransaction.Tx)
		require.NoError(t, err)
		signable := signableBeef.FindAtomicTransactionByHash(signableTxID)
		require.NotNil(t, signable)
		source := signable.Inputs[0].SourceTxOutput()
		require.NotNil(t, source, "generation %d: token input has no source output", gen)
		assert.Equal(t, uint64(tokenSats), source.Satoshis)
		assert.Equal(t, tokenScript.Bytes(), source.LockingScript.Bytes())

		signed, err := aliceWallet.SignAction(t.Context(), sdk.SignActionArgs{
			Reference: created.SignableTransaction.Reference,
			Spends:    map[uint32]sdk.SignActionSpend{0: {UnlockingScript: script.Script{script.Op3}}},
			Options:   &sdk.SignActionOptions{AcceptDelayedBroadcast: new(true)},
		}, fixtures.DefaultOriginator)
		require.NoError(t, err, "generation %d signAction", gen)
		for _, sendWith := range signed.SendWithResults {
			require.NotEqual(t, sdk.ActionResultStatusFailed, sendWith.Status, "generation %d broadcast", gen)
		}

		prev = signed.Txid
	}
}
