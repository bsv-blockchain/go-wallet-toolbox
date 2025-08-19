package actions

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"log/slog"
	"time"
)

const (
	sendWaitingLimit = 100
)

var (
	statusesOfWaitingTxs = []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusUnsent,
		wdk.ProvenTxStatusSending,
	}
)

func (p *process) SendWaitingTransactions(ctx context.Context, agedLimit time.Duration) error {
	log := p.logger.With("action", "send waiting transactions").With("agedLimit", agedLimit.Seconds())
	log.InfoContext(ctx, "Attempting to send waiting transactions")

	since := queryopts.Since{
		Time: time.Now().Add(-agedLimit),
	}
	txsToBroadcast, err := p.knownTxRepo.FindKnownTxIDsByStatuses(ctx, sendWaitingLimit, statusesOfWaitingTxs, queryopts.WithSince(since))
	if err != nil {
		return fmt.Errorf("failed to find known txs by statuses: %w", err)
	}

	if len(txsToBroadcast) == 0 {
		log.InfoContext(ctx, "No transactions found to send")
		return nil
	}

	log.InfoContext(ctx, "Found transactions to send", "count", len(txsToBroadcast))

	for _, transaction := range txsToBroadcast {
		err = p.delayedBroadcastTransaction(ctx, log, transaction)
		if err != nil {
			log.ErrorContext(ctx, "Failed to broadcast transaction", "txID", transaction.TxID, "error", err)
		} else {
			log.InfoContext(ctx, "Successfully broadcasted transaction", "txID", transaction.TxID)
		}
	}

	return nil
}

func (p *process) delayedBroadcastTransaction(ctx context.Context, log *slog.Logger, transaction *entity.KnownTxForStatusSync) error {
	log.InfoContext(ctx, "Attempting to broadcast transaction", "txID", transaction.TxID)

	var txIDs []string
	if transaction.Batch == nil {
		txIDs = []string{transaction.TxID}
	} else {
		// TODO: handle batch transactions
	}

	// TODO: increment the attempt count for txIDs

	result, err := p.broadcastTxs(ctx, txIDs, false)
	if err != nil {
		return fmt.Errorf("failed to broadcast transaction %s: %w", transaction.TxID, err)
	}

	success := true
	for _, res := range result.NotDelayedResults {
		if res.Status != wdk.ReviewActionResultStatusSuccess {
			success = false
			log.WarnContext(ctx, "Problematic broadcast result", "txID", transaction.TxID, "status", res.Status)
		}
	}

	if !success {
		return fmt.Errorf("failed to broadcast transaction %s", transaction.TxID)
	}

	return nil
}
