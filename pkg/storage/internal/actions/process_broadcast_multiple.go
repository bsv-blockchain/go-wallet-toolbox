package actions

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"maps"
)

func (p *process) broadcastMultipleTxs(ctx context.Context, txIDs []string) (*wdk.ProcessActionResult, error) {
	if len(txIDs) == 0 {
		return &wdk.ProcessActionResult{
			SendWithResults: nil,
		}, nil
	}

	sendStatusesLookup, err := p.getSendStatuses(ctx, txIDs...)
	if err != nil {
		return nil, err
	}

	sendStatuses := maps.Values(sendStatusesLookup)
	if seq.ContainsAll(sendStatuses, wdk.SendWithResultStatusUnproven) {
		return &wdk.ProcessActionResult{
			SendWithResults: seq.Collect(seq2.MapTo(maps.All(sendStatusesLookup), func(txID string, status wdk.SendWithResultStatus) wdk.SendWithResult {
				return wdk.SendWithResult{
					TxID:   primitives.TXIDHexString(txID),
					Status: status,
				}
			})),
		}, nil
	}

	readyToSendTxIDs := seq.Collect(seq2.Keys(seq2.Filter(maps.All(sendStatusesLookup), func(txID string, status wdk.SendWithResultStatus) bool {
		return status == wdk.SendWithResultStatusSending || status == wdk.SendWithResultStatusFailed
	})))

	beef, err := p.knownTxRepo.BuildValidBEEFForTxIDs(ctx, seq.FromSlice(readyToSendTxIDs), nil, wdk.ProvenTxReqProblematicStatuses)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	if ok, err := beef.Verify(ctx, p.services, false); err != nil {
		return nil, fmt.Errorf("failed to verify beef: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("provided beef is not valid")
	}

	results, err := p.services.PostBEEF(ctx, beef, readyToSendTxIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to post BEEF: %w", err)
	}

	var sendWithResults []wdk.SendWithResult
	var notDelayedResults []wdk.ReviewActionResult

	aggregated := results.Aggregated(txIDs)
	for _, txID := range txIDs {
		aggBroadcastResult, ok := aggregated[txID]
		if !ok {
			sendWithResults = append(sendWithResults, wdk.SendWithResult{
				TxID:   primitives.TXIDHexString(txID),
				Status: wdk.SendWithResultStatusFailed,
			})
			notDelayedResults = append(notDelayedResults, wdk.ReviewActionResult{
				TxID:   primitives.TXIDHexString(txID),
				Status: wdk.ReviewActionResultStatusServiceError,
			})
			continue
		}

		newReqStatus, newTxStatus, reviewActionResult, sendWithResult, err := p.processBroadcastSingleTxResult(aggBroadcastResult, txID)
		if err != nil {
			return nil, err
		}

		notes := p.notesForPostBEEF(newReqStatus, aggBroadcastResult, results.ServiceErrors(), beef, []string{txID})

		err = p.txRepo.UpdateTransactionStatusByTxID(ctx, txID, newTxStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to update transaction status after broadcast: %w", err)
		}

		err = p.knownTxRepo.UpdateKnownTxStatus(ctx, txID, newReqStatus, wdk.ProvenTxReqBeyondBroadcastStageStatuses, notes)
		if err != nil {
			return nil, fmt.Errorf("failed to update transaction status after broadcast: %w", err)
		}

		sendWithResults = append(sendWithResults, sendWithResult)
		notDelayedResults = append(notDelayedResults, reviewActionResult)
	}

	return &wdk.ProcessActionResult{
		SendWithResults:   sendWithResults,
		NotDelayedResults: notDelayedResults,
	}, nil
}
