package assembler

import (
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type CreateActionTransactionAssembler struct {
	tx                       *transaction.Transaction
	keyDeriver               *wallet.KeyDeriver
	createActionResult       *wdk.StorageCreateActionResult
	providedInputs           []wallet.CreateActionInput
	providedInputsByOutpoint map[string]wallet.CreateActionInput
	inputBEEF                *transaction.Beef
}

func NewCreateActionTransactionAssembler(keyDeriver *wallet.KeyDeriver, providedInputs []wallet.CreateActionInput, createActionResult *wdk.StorageCreateActionResult) *CreateActionTransactionAssembler {
	byOutpoint := make(map[string]wallet.CreateActionInput, len(providedInputs))
	for _, inp := range providedInputs {
		key := fmt.Sprintf("%s.%d", inp.Outpoint.Txid.String(), inp.Outpoint.Index)
		byOutpoint[key] = inp
	}
	return &CreateActionTransactionAssembler{
		keyDeriver:               keyDeriver,
		createActionResult:       createActionResult,
		providedInputs:           providedInputs,
		providedInputsByOutpoint: byOutpoint,
		tx:                       &transaction.Transaction{},
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

	assembled := &AssembledTransaction{Transaction: a.tx, inputBEEF: a.inputBEEF}
	assembled.initialTxID = assembled.TxID()
	return assembled, nil
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
		var lockingScript *script.Script
		lockingScript, err = script.NewFromHex(it.SourceLockingScript)
		if err != nil {
			return nil, fmt.Errorf("cannot parse input %d locking script: %w", it.Vin, err)
		}

		var satoshis uint64
		satoshis, err = to.UInt64(it.SourceSatoshis)
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
	key := fmt.Sprintf("%s.%d", it.SourceTxID, it.SourceVout)
	argsInput, ok := a.providedInputsByOutpoint[key]
	if !ok {
		return nil, fmt.Errorf("unexpected input (outpoint: %s) not found in provided inputs", key)
	}

	sourceTx, err := a.sourceTransactionFor(it)
	if err != nil {
		return nil, err
	}

	return &transaction.TransactionInput{
		SourceTXID:        sourceTxID,
		SourceTxOutIndex:  argsInput.Outpoint.Index,
		UnlockingScript:   script.NewFromBytes(argsInput.UnlockingScript),
		SequenceNumber:    to.ValueOr(argsInput.SequenceNumber, transaction.DefaultSequenceNumber),
		SourceTransaction: sourceTx,
	}, nil
}

// sourceTransactionFor resolves the parent transaction of a caller-provided
// input, preferring the inputBEEF and falling back to the raw bytes storage
// already attached to the input itself.
//
// The fallback is what lets a caller stop shipping ancestry it does not need to
// ship. Every signable input must carry a SourceTransaction — assembling an
// atomic BEEF fails outright without one — and until now the only place the
// assembler looked was the inputBEEF echoed back in the storage response. That
// made the response BEEF load-bearing for assembly, so a caller could not pass a
// trimmed (or absent) inputBEEF even when storage already held every parent it
// needed: the persisted blob had to stay large purely so this lookup would
// succeed.
//
// Storage populates StorageCreateTransactionSdkInput.SourceTransaction for
// exactly these inputs when IncludeInputSourceRawTxs is set, reading raw_tx
// straight off the known-tx row (see create.go, inputToStorageInput). Preferring
// the BEEF keeps existing behaviour byte-for-byte; the fallback only engages
// where assembly would previously have failed.
func (a *CreateActionTransactionAssembler) sourceTransactionFor(it *wdk.StorageCreateTransactionSdkInput) (*transaction.Transaction, error) {
	if sourceTx := a.inputBEEF.FindTransaction(it.SourceTxID); sourceTx != nil {
		return sourceTx, nil
	}

	if len(it.SourceTransaction) == 0 {
		// Neither route has it. Returning nil here would surface later as the
		// opaque "every signable transaction input must have a source
		// transaction"; name the outpoint instead.
		return nil, fmt.Errorf(
			"no source transaction for input (outpoint: %s.%d): absent from inputBeef and not supplied by storage",
			it.SourceTxID, it.SourceVout,
		)
	}

	sourceTx, err := transaction.NewTransactionFromBytes(it.SourceTransaction)
	if err != nil {
		return nil, fmt.Errorf("cannot parse source transaction for input (outpoint: %s.%d): %w", it.SourceTxID, it.SourceVout, err)
	}
	if got := sourceTx.TxID().String(); got != it.SourceTxID {
		return nil, fmt.Errorf("source transaction txid mismatch for input (outpoint: %s.%d): expected %s got %s", it.SourceTxID, it.SourceVout, it.SourceTxID, got)
	}

	return sourceTx, nil
}

func (a *CreateActionTransactionAssembler) isInputFromArgs(it *wdk.StorageCreateTransactionSdkInput) bool {
	key := fmt.Sprintf("%s.%d", it.SourceTxID, it.SourceVout)
	_, ok := a.providedInputsByOutpoint[key]
	return ok
}

func (a *CreateActionTransactionAssembler) parseInputBEEF() error {
	// An absent inputBeef is legitimate now that a caller may decline to ship
	// ancestry it does not need to: sourceTransactionFor falls back to the raw
	// bytes storage attaches to each input. Start from an empty BEEF so lookups
	// miss cleanly instead of failing the whole assembly here.
	if len(a.createActionResult.InputBeef) == 0 {
		a.inputBEEF = transaction.NewBeefV2()
		return nil
	}

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
