package actions

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/optional"
	commonslices "github.com/go-softwarelab/common/pkg/slices"
)

func (l *listActions) boolFromLabelQueryMode(labelQueryMode *primitives.LabelQueryModeString) (bool, error) {
	if labelQueryMode == nil {
		return false, nil
	}
	switch strings.ToLower(string(*labelQueryMode)) {
	case "all":
		return true, nil
	case "any":
		return false, nil
	default:
		return false, fmt.Errorf("invalid LabelQueryMode: %s", *labelQueryMode)
	}
}

func (l *listActions) toFilterParams(userID int, args *wdk.ListActionsArgs) (entity.ListActionsFilter, error) {
	labelNames := commonslices.Map(args.Labels, func(label primitives.StringUnder300) string {
		return string(label)
	})

	statuses := []wdk.TxStatus{
		wdk.TxStatusCompleted, wdk.TxStatusUnprocessed, wdk.TxStatusSending, wdk.TxStatusUnproven,
		wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal,
	}
	labelQueryMode, err := l.boolFromLabelQueryMode(args.LabelQueryMode)
	if err != nil {
		return entity.ListActionsFilter{}, fmt.Errorf("failed to parse LabelQueryMode: %w", err)
	}

	return entity.ListActionsFilter{
		UserID:           userID,
		Labels:           labelNames,
		Status:           statuses,
		IncludeAllLabels: labelQueryMode,
		Limit:            must.ConvertToIntFromUnsigned(args.Limit),
		Offset:           must.ConvertToIntFromUnsigned(args.Offset),
	}, nil
}

func (l *listActions) mapTransactionsToActions(txs []*wdk.TableTransaction) ([]uint, []string, []wdk.WalletAction) {
	transactionIDs := make([]uint, len(txs))
	var txIDs []string
	actions := make([]wdk.WalletAction, len(txs))

	for i, tx := range txs {
		transactionIDs[i] = tx.TransactionID
		if tx.TxID != nil {
			txIDs = append(txIDs, *tx.TxID)
		}

		actions[i] = wdk.WalletAction{
			Satoshis:    tx.Satoshis,
			Status:      string(tx.Status),
			IsOutgoing:  tx.IsOutgoing,
			Description: tx.Description,
			TxID:        optional.OfPtr(tx.TxID).OrZeroValue(),
			Version:     optional.OfPtr(tx.Version).OrZeroValue(),
			LockTime:    optional.OfPtr(tx.LockTime).OrZeroValue(),
		}
	}

	return transactionIDs, txIDs, actions
}

func (l *listActions) fetchInputsOutputs(ctx context.Context, txIDs []uint, args *wdk.ListActionsArgs) (map[uint][]*wdk.TableOutput, map[uint][]*wdk.TableOutput, error) {
	if args.IncludeInputs.Value() || args.IncludeOutputs.Value() {
		inputs, outputs, err := l.outputsRepo.FindInputsAndOutputsWithBaskets(ctx, txIDs, args.IncludeOutputLockingScripts.Value())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch inputs/outputs: %w", err)
		}
		return inputs, outputs, nil
	}
	return map[uint][]*wdk.TableOutput{}, map[uint][]*wdk.TableOutput{}, nil
}

func (l *listActions) loadLabelsIfNeeded(ctx context.Context, txIDs []uint, include *primitives.BooleanDefaultFalse) (map[uint][]string, error) {
	if !include.Value() {
		return map[uint][]string{}, nil
	}

	labelMap, err := l.transactionsRepo.GetLabelsForTransactions(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}

	return labelMap, nil
}

func (l *listActions) loadRawTxsIfNeeded(ctx context.Context, txIDStrs []string, include *primitives.BooleanDefaultFalse) (map[string][]byte, error) {
	if !include.Value() {
		return map[string][]byte{}, nil
	}

	rawTxMap, err := l.knownTxRepo.FindKnownTxRawTxs(ctx, txIDStrs)
	if err != nil {
		return nil, fmt.Errorf("failed to load raw transactions: %w", err)
	}

	return rawTxMap, nil
}

