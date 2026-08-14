package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/walletargs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
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
