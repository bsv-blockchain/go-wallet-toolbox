package actions

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"strings"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/slices"
)

var readyToBeInputProvenTxStatuses = []wdk.ProvenTxReqStatus{
	wdk.ProvenTxStatusUnsent,
	wdk.ProvenTxStatusUnmined,
	wdk.ProvenTxStatusUnconfirmed,
	wdk.ProvenTxStatusSending,
	wdk.ProvenTxStatusNoSend,
	wdk.ProvenTxStatusCompleted,
}

type xinputDefinition struct {
	*wdk.ValidCreateActionInput
	Satoshis      satoshi.Value
	LockingScript []byte

	knownOutput *pkgentity.Output // This is used only for known UTXOs, can be nil for unknown UTXOs
}

type xinputDefinitions []*xinputDefinition

func (inputs xinputDefinitions) iter() iter.Seq[*xinputDefinition] {
	return seq.FromSlice(inputs)
}

func (inputs xinputDefinitions) knownOutputs() iter.Seq[*pkgentity.Output] {
	knownOutputs := func(input *xinputDefinition) bool { return input.knownOutput != nil }
	toTableOutput := func(input *xinputDefinition) *pkgentity.Output { return input.knownOutput }

	return seq.Map(seq.Filter(inputs.iter(), knownOutputs), toTableOutput)
}

func (inputs xinputDefinitions) providedByUserAndUnknown() iter.Seq[*xinputDefinition] {
	unknownOutputs := func(input *xinputDefinition) bool { return input.knownOutput == nil }

	return seq.Filter(inputs.iter(), unknownOutputs)
}

type processedInputsResult struct {
	Inputs          xinputDefinitions
	Beef            *transaction.Beef
	ChangeOutputIDs []uint
}

type inputsProcessor struct {
	parent         *create
	ctx            context.Context
	userID         int
	providedInputs []wdk.ValidCreateActionInput
	inputBEEF      []byte
	trustSelf      bool
	txIDsLookup    map[chainhash.Hash]struct{}
	beef           *transaction.Beef
	logger         *slog.Logger
	beefVerifier   wdk.BeefVerifier
}

func newInputsProcessor(
	ctx context.Context,
	parent *create,
	userID int,
	reference string,
	providedInputs []wdk.ValidCreateActionInput,
	inputBEEF []byte,
	trustSelf bool,
	beefVerifier wdk.BeefVerifier,
) (*inputsProcessor, error) {
	txIDsLookup := make(map[chainhash.Hash]struct{}, len(providedInputs))
	for _, input := range providedInputs {
		txIDHash, err := chainhash.NewHashFromHex(input.Outpoint.TxID)
		if err != nil {
			return nil, fmt.Errorf("failed to parse txID %s: %w", input.Outpoint.TxID, err)
		}
		txIDsLookup[*txIDHash] = struct{}{}
	}

	logger := logging.Child(parent.logger, "inputsProcessor")
	logger = logger.With(logging.UserID(userID), logging.Reference(reference))

	return &inputsProcessor{
		ctx:            ctx,
		logger:         logger,
		parent:         parent,
		userID:         userID,
		inputBEEF:      inputBEEF,
		trustSelf:      trustSelf,
		txIDsLookup:    txIDsLookup,
		providedInputs: providedInputs,
		beef:           transaction.NewBeefV2(),
		beefVerifier:   beefVerifier,
	}, nil
}

