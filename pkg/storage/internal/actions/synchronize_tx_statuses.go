package actions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

const (
	syncTxStatusLimit = 100
)

var (
	statusesReadyToSync = []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusCallback,
		wdk.ProvenTxStatusUnmined,
		wdk.ProvenTxStatusSending,
		wdk.ProvenTxStatusUnknown,
		wdk.ProvenTxStatusUnconfirmed,
	}
)

type synchronizeTxStatuses struct {
	lock                 sync.Mutex
	logger               *slog.Logger
	provenTxRepo         ProvenTxRepo
	services             wdk.Services
	syncTxStatusesConfig defs.SynchronizeTxStatuses
}

func newSynchronizeTxStatuses(logger *slog.Logger, syncTxStatusesConfig defs.SynchronizeTxStatuses, services wdk.Services, provenTxRepo ProvenTxRepo) *synchronizeTxStatuses {
	logger = logging.Child(logger, "synchronize_tx_statuses")

	if syncTxStatusesConfig.MaxAttempts == 0 {
		logger.Warn("synchronizeTxStatusesConfig.MaxAttempts is 0 which means that transactions will be tried to synchronize indefinitely; this may lead to performance issues")
	}

	return &synchronizeTxStatuses{
		logger:               logging.Child(logger, "synchronize_tx_statuses"),
		provenTxRepo:         provenTxRepo,
		syncTxStatusesConfig: syncTxStatusesConfig,
		services:             services,
	}
}

func (s *synchronizeTxStatuses) SynchronizeTxStatuses(ctx context.Context) error {
	lockAcquired := s.lock.TryLock()
	if !lockAcquired {
		s.logger.Warn("synchronizeTxStatuses is already running, skipping this run")
		return nil
	}
	defer s.lock.Unlock()

	// TODO Check current block height; skip if already synchronized for this block height

	// TODO: Use pagination (plus created_at older than now) strategy to process all the transactions that need synchronization
	txsToSync, err := s.provenTxRepo.FindProvenTxIDsByStatuses(ctx, syncTxStatusLimit, statusesReadyToSync...)
	if err != nil {
		return fmt.Errorf("provenTxRepo.FindTxIDsByStatuses failed: %w", err)
	}
	if len(txsToSync) == 0 {
		s.logger.Info("no transactions need synchronization")
		return nil
	}

	var failedAttempts []string
	for _, txToSync := range txsToSync {
		if err = ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled, aborting synchronizeTxStatuses: %w", err)
		}

		s.logger.Info("synchronizing", slog.String("txID", txToSync.TxID), slog.Uint64("attempts", txToSync.Attempts))

		merkleResult, err := s.services.MerklePath(ctx, txToSync.TxID)
		if err != nil {
			s.logger.Warn(
				"failed to get merkle path for transaction",
				slog.Any("err", err),
				slog.String("txID", txToSync.TxID),
				slog.Uint64("attempts", txToSync.Attempts),
				slog.String("status", string(txToSync.Status)),
			)

			failedAttempts = append(failedAttempts, txToSync.TxID)
			continue
		}

		if merkleResult.Header == nil || merkleResult.MerklePath == nil {
			s.logger.Info(
				"merkle path result is empty, this may be normal if the transaction is not yet mined",
				slog.String("txID", txToSync.TxID),
				slog.String("status", string(txToSync.Status)),
			)

			failedAttempts = append(failedAttempts, txToSync.TxID)
			continue
		}

		// TODO: Support history notes

		err = s.provenTxRepo.UpdateProvenTxAsMined(ctx, &entity.ProvenTxAsMined{
			TxID:        txToSync.TxID,
			BlockHeight: merkleResult.Header.Height,
			MerklePath:  merkleResult.MerklePath.Bytes(),
			BlockHash:   merkleResult.Header.Hash,
			MerkleRoot:  merkleResult.Header.MerkleRoot,
		})
		if err != nil {
			return fmt.Errorf("failed to update proven txs as mined: %w", err)
		}
	}

	err = s.provenTxRepo.IncreaseProvenTxAttemptsForTxIDs(ctx, failedAttempts)
	if err != nil {
		return fmt.Errorf("failed to increase attempts for txs: %w", err)
	}

	//NOTE: In TS, there is a periodic "review status" job that gets all the "invalid" proven tx transactions and
	//updates matching (user) transactions to "failed" and tidies outputs
	//TODO: Consider if we want to do the same or do it right away here
	err = s.provenTxRepo.SetStatusForProvenTxAboveAttempts(ctx, s.syncTxStatusesConfig.MaxAttempts, wdk.ProvenTxStatusInvalid)
	if err != nil {
		return fmt.Errorf("failed to set status for txs above attempts: %w", err)
	}

	return nil
}
