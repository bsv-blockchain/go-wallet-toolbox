package actions

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
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

	knownOutput *entity.Output // This is used only for known UTXOs, can be nil for unknown UTXOs
}

type xinputDefinitions []*xinputDefinition

func (inputs xinputDefinitions) iter() iter.Seq[*xinputDefinition] {
	return seq.FromSlice(inputs)
}

func (inputs xinputDefinitions) knownOutputs() iter.Seq[*entity.Output] {
	knownOutputs := func(input *xinputDefinition) bool { return input.knownOutput != nil }
	toTableOutput := func(input *xinputDefinition) *entity.Output { return input.knownOutput }

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
	txIDsLookup    map[string]struct{}
	beef           *transaction.Beef
	logger         *slog.Logger
}

func newInputsProcessor(ctx context.Context, parent *create, userID int, reference string, providedInputs []wdk.ValidCreateActionInput, inputBEEF []byte, trustSelf bool) *inputsProcessor {
	txIDsLookup := make(map[string]struct{})
	for _, input := range providedInputs {
		txIDsLookup[input.Outpoint.TxID] = struct{}{}
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
	}
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

	// TODO: Make SPV

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

	txIDOnlyIDs := seq2.Keys(seq2.Filter(maps.All(proc.beef.Transactions), func(_ string, beefTx *transaction.BeefTx) bool {
		return beefTx.DataFormat == transaction.TxIDOnly
	}))

	if !proc.trustSelf && seq.IsNotEmpty(txIDOnlyIDs) {
		return proc.missingProofError(seq.Collect(txIDOnlyIDs), "inputBEEF contains transactions with TxIDOnly that causes error if trustSelf not set")
	}

	// not provided in inputs but exists in the inputBEEF
	notProvidedInInputsTxs := seq.Collect(seq.Filter(txIDOnlyIDs, func(txID string) bool {
		_, ok := proc.txIDsLookup[txID]
		return !ok
	}))

	if len(notProvidedInInputsTxs) == 0 {
		return nil
	}

	allKnown, err := proc.parent.knownTxRepo.AllKnownTxsExist(proc.ctx, notProvidedInInputsTxs, readyToBeInputProvenTxStatuses)
	if err != nil {
		return fmt.Errorf("failed to check if transactions are known: %w", err)
	}

	if !allKnown {
		return proc.missingProofError(notProvidedInInputsTxs, "some tx in the inputBEEF is not known to storage")
	}

	return nil
}

func (proc *inputsProcessor) checkInputsAndMergeTxIDsToBEEF() error {
	missingFullProofs := seq.Collect(seq.Filter(maps.Keys(proc.txIDsLookup), func(txID string) bool {
		btx, ok := proc.beef.Transactions[txID]
		return !ok || btx.DataFormat == transaction.TxIDOnly
	}))

	if len(missingFullProofs) == 0 {
		return nil
	}

	if !proc.trustSelf {
		return proc.missingProofError(missingFullProofs, "provided inputs contain transactions that are missing full proof in the inputBEEF")
	}

	allKnown, err := proc.parent.knownTxRepo.AllKnownTxsExist(proc.ctx, missingFullProofs, readyToBeInputProvenTxStatuses)
	if err != nil {
		return fmt.Errorf("failed to check if transactions are known: %w", err)
	}

	if !allKnown {
		return proc.missingProofError(missingFullProofs, "some tx used in provided input is not known to storage")
	}

	for _, txID := range missingFullProofs {
		proc.beef.MergeTxidOnly(txID)
	}

	return nil
}

func (proc *inputsProcessor) xinputDefOnKnownUTXO(xinput *wdk.ValidCreateActionInput, output *entity.Output) (*xinputDefinition, error) {
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
	btx, ok := proc.beef.Transactions[xinput.Outpoint.TxID]
	if !ok || btx == nil {
		return nil, fmt.Errorf("input %s not found in beef or outputs", xinput.Outpoint)
	}

	if btx.DataFormat == transaction.TxIDOnly {
		beefForTx, err := proc.parent.knownTxRepo.BuildValidBEEF(proc.ctx, xinput.Outpoint.TxID, readyToBeInputProvenTxStatuses)
		if err != nil {
			return nil, fmt.Errorf("failed to build beef for tx %s: %w", xinput.Outpoint.TxID, err)
		}

		btx, ok = beefForTx.Transactions[xinput.Outpoint.TxID]
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

func (proc *inputsProcessor) missingProofError(txIDs []string, msgParts ...string) error {
	if len(txIDs) == 0 {
		return fmt.Errorf("%s", strings.Join(msgParts, "; "))
	}

	var subject string
	if len(txIDs) > 1 {
		subject = "transactions"
	} else {
		subject = "transaction"
	}

	txMsgPart := fmt.Sprintf("valid and contain complete proof data for %s: %s", subject, strings.Join(txIDs, ", "))
	if len(msgParts) > 0 {
		return fmt.Errorf("%s; %s", strings.Join(msgParts, "; "), txMsgPart)
	}
	return fmt.Errorf("%s", txMsgPart)
}