func (proc *inputsProcessor) processInputs() (*processedInputsResult, error) {
	var err error

	if len(proc.providedInputs) == 0 {
		proc.logger.DebugContext(proc.ctx, "No inputs provided, skipping processing inputs")
		return &processedInputsResult{
			Beef: transaction.NewBeefV2(),
		}, nil
	}

	if len(proc.inputBEEF) > 0 {
		proc.logger.DebugContext(proc.ctx, "Processing inputBEEF")
		if err = proc.processInputBEEF(); err != nil {
			return nil, fmt.Errorf("failed to process inputBEEF: %w", err)
		}
	}

	proc.logger.DebugContext(proc.ctx, "Processing inputs")
	if err = proc.checkInputsAndMergeTxIDsToBEEF(); err != nil {
		return nil, fmt.Errorf("failed to get beef for inputs: %w", err)
	}

	if ok, err := proc.beefVerifier.VerifyBeef(proc.ctx, proc.beef, true); err != nil {
		return nil, fmt.Errorf("failed to verify beef: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("provided beef is not valid")
	}

	return proc.buildInputsDefinition()
}

func (proc *inputsProcessor) buildInputsDefinition() (*processedInputsResult, error) {
	xinputDefs := make([]*xinputDefinition, 0, len(proc.providedInputs))
	var changeOutputIDs []uint
	for _, xinput := range proc.providedInputs {
		output, err := proc.parent.outputRepo.FindOutput(proc.ctx, proc.userID, xinput.Outpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to find output for input %s: %w", xinput.Outpoint, err)
		}

		var newXInput *xinputDefinition
		if output != nil {
			if output.Change {
				changeOutputIDs = append(changeOutputIDs, output.ID)
			}
			newXInput, err = proc.xinputDefOnKnownUTXO(&xinput, output)
		} else {
			newXInput, err = proc.xinputDefOnUnknownUTXO(&xinput)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to process input %s: %w", xinput.Outpoint, err)
		}

		xinputDefs = append(xinputDefs, newXInput)
	}

	return &processedInputsResult{
		Inputs:          xinputDefs,
		Beef:            proc.beef,
		ChangeOutputIDs: changeOutputIDs,
	}, nil
}

func (proc *inputsProcessor) processInputBEEF() error {
	var err error

	if err = proc.beef.MergeBeefBytes(proc.inputBEEF); err != nil {
		return fmt.Errorf("failed to merge input beef: %w", err)
	}

	txIDOnlyIDs := seq2.Keys(seq2.Filter(maps.All(proc.beef.Transactions), func(_ chainhash.Hash, beefTx *transaction.BeefTx) bool {
		return beefTx.DataFormat == transaction.TxIDOnly
	}))

	if !proc.trustSelf && seq.IsNotEmpty(txIDOnlyIDs) {
		return missingProofError(toStringIDs(seq.Collect(txIDOnlyIDs)), "inputBEEF contains transactions with TxIDOnly that causes error if trustSelf not set")
	}

	// not provided in inputs but exists in the inputBEEF
	notProvidedInInputs := seq.Filter(txIDOnlyIDs, func(txIDHash chainhash.Hash) bool {
		_, ok := proc.txIDsLookup[txIDHash]
		return !ok
	})

	notProvidedInInputsTxIDs := seq.Collect(seq.Map(notProvidedInInputs, func(txIDHash chainhash.Hash) string {
		return txIDHash.String()
	}))

	if len(notProvidedInInputsTxIDs) == 0 {
		return nil
	}

	allKnown, err := proc.parent.knownTxRepo.AllKnownTxsExist(proc.ctx, notProvidedInInputsTxIDs, readyToBeInputProvenTxStatuses)
	if err != nil {
		return fmt.Errorf("failed to check if transactions are known: %w", err)
	}

	if !allKnown {
		return missingProofError(notProvidedInInputsTxIDs, "some tx in the inputBEEF is not known to storage")
	}

	return nil
}

func (proc *inputsProcessor) checkInputsAndMergeTxIDsToBEEF() error {
	missingFullProofs := seq.Collect(seq.Filter(maps.Keys(proc.txIDsLookup), func(txID chainhash.Hash) bool {
		btx, ok := proc.beef.Transactions[txID]
		return !ok || btx.DataFormat == transaction.TxIDOnly
	}))

	missingFullProofsTxIDs := slices.Map(missingFullProofs, func(txID chainhash.Hash) string {
		return txID.String()
	})

	if len(missingFullProofsTxIDs) == 0 {
		return nil
	}

	if !proc.trustSelf {
		return missingProofError(missingFullProofsTxIDs, "provided inputs contain transactions that are missing full proof in the inputBEEF")
	}

	allKnown, err := proc.parent.knownTxRepo.AllKnownTxsExist(proc.ctx, missingFullProofsTxIDs, readyToBeInputProvenTxStatuses)
	if err != nil {
		return fmt.Errorf("failed to check if transactions are known: %w", err)
	}

	if !allKnown {
		return missingProofError(missingFullProofsTxIDs, "some tx used in provided input is not known to storage")
	}

	for _, txIDHash := range missingFullProofs {
		proc.beef.MergeTxidOnly(&txIDHash)
	}

	return nil
}

func (proc *inputsProcessor) xinputDefOnKnownUTXO(xinput *wdk.ValidCreateActionInput, output *pkgentity.Output) (*xinputDefinition, error) {
	if len(output.LockingScript) == 0 || output.Satoshis <= 0 {
		return nil, fmt.Errorf("output %s has no locking script or positive satoshis", xinput.Outpoint)
	}

	if !output.Spendable {
		return nil, fmt.Errorf("output %s is not spendable", xinput.Outpoint)
	}

	return &xinputDefinition{
		ValidCreateActionInput: xinput,
		Satoshis:               satoshi.MustFrom(output.Satoshis),
		LockingScript:          output.LockingScript,
		knownOutput:            output,
	}, nil
}

func (proc *inputsProcessor) xinputDefOnUnknownUTXO(xinput *wdk.ValidCreateActionInput) (*xinputDefinition, error) {
	txIDHash, err := chainhash.NewHashFromHex(xinput.Outpoint.TxID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse txID %s: %w", xinput.Outpoint.TxID, err)
	}

	btx, ok := proc.beef.Transactions[*txIDHash]
	if !ok || btx == nil {
		return nil, fmt.Errorf("input %s not found in beef or outputs", xinput.Outpoint)
	}

	if btx.DataFormat == transaction.TxIDOnly {
		beefForTx, err := proc.parent.knownTxRepo.GetBEEFForTxID(proc.ctx, xinput.Outpoint.TxID, entity.WithStatusesToFilterOut(readyToBeInputProvenTxStatuses...))
		if err != nil {
			return nil, fmt.Errorf("failed to build beef for tx %s: %w", xinput.Outpoint.TxID, err)
		}

		btx, ok = beefForTx.Transactions[*txIDHash]
		if !ok || btx == nil {
			return nil, fmt.Errorf("tx %s not found in beef", xinput.Outpoint.TxID)
		}

		if _, err = proc.beef.MergeBeefTx(btx); err != nil {
			return nil, fmt.Errorf("failed to merge beef for tx %s: %w", xinput.Outpoint.TxID, err)
		}
	}

	voutInt := must.ConvertToIntFromUnsigned(xinput.Outpoint.Vout)
	if voutInt >= len(btx.Transaction.Outputs) {
		return nil, fmt.Errorf("input %s has invalid vout %d for tx %s with outputs count %d",
			xinput.Outpoint, xinput.Outpoint.Vout, xinput.Outpoint.TxID, len(btx.Transaction.Outputs))
	}

	out := btx.Transaction.Outputs[voutInt]

	return &xinputDefinition{
		ValidCreateActionInput: xinput,
		Satoshis:               satoshi.MustFrom(out.Satoshis),
		LockingScript:          out.LockingScript.Bytes(),
	}, nil
}

func missingProofError(txIDs []string, msgParts ...string) error {
	if len(txIDs) == 0 {
		return fmt.Errorf("%s", strings.Join(msgParts, "; "))
	}

	var subject string
	if len(txIDs) > 1 {
		subject = "transactions"
	} else {
		subject = "transaction"
	}

	txMsgPart := fmt.Sprintf("valid and contain complete proof data for %s: %s", subject, strings.Join(msgParts, ", "))
	if len(msgParts) > 0 {
		return fmt.Errorf("%s; %s", strings.Join(msgParts, "; "), txMsgPart)
	}
	return fmt.Errorf("%s", txMsgPart)
}

func toStringIDs(hashes []chainhash.Hash) []string {
	return slices.Map(hashes, func(hash chainhash.Hash) string {
		return hash.String()
	})
}
