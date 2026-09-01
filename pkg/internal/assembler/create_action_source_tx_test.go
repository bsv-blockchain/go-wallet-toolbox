package assembler_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// A caller-provided input needs its parent transaction to assemble, and until
// now the only place the assembler looked was the inputBeef echoed back in the
// storage response. That made the persisted blob load-bearing for assembly: a
// caller could not trim the ancestry it ships even when storage already held
// every parent, because assembly would then fail with
// "every signable transaction input must have a source transaction".
//
// Storage populates StorageCreateTransactionSdkInput.SourceTransaction for
// exactly these inputs, so the assembler can fall back to it.

// givenParentTransaction returns a spendable parent and its serialised bytes.
func givenParentTransaction(t *testing.T) (*transaction.Transaction, []byte) {
	t.Helper()

	lockingScript, err := script.NewFromHex("76a914aabbccddeeff00112233445566778899aabbccdd88ac")
	require.NoError(t, err)

	parent := transaction.NewTransaction()
	prev, err := chainhash.NewHashFromHex(
		"0000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(t, err)
	parent.AddInput(&transaction.TransactionInput{
		SourceTXID:       prev,
		SourceTxOutIndex: 0,
		SequenceNumber:   transaction.DefaultSequenceNumber,
	})
	parent.AddOutput(&transaction.TransactionOutput{Satoshis: 1000, LockingScript: lockingScript})

	return parent, parent.Bytes()
}

// givenResultWithProvidedInput builds the storage response for a single
// caller-provided input, with whatever inputBeef the caller is meant to have
// shipped back.
func givenResultWithProvidedInput(
	t *testing.T,
	parent *transaction.Transaction,
	parentRaw []byte,
	inputBeef []byte,
	attachSourceTx bool,
) (*wdk.StorageCreateActionResult, []sdk.CreateActionInput) {
	t.Helper()

	txid := parent.TxID()
	unlockingLen := new(primitives.PositiveInteger)
	*unlockingLen = 108

	in := &wdk.StorageCreateTransactionSdkInput{
		Vin:                   0,
		SourceTxID:            txid.String(),
		SourceVout:            0,
		SourceSatoshis:        1000,
		SourceLockingScript:   parent.Outputs[0].LockingScript.String(),
		UnlockingScriptLength: unlockingLen,
		ProvidedBy:            wdk.ProvidedByYou,
		Type:                  wdk.OutputTypeCustom,
	}
	if attachSourceTx {
		in.SourceTransaction = parentRaw
	}

	result := &wdk.StorageCreateActionResult{
		InputBeef: inputBeef,
		Inputs:    []*wdk.StorageCreateTransactionSdkInput{in},
		Version:   1,
		Reference: "ref",
	}

	provided := []sdk.CreateActionInput{{
		Outpoint:              transaction.Outpoint{Txid: *txid, Index: 0},
		InputDescription:      "provided input",
		UnlockingScriptLength: 108,
	}}

	return result, provided
}

func TestAssemblerFallsBackToStorageSourceTransaction(t *testing.T) {
	keyDeriver := givenKeyDeriver(t, testusers.Alice)
	parent, parentRaw := givenParentTransaction(t)

	t.Run("assembles with an empty inputBeef when storage supplied the parent", func(t *testing.T) {
		// given: the caller shipped no ancestry at all
		result, provided := givenResultWithProvidedInput(t, parent, parentRaw, nil, true)

		// when:
		assembled, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, provided, result).Assemble()

		// then:
		require.NoError(t, err)
		require.Len(t, assembled.Inputs, 1)
		require.NotNil(t, assembled.Inputs[0].SourceTransaction,
			"the input must carry a source transaction or the atomic BEEF cannot be built")
		require.Equal(t, parent.TxID().String(), assembled.Inputs[0].SourceTransaction.TxID().String())
	})

	t.Run("the assembled transaction can still produce an atomic BEEF", func(t *testing.T) {
		// This is the check that actually mattered: ToAtomicBEEF rejects any
		// signable input whose source transaction is nil.
		result, provided := givenResultWithProvidedInput(t, parent, parentRaw, nil, true)

		assembled, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, provided, result).Assemble()
		require.NoError(t, err)

		beef, err := assembled.ToAtomicBEEF(false)
		require.NoError(t, err)
		require.NotEmpty(t, beef)
	})

	t.Run("prefers the inputBeef when it carries the parent", func(t *testing.T) {
		// Existing behaviour must be unchanged where the BEEF has the parent:
		// the fallback engages only where assembly would previously have failed.
		beef := transaction.NewBeefV2()
		_, err := beef.MergeRawTx(parentRaw, nil)
		require.NoError(t, err)
		beefBytes, err := beef.Bytes()
		require.NoError(t, err)

		result, provided := givenResultWithProvidedInput(t, parent, parentRaw, beefBytes, false)

		assembled, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, provided, result).Assemble()
		require.NoError(t, err)
		require.Equal(t, parent.TxID().String(), assembled.Inputs[0].SourceTransaction.TxID().String())
	})

	t.Run("names the outpoint when neither route has the parent", func(t *testing.T) {
		result, provided := givenResultWithProvidedInput(t, parent, parentRaw, nil, false)

		_, err := assembler.NewCreateActionTransactionAssembler(keyDeriver, provided, result).Assemble()
		require.Error(t, err)
		require.Contains(t, err.Error(), "no source transaction for input")
		require.Contains(t, err.Error(), parent.TxID().String())
	})
}
