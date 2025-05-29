package actions

import (
	"context"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"iter"
	"maps"
	"strings"
)

type xinputDefinition struct {
	*wdk.ValidCreateActionInput
	Satoshis      satoshi.Value
	LockingScript string
}

type processedInputsResult struct {
	Inputs          iter.Seq[*xinputDefinition]
	Beef            *transaction.Beef
	ChangeOutputIDs []uint
}

type inputsProcessor struct {
	parent            *create
	ctx               context.Context
	userID            int
	providedInputs    []wdk.ValidCreateActionInput
	inputBEEF         []byte
	trustSelf         bool
	txIDsLookup       map[string]struct{}
	beef              *transaction.Beef
	basketIDForChange int
}

func newInputsProcessor(
	ctx context.Context,
	parent *create,
	userID int,
	providedInputs []wdk.ValidCreateActionInput,
	inputBEEF []byte,
	trustSelf bool,
	basketIDForChange int,
) *inputsProcessor {
	txIDsLookup := make(map[string]struct{})
	for _, input := range providedInputs {
		txIDsLookup[input.Outpoint.TxID] = struct{}{}
	}

	return &inputsProcessor{
		ctx:               ctx,
		parent:            parent,
		userID:            userID,
		inputBEEF:         inputBEEF,
		trustSelf:         trustSelf,
		txIDsLookup:       txIDsLookup,
		providedInputs:    providedInputs,
		basketIDForChange: basketIDForChange,
		beef:              transaction.NewBeefV2(),
	}
}

func (proc *inputsProcessor) processInputs() (*processedInputsResult, error) {
	var err error

	if len(proc.providedInputs) == 0 {
		return &processedInputsResult{
			Inputs: seq.Empty[*xinputDefinition](),
			Beef:   transaction.NewBeefV2(),
		}, nil
	}

	if len(proc.inputBEEF) > 0 {
		if err = proc.processInputBEEF(); err != nil {
			return nil, fmt.Errorf("failed to process inputBEEF: %w", err)
		}
	}

	if err = proc.checkInputsAndMergeTxIDsToBEEF(); err != nil {
		return nil, fmt.Errorf("failed to get beef for inputs: %w", err)
	}

	// TODO: Make SPV

	xinputDefs := make([]*xinputDefinition, 0, len(proc.providedInputs))
	var changeOutputIDs []uint
	for _, xinput := range proc.providedInputs {
		output, err := proc.parent.outputRepo.FindOutput(proc.ctx, proc.userID, xinput.Outpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to find output for input %s: %w", xinput.Outpoint, err)
		}

		var newXInput *xinputDefinition
		if output != nil {
			if output.BasketID != nil && *output.BasketID == proc.basketIDForChange {
				changeOutputIDs = append(changeOutputIDs, output.OutputID)
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
		Inputs:          seq.FromSlice(xinputDefs),
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
		missingIds := strings.Join(seq.Collect(txIDOnlyIDs), ",")
		return fmt.Errorf("valid and contain complete proof data for %s", missingIds)
	}

	// not provided in inputs but exists in the inputBEEF
	notProvidedInInputsTxs := seq.Filter(txIDOnlyIDs, func(txID string) bool {
		_, ok := proc.txIDsLookup[txID]
		return !ok
	})

	for txID := range notProvidedInInputsTxs {
		known, err := proc.parent.provenTxRepo.ExistsProvenTx(proc.ctx, txID, statusesOfTxReadyToBeUsedAsInput)
		if err != nil {
			return fmt.Errorf("failed to check if tx %s is known: %w", txID, err)
		}
		if !known {
			return fmt.Errorf("tx used in provided input is not known to storage; valid and contain complete proof data for unknown %s in inputBEEF", txID)
		}
	}

	return nil
}

func (proc *inputsProcessor) checkInputsAndMergeTxIDsToBEEF() error {
	for txID := range proc.txIDsLookup {
		if btx, ok := proc.beef.Transactions[txID]; ok && btx.DataFormat != transaction.TxIDOnly {
			continue
		}

		if !proc.trustSelf {
			return fmt.Errorf("valid and contain complete proof data for %s", txID)
		}

		known, err := proc.parent.provenTxRepo.ExistsProvenTx(proc.ctx, txID, statusesOfTxReadyToBeUsedAsInput)
		if err != nil {
			return fmt.Errorf("failed to check if tx %s is known: %w", txID, err)
		}

		if !known {
			return fmt.Errorf("tx used in provided input is not known to storage; valid and contain complete proof data for unknown %s in inputBEEF", txID)
		}

		proc.beef.MergeTxidOnly(txID)
	}

	return nil
}

func (proc *inputsProcessor) xinputDefOnKnownUTXO(xinput *wdk.ValidCreateActionInput, output *wdk.TableOutput) (*xinputDefinition, error) {
	if output.LockingScript == nil || len(*output.LockingScript) == 0 || output.Satoshis <= 0 {
		return nil, fmt.Errorf("output %s has no locking script or positive satoshis", xinput.Outpoint)
	}

	if !output.Spendable {
		return nil, fmt.Errorf("output %s is not spendable", xinput.Outpoint)
	}

	return &xinputDefinition{
		ValidCreateActionInput: xinput,
		Satoshis:               satoshi.MustFrom(output.Satoshis),
		LockingScript:          *output.LockingScript,
	}, nil
}

func (proc *inputsProcessor) xinputDefOnUnknownUTXO(xinput *wdk.ValidCreateActionInput) (*xinputDefinition, error) {
	btx, ok := proc.beef.Transactions[xinput.Outpoint.TxID]
	if !ok || btx == nil {
		return nil, fmt.Errorf("input %s not found in beef or outputs", xinput.Outpoint)
	}

	if btx.DataFormat == transaction.TxIDOnly {
		beefForTx, err := proc.parent.provenTxRepo.BuildValidBEEF(proc.ctx, xinput.Outpoint.TxID, statusesOfTxReadyToBeUsedAsInput)
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
		LockingScript:          out.LockingScript.String(),
	}, nil
}
