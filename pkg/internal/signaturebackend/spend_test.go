package signaturebackend_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	crypto "github.com/bsv-blockchain/go-sdk/primitives/hash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/script/interpreter"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// p2pkhSpend builds a one-input transaction spending a P2PKH output of key,
// signed with whichever backend is installed.
func p2pkhSpend(t *testing.T, key *ec.PrivateKey) *transaction.Transaction {
	t.Helper()
	pub := key.PubKey().Compressed()

	lock := &script.Script{}
	require.NoError(t, lock.AppendOpcodes(script.OpDUP, script.OpHASH160))
	require.NoError(t, lock.AppendPushData(crypto.Hash160(pub)))
	require.NoError(t, lock.AppendOpcodes(script.OpEQUALVERIFY, script.OpCHECKSIG))

	var prev chainhash.Hash
	prev[0] = 1

	tx := transaction.NewTransaction()
	tx.AddInputWithOutput(&transaction.TransactionInput{
		SourceTXID:     &prev,
		SequenceNumber: 0xffffffff,
	}, &transaction.TransactionOutput{Satoshis: 10_000, LockingScript: lock})
	tx.AddOutput(&transaction.TransactionOutput{Satoshis: 9_000, LockingScript: lock})

	digest, err := tx.CalcInputSignatureHash(0, sighash.AllForkID)
	require.NoError(t, err)
	sig, err := key.Sign(digest)
	require.NoError(t, err)

	unlock := &script.Script{}
	require.NoError(t, unlock.AppendPushData(append(sig.Serialize(), byte(sighash.AllForkID))))
	require.NoError(t, unlock.AppendPushData(pub))
	tx.Inputs[0].UnlockingScript = unlock
	return tx
}

// verify executes the input script the way storage's script verifier does.
func verify(tx *transaction.Transaction) error {
	return interpreter.NewEngine().Execute(
		interpreter.WithTx(tx, 0, tx.Inputs[0].SourceTxOutput()),
		interpreter.WithForkID(),
		interpreter.WithAfterGenesis(),
	)
}

func TestInstalledBackend_VerifiesValidSpendAndRejectsTampered(t *testing.T) {
	key, err := ec.NewPrivateKey()
	require.NoError(t, err)

	tx := p2pkhSpend(t, key)
	require.NoError(t, verify(tx), "a correctly signed spend must verify")

	tx.Outputs[0].Satoshis = 8_999
	assert.Error(t, verify(tx), "a spend altered after signing must be rejected")
}
