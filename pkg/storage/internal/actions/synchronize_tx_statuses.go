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
	return &synchronizeTxStatuses{
		logger:               logging.Child(logger, "synchronize_tx_statuses"),
		provenTxRepo:         provenTxRepo,
		syncTxStatusesConfig: syncTxStatusesConfig,
		services:             services,
	}
}

func (s *synchronizeTxStatuses) SynchronizeTxStatuses(ctx context.Context) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// TODO Check current block height; skip if already synchronized for this block height

	txsToSync, err := s.provenTxRepo.FindProvenTxIDsByStatuses(ctx, syncTxStatusLimit, statusesReadyToSync...)
	if err != nil {
		return fmt.Errorf("provenTxRepo.FindTxIDsByStatuses failed: %w", err)
	}
	if len(txsToSync) == 0 {
		s.logger.Info("no transactions need synchronization")
		return nil
	}

	for _, txToSync := range txsToSync {
		s.logger.Info("synchronizing", slog.String("txID", txToSync.TxID), slog.Uint64("attempts", txToSync.Attempts))

		merkleResult, err := s.services.MerklePath(ctx, txToSync.TxID)
		if err != nil {
			s.logger.Warn(
				"failed to get merkle path for transaction",
				slog.Any("err", err),
				slog.String("txID", txToSync.TxID),
				slog.Uint64("attempts", txToSync.Attempts),
			)

			// TODO: Increase attempts
			continue
		}

		if merkleResult.Header == nil || merkleResult.MerklePath == nil {
			s.logger.Warn(
				"merkle path result is empty, this may be normal if the transaction is not yet mined",
				slog.String("txID", txToSync.TxID),
			)

			// TODO: Increase attempts
			continue
		}

		// TODO: Support history notes
		// TODO: Check how old it the tx and if it is older than x hours, mark it as invalid

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

	// TODO: Update txs attempts for transactions that merkle path was NOT found
	// TODO: For transactions that attempts > maxAttempts, mark them as invalid

	return nil
}
