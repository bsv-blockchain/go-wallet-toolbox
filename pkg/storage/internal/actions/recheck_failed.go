package actions

import (
	"context"
	"fmt"
	"log/slog"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	recheckFailedMaxPages     = 10
	recheckFailedItemsPerPage = 1000
)

type recheckFailed struct {
	logger      *slog.Logger
	txRepo      TransactionsRepo
	knownTxRepo KnownTxRepo
	services    wdk.Services
}

func newRecheckFailed(logger *slog.Logger, txRepo TransactionsRepo, knownTxRepo KnownTxRepo, services wdk.Services) *recheckFailed {
	return &recheckFailed{
		logger:      logging.Child(logger, "recheck_failed"),
		txRepo:      txRepo,
		knownTxRepo: knownTxRepo,
		services:    services,
	}
}

// CheckFailedTransactions finds failed transactions, queries their on-chain status, and updates to unproven/completed if found.
func (a *recheckFailed) CheckFailedTransactions(ctx context.Context) error {
	log := a.logger.With("action", "checkFailedTransactions")
	log.InfoContext(ctx, "Checking failed transactions")

	// Collect txids of failed transactions in pages
	paging := queryopts.Paging{Limit: recheckFailedItemsPerPage, Sort: "asc"}
	var txIDs []string

	for range recheckFailedMaxPages {
		spec := &pkgentity.TransactionReadSpecification{}
		spec.Status = &pkgentity.Comparable[wdk.TxStatus]{Value: wdk.TxStatusFailed, Cmp: pkgentity.Equal}

		txs, err := a.txRepo.FindTransactions(ctx, spec, queryopts.WithPage(paging))
		if err != nil {
			return fmt.Errorf("failed to list failed transactions: %w", err)
		}

		for _, tx := range txs {
			if tx.TxID == nil || *tx.TxID == "" {
				continue
			}
			txIDs = append(txIDs, *tx.TxID)
		}

		if len(txs) < recheckFailedItemsPerPage {
			break
		}
		paging.Next()
	}

	if len(txIDs) == 0 {
		log.InfoContext(ctx, "No failed transactions with txid to recheck")
		return nil
	}

	statusRes, err := a.services.GetStatusForTxIDs(ctx, txIDs)
	if err != nil {
		return fmt.Errorf("failed to get status for txids: %w", err)
	}

	for _, item := range statusRes.Results {
		switch item.Status {
		case wdk.ResultStatusForTxIDMined.String():
			if err := a.txRepo.UpdateTransactionStatusByTxID(ctx, item.TxID, wdk.TxStatusCompleted); err != nil {
				log.ErrorContext(ctx, "Failed to update mined failed tx to completed", slog.String("txID", item.TxID), logging.Error(err))
			} else {
				log.InfoContext(ctx, "Marked previously failed tx as completed", slog.String("txID", item.TxID))
			}
			_ = a.knownTxRepo.UpdateKnownTxStatus(ctx, item.TxID, wdk.ProvenTxStatusCompleted, nil, nil)
		case wdk.ResultStatusForTxIDKnown.String():
			if err := a.txRepo.UpdateTransactionStatusByTxID(ctx, item.TxID, wdk.TxStatusUnproven); err != nil {
				log.ErrorContext(ctx, "Failed to update failed tx to unproven", slog.String("txID", item.TxID), logging.Error(err))
			} else {
				log.InfoContext(ctx, "Marked previously failed tx as unproven", slog.String("txID", item.TxID))
			}
			_ = a.knownTxRepo.UpdateKnownTxStatus(ctx, item.TxID, wdk.ProvenTxStatusUnmined, nil, nil)
		default:
			// unknown -> keep failed / doubleSpend
			//TODO: how to handle many checks of the same txid to save resources? cut off after n times? or time period since broadcast attempt?
			// can we use bulk get status for txids?
		}
	}

	return nil
}
