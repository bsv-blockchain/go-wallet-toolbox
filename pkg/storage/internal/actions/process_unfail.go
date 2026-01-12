package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	unfailMaxPages     = 10
	unfailItemsPerPage = 1000
)

var (
	statusesOfUnfailTxs = []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusUnfail,
	}
)

// UnFail scans known transactions with status 'unfail' and attempts to move them forward.
// If MerklePath is found: set KnownTx to 'unmined', set Transaction to 'unproven', and create UTXOs for spendable outputs.
// If not found: set KnownTx back to 'invalid'.
func (p *process) UnFail(ctx context.Context) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-UnFail")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	log := p.logger.With("action", "unfail")
	log.InfoContext(ctx, "Attempting to process 'unfail' transactions")

	startTime := time.Now()

	paging := queryopts.Paging{Limit: unfailItemsPerPage, Sort: "asc"}

	processed := 0
	for range unfailMaxPages {
		itemsPage, err := p.knownTxRepo.FindKnownTxIDsByStatuses(
			ctx,
			statusesOfUnfailTxs,
			queryopts.WithPage(paging),
			queryopts.WithUntil(queryopts.Until{Time: startTime}),
		)
		if err != nil {
			return fmt.Errorf("failed to find known txs by status 'unfail': %w", err)
		}

		if len(itemsPage) == 0 {
			if processed == 0 {
				log.InfoContext(ctx, "No transactions found to unfail")
			}
			return nil
		}

		for _, item := range itemsPage {
			p.unfailSingle(ctx, log, item.TxID)
			processed++
		}

		if len(itemsPage) < unfailItemsPerPage {
			break
		}

		paging.Next()
	}

	log.InfoContext(ctx, "Completed unfail processing", "processed", processed)

	return nil
}

// unfailSingle handles a single txID through the unfail flow.
func (p *process) unfailSingle(ctx context.Context, log *slog.Logger, txID string) {
	mp, err := p.services.MerklePath(ctx, txID)
	if err != nil && !errors.Is(err, wdk.ErrNotFoundError) {
		log.ErrorContext(ctx, "MerklePath query failed", slog.String("txID", txID), logging.Error(err))
		return
	}

	if mp != nil && mp.MerklePath != nil {
		p.markAsUnminedAndUnproven(ctx, log, txID)
		return
	}

	builder := history.NewBuilder().GetMerklePathNotFound(string(wdk.ProvenTxStatusUnfail))
	if err := p.knownTxRepo.UpdateKnownTxStatus(ctx, txID, wdk.ProvenTxStatusInvalid, nil, []history.Builder{builder}); err != nil {
		log.ErrorContext(ctx, "Failed to set known tx to 'invalid'", slog.String("txID", txID), logging.Error(err))
	} else {
		log.InfoContext(ctx, "MerklePath not found; known tx set to 'invalid'", slog.String("txID", txID))
	}
}

// markAsUnminedAndUnproven moves KnownTx and Transaction forward and ensures outputs are spendable.
func (p *process) markAsUnminedAndUnproven(ctx context.Context, log *slog.Logger, txID string) {
	builder := history.NewBuilder().GetMerklePathSuccess(string(wdk.ProvenTxStatusUnfail))
	if err := p.knownTxRepo.UpdateKnownTxStatus(ctx, txID, wdk.ProvenTxStatusUnmined, nil, []history.Builder{builder}); err != nil {
		log.ErrorContext(ctx, "Failed to set known tx to 'unmined'", slog.String("txID", txID), logging.Error(err))
	}
	if err := p.txRepo.UpdateTransactionStatusByTxID(ctx, txID, wdk.TxStatusUnproven); err != nil {
		log.ErrorContext(ctx, "Failed to set tx to 'unproven'", slog.String("txID", txID), logging.Error(err))
	} else {
		log.InfoContext(ctx, "Transaction set to 'unproven'", slog.String("txID", txID))
	}
	if err := p.utxoRepo.CreateUTXOForSpendableOutputsByTxID(ctx, txID); err != nil {
		log.ErrorContext(ctx, "Failed to create UTXOs for spendable outputs", slog.String("txID", txID), logging.Error(err))
	}
}
