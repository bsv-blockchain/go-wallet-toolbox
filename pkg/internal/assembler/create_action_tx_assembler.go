package assembler

import (
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/go-softwarelab/common/pkg/to"
)

type CreateActionTransactionAssembler struct {
	tx                 *transaction.Transaction
	keyDeriver         *wallet.KeyDeriver
	createActionResult *wdk.StorageCreateActionResult
	providedInputs     []wallet.CreateActionInput
	inputBEEF          *transaction.Beef
}

func NewCreateActionTransactionAssembler(keyDeriver *wallet.KeyDeriver, providedInputs []wallet.CreateActionInput, createActionResult *wdk.StorageCreateActionResult) *CreateActionTransactionAssembler {
	return &CreateActionTransactionAssembler{
		keyDeriver:         keyDeriver,
		createActionResult: createActionResult,
		providedInputs:     providedInputs,
		tx:                 &transaction.Transaction{},
	}
}

func (a *CreateActionTransactionAssembler) Assemble() (*AssembledTransaction, error) {
	err := a.parseInputBEEF()
	if err != nil {
		return nil, err
	}

	a.fillTransactionHeader()

	err = a.fillInputs()
	if err != nil {
		return nil, err
	}

	err = a.fillOutputs()
	if err != nil {
		return nil, err
	}

	return &AssembledTransaction{Transaction: a.tx, inputBEEF: a.inputBEEF}, nil
}

func (a *CreateActionTransactionAssembler) fillTransactionHeader() {
	a.tx.Version = a.createActionResult.Version
	a.tx.LockTime = a.createActionResult.LockTime
}

func (a *CreateActionTransactionAssembler) fillInputs() error {
	inputs := seq.MapOrErr(a.inputs(), a.toTxInput)
	err := seqerr.ForEach(inputs, a.tx.AddInput)
	if err != nil {
		return fmt.Errorf("failed to build transaction inputs from storage result: %w", err)
	}
	return nil
}

func (a *CreateActionTransactionAssembler) fillOutputs() error {
	outputs := seq.MapOrErr(a.outputs(), a.toTxOutput)
	err := seqerr.ForEach(outputs, a.tx.AddOutput)
	if err != nil {
		return fmt.Errorf("failed to build transaction outputs from storage result: %w", err)
	}
	return nil
}

func (a *CreateActionTransactionAssembler) inputs() iter.Seq[*wdk.StorageCreateTransactionSdkInput] {
	return seq.SortBy(seq.FromSlice(a.createActionResult.Inputs), func(it *wdk.StorageCreateTransactionSdkInput) int { return it.Vin })
}

func (a *CreateActionTransactionAssembler) outputs() iter.Seq[*wdk.StorageCreateTransactionSdkOutput] {
	return seq.SortBy(seq.FromSlice(a.createActionResult.Outputs), func(it *wdk.StorageCreateTransactionSdkOutput) uint32 {
		return it.Vout
	})
}

func (a *CreateActionTransactionAssembler) toTxInput(it *wdk.StorageCreateTransactionSdkInput) (*transaction.TransactionInput, error) {
	sourceTxID, err := chainhash.NewHashFromHex(it.SourceTxID)
	if err != nil {
		return nil, fmt.Errorf("cannot parse input %d source txid returned from storage: %w", it.Vin, err)
	} else if sourceTxID == nil {
		return nil, fmt.Errorf("cannot parsed input %d  source tx id is nil: %w", it.Vin, err)
	}

	if a.isInputFromArgs(it) {
		return a.toTxInputFromArgs(it, sourceTxID)
	}

	return a.toTxInputFromManagedInput(it, sourceTxID)

}

