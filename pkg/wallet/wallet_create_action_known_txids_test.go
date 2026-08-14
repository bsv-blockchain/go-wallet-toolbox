package wallet_test

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// CreateAction tells storage which transactions this wallet already holds, so
// storage answers with txid-only stubs instead of resending their whole
// ancestry. The saving is on the wire only: the wallet resolves those stubs
// against its beef party before returning, so the caller sees the same
// transaction data it always did.
//
// This is what guards that contract. If the advertise stops firing the test
// still passes but the transfer grows again; if the resolve stops firing the
// caller starts receiving stubs and the test fails.
func (s *WalletTestSuite) TestCreateActionResolvesKnownTxidStubs() {
	t := s.T()

	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(s.StorageType)
	given.Faucet(aliceWallet).TopUp(satoshi.MustFrom(1_000_000))

	newAction := func() []byte {
		args := fixtures.DefaultWalletCreateActionArgs(t,
			walletargs.WithSignAndProcess(true),
			walletargs.WithSatoshisAsFirstOutput(1_000),
		)
		result, err := aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotEmpty(t, result.Tx, "caller asked for the transaction, so it must be returned")
		return result.Tx
	}

	// The first action teaches the beef party about its inputs; from the second
	// on, storage has something it can legitimately send back as a stub.
	first := newAction()
	second := newAction()

	for name, raw := range map[string][]byte{"first": first, "second": second} {
		beef, subject, err := transaction.NewBeefFromAtomicBytes(raw)
		require.NoErrorf(t, err, "%s action: returned BEEF does not parse", name)

		// The subject transaction is always fully present.
		subjectTx := beef.FindTransaction(subject.String())
		require.NotNilf(t, subjectTx, "%s action: subject transaction missing", name)

		// And so is everything it spends - resolved back from the party graph.
		for _, btx := range beef.Transactions {
			assert.NotEqualf(t, transaction.TxIDOnly, btx.DataFormat,
				"%s action: returned a txid-only stub the caller cannot use", name)
		}

		for _, input := range subjectTx.Inputs {
			require.NotNilf(t, input.SourceTXID, "%s action: input without a source txid", name)
			assert.NotNilf(t, beef.FindTransaction(input.SourceTXID.String()),
				"%s action: input source %s missing from the returned BEEF", name, input.SourceTXID)
		}
	}
}

// A caller may supply its own KnownTxids, in which case the wallet does not
// advertise and so never reads the graph. The reply is still merged into it, so
// something else has to bound it — otherwise the graph grows on every action for
// the whole life of the wallet, which is the leak DefaultMaxGraphTxs exists to
// prevent.
func (s *WalletTestSuite) TestCreateActionBoundsGraphWithCallerSuppliedKnownTxids() {
	t := s.T()

	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(s.StorageType)
	given.Faucet(aliceWallet).TopUp(satoshi.MustFrom(1_000_000))

	// Push the shared graph past its cap up front, so one action is enough to
	// show whether the bound is ever applied.
	oversized := transaction.NewBeef()
	for i := range wdk.DefaultMaxGraphTxs + 1 {
		tx := &transaction.Transaction{Version: 1, LockTime: uint32(i)}
		tx.Outputs = append(tx.Outputs, &transaction.TransactionOutput{
			Satoshis:      1000,
			LockingScript: &script.Script{},
		})
		_, err := oversized.MergeTransaction(tx)
		require.NoError(t, err)
	}
	oversizedBytes, err := oversized.Bytes()
	require.NoError(t, err)

	beefParty := aliceWallet.GetBeefParty()
	require.NoError(t, beefParty.MergeBeefFromParty(t.Context(), "storage-seed", oversizedBytes))
	require.Greater(t, graphSize(t, beefParty), wdk.DefaultMaxGraphTxs, "seeding should push the graph past its cap")

	// A non-empty list means the wallet uses the caller's and never advertises,
	// so the advertise-time prune cannot fire. The txid is not in the reply, so
	// nothing comes back as a stub and the result is unaffected.
	args := fixtures.DefaultWalletCreateActionArgs(t,
		walletargs.WithSignAndProcess(true),
		walletargs.WithSatoshisAsFirstOutput(1_000),
	)
	if args.Options == nil {
		args.Options = &sdk.CreateActionOptions{}
	}
	unrelated := &transaction.Transaction{Version: 1, LockTime: 987654}
	args.Options.KnownTxids = []chainhash.Hash{*unrelated.TxID()}

	_, err = aliceWallet.CreateAction(t.Context(), args, fixtures.DefaultOriginator)
	require.NoError(t, err)

	assert.LessOrEqual(t, graphSize(t, beefParty), wdk.DefaultMaxGraphTxs,
		"the graph must be bounded even when the caller supplies its own KnownTxids")
}

func graphSize(t require.TestingT, bp *wdk.BeefParty) int {
	var size int
	err := bp.WithLock(context.Background(), func(beef *transaction.Beef) error {
		size = len(beef.Transactions)
		return nil
	})
	require.NoError(t, err)
	return size
}