func (l *listActions) mapInputsOutputsLabels(actions []wdk.WalletAction, txs []*wdk.TableTransaction, inputMap, outputMap map[uint][]*wdk.TableOutput, labelMap map[uint][]string, rawTxMap map[string][]byte, args *wdk.ListActionsArgs) error {
	for i, tx := range txs {
		action := &actions[i]

		if args.IncludeLabels.Value() {
			l.mapLabelsToAction(action, tx.TransactionID, labelMap)
		}

		if args.IncludeOutputs.Value() {
			l.mapOutputsToAction(action, tx.TransactionID, outputMap)
		}

		if args.IncludeInputs.Value() && tx.TxID != nil {
			if err := l.mapInputsToAction(action, tx, inputMap, rawTxMap, args); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *listActions) mapLabelsToAction(action *wdk.WalletAction, txID uint, labelMap map[uint][]string) {
	if labels, ok := labelMap[txID]; ok {
		action.Labels = slices.Clone(labels)
	} else {
		action.Labels = []string{}
	}
}

func (l *listActions) mapOutputsToAction(action *wdk.WalletAction, txID uint, outputMap map[uint][]*wdk.TableOutput, tags ...string) {
	outputs := outputMap[txID]
	action.Outputs = l.mapToWalletActionOutputs(outputs, tags...)
}

func (l *listActions) mapToWalletActionOutputs(outputs []*wdk.TableOutput, tags ...string) []wdk.WalletActionOutput {
	result := make([]wdk.WalletActionOutput, 0, len(outputs))
	for _, o := range outputs {
		if o.BasketName == nil {
			continue
		}

		// TODO: Implement tags mapping if needed, currently empty as CreateActionArgs does not support tags

		lockingScript := o.LockingScript.Hex()

		result = append(result, wdk.WalletActionOutput{
			Satoshis:          must.ConvertToUInt64(o.Satoshis),
			Spendable:         o.Spendable,
			Tags:              tags,
			OutputIndex:       o.Vout,
			OutputDescription: o.OutputDescription,
			Basket:            *o.BasketName,
			LockingScript:     lockingScript,
		})
	}

	return result
}

func (l *listActions) mapInputsToAction(action *wdk.WalletAction, tx *wdk.TableTransaction, inputMap map[uint][]*wdk.TableOutput, rawTxMap map[string][]byte, args *wdk.ListActionsArgs) error {
	rawTx := rawTxMap[*tx.TxID]
	if rawTx == nil {
		return nil
	}

	inputs := inputMap[tx.TransactionID]
	mappedInputs, err := l.mapToWalletActionInputs(inputs, rawTx, args.IncludeInputSourceLockingScripts, args.IncludeInputUnlockingScripts)
	if err != nil {
		return fmt.Errorf("failed to map inputs: %w", err)
	}

	action.Inputs = mappedInputs
	if action.Inputs == nil {
		action.Inputs = []wdk.WalletActionInput{}
	}
	return nil
}

func (l *listActions) mapToWalletActionInputs(inputs []*wdk.TableOutput, rawTx []byte, includeSourceLockingScripts, includeUnlockingScripts *primitives.BooleanDefaultFalse) ([]wdk.WalletActionInput, error) {
	result := make([]wdk.WalletActionInput, 0, len(inputs))

	var tx *transaction.Transaction
	if len(rawTx) > 0 {
		var err error
		tx, err = transaction.NewTransactionFromBytes(rawTx)
		if err != nil {
			return result, fmt.Errorf("failed to parse raw transaction: %w", err)
		}
	}

	for _, o := range inputs {
		input := wdk.WalletActionInput{
			SourceSatoshis:   must.ConvertToUInt64(o.Satoshis),
			InputDescription: o.OutputDescription,
			SequenceNumber:   0,
		}

		if o.TxID != nil {
			input.SourceOutpoint = fmt.Sprintf("%s.%d", *o.TxID, o.Vout)
		}

		if includeSourceLockingScripts.Value() && o.LockingScript != nil {
			input.SourceLockingScript = o.LockingScript.Hex()
		}

		if tx != nil && includeUnlockingScripts.Value() {
			for _, txIn := range tx.Inputs {
				if txIn.SourceTXID.String() == *o.TxID && txIn.SourceTxOutIndex == o.Vout {
					input.SequenceNumber = txIn.SequenceNumber
					if txIn.UnlockingScript != nil {
						input.UnlockingScript = txIn.UnlockingScript.String()
					}
					break
				}
			}
		}

		result = append(result, input)
	}

	return result, nil
}