func (a *CreateActionTransactionAssembler) toTxInputFromManagedInput(it *wdk.StorageCreateTransactionSdkInput, sourceTxID *chainhash.Hash) (*transaction.TransactionInput, error) {
	if it.Type != wdk.OutputTypeP2PKH {
		return nil, fmt.Errorf("unexpected locking script type %q on input %d managed by storage", it.Type, it.Vin)
	}

	input := &transaction.TransactionInput{
		SourceTXID:       sourceTxID,
		SourceTxOutIndex: it.SourceVout,
		SequenceNumber:   transaction.DefaultSequenceNumber,
	}

	var err error
	if it.SourceTransaction != nil {
		input.SourceTransaction, err = transaction.NewTransactionFromBytes(it.SourceTransaction)
		if err != nil {
			return nil, fmt.Errorf("cannot parse source transaction on input %d returned from storage: %w", it.Vin, err)
		}
	} else {
		lockingScript, err := script.NewFromHex(it.SourceLockingScript)
		if err != nil {
			return nil, fmt.Errorf("cannot parse input %d locking script: %w", it.Vin, err)
		}

		satoshis, err := to.UInt64(it.SourceSatoshis)
		if err != nil {
			return nil, fmt.Errorf("cannot convert input %d source satoshis to uint64: %w", it.Vin, err)
		}

		input.SetSourceTxOutput(&transaction.TransactionOutput{
			Satoshis:      satoshis,
			LockingScript: lockingScript,
			Change:        false,
		})
	}

	senderIdentityKey := to.ValueOrGet(it.SenderIdentityKey, a.keyDeriver.IdentityKeyHex)

	// TODO: in TS they don't create unlocking script template at this stage, but rather create it later
	// 	BUT they need then store the KeyID for each input in the wallet instance
	//  or need to query for those inputs the storage
	//  for now (until SignAction is implemented) we create unlocking script template here.
	input.UnlockingScriptTemplate, err = brc29.Unlock(
		brc29.PubHex(senderIdentityKey),
		brc29.KeyID{
			DerivationPrefix: to.Value(it.DerivationPrefix),
			DerivationSuffix: to.Value(it.DerivationSuffix),
		},
		a.keyDeriver,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create unlocking template for input %d: %w", it.Vin, err)
	}

	return input, nil
}

func (a *CreateActionTransactionAssembler) toTxInputFromArgs(it *wdk.StorageCreateTransactionSdkInput, sourceTxID *chainhash.Hash) (*transaction.TransactionInput, error) {
	if it.Vin < 0 {
		return nil, fmt.Errorf("unexpected negative input index %d", it.Vin)
	}

	argsInput := a.providedInputs[it.Vin]
	if !argsInput.Outpoint.Txid.Equal(*sourceTxID) || argsInput.Outpoint.Index != it.SourceVout {
		return nil, fmt.Errorf("unexpected input (outpoint: %s.%d) on index %d", it.SourceTxID, it.SourceVout, it.Vin)
	}

	sourceTx := a.inputBEEF.FindTransaction(it.SourceTxID)

	return &transaction.TransactionInput{
		SourceTXID:        &argsInput.Outpoint.Txid,
		SourceTxOutIndex:  argsInput.Outpoint.Index,
		UnlockingScript:   script.NewFromBytes(argsInput.UnlockingScript),
		SequenceNumber:    to.Value(argsInput.SequenceNumber),
		SourceTransaction: sourceTx,
	}, nil
}

func (a *CreateActionTransactionAssembler) isInputFromArgs(it *wdk.StorageCreateTransactionSdkInput) bool {
	return len(a.providedInputs) > it.Vin
}

func (a *CreateActionTransactionAssembler) parseInputBEEF() error {
	inputBEEF, err := transaction.NewBeefFromBytes(a.createActionResult.InputBeef)
	if err != nil {
		return fmt.Errorf("cannot parse inputBeef: %w", err)
	}
	a.inputBEEF = inputBEEF
	return nil
}

func (a *CreateActionTransactionAssembler) toTxOutput(it *wdk.StorageCreateTransactionSdkOutput) (*transaction.TransactionOutput, error) {
	var err error
	isChange := it.ProvidedBy == wdk.ProvidedByStorage && it.Purpose == "change"

	var lockingScript *script.Script
	if isChange {
		lockingScript, err = a.changeLockingScript(it)
		if err != nil {
			return nil, err
		}
	} else {
		lockingScript, err = script.NewFromHex(it.LockingScript.String())
		if err != nil {
			return nil, fmt.Errorf("cannot parse output %d locking script: %w", it.Vout, err)
		}
	}

	return &transaction.TransactionOutput{
		Satoshis:      uint64(it.Satoshis),
		LockingScript: lockingScript,
		Change:        isChange,
	}, nil
}

func (a *CreateActionTransactionAssembler) changeLockingScript(it *wdk.StorageCreateTransactionSdkOutput) (*script.Script, error) {
	keyID := brc29.KeyID{
		DerivationPrefix: a.createActionResult.DerivationPrefix,
		DerivationSuffix: to.Value(it.DerivationSuffix),
	}

	err := keyID.Validate()
	if err != nil {
		return nil, fmt.Errorf("cannot create change locking script for output %d: %w", it.Vout, err)
	}

	lockingScript, err := brc29.LockForCounterparty(a.keyDeriver, keyID, a.keyDeriver)
	if err != nil {
		return nil, fmt.Errorf("cannot create change locking script for output %d: %w", it.Vout, err)
	}
	return lockingScript, nil
}
