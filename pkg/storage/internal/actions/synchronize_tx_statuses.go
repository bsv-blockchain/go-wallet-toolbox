package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	stdslices "slices"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/slices"
)

const (
	syncTxStatusMaxPages  = 10
	syncTxStatusesPerPage = 1000
	lastBlockHeightKey    = "synchronize_tx_statuses_last_block_height"
	noSendLastCheck       = "synchronize_tx_statuses_last_check_no_send"
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
	provenTxRepo         KnownTxRepo
	keyValueRepo         KeyValueRepo
	services             wdk.Services
	syncTxStatusesConfig defs.SynchronizeTxStatuses
	transactionRepo      TransactionsRepo
	checkNoSendDuration  time.Duration
}

func newSynchronizeTxStatuses(
	logger *slog.Logger,
	syncTxStatusesConfig defs.SynchronizeTxStatuses,
	services wdk.Services,
	provenTxRepo KnownTxRepo,
	keyValueRepo KeyValueRepo,
	transactionRepo TransactionsRepo,
) *synchronizeTxStatuses {
	logger = logging.Child(logger, "synchronize_tx_statuses")

	if syncTxStatusesConfig.MaxAttempts == 0 {
		logger.Warn("synchronizeTxStatusesConfig.MaxAttempts is 0 which means that transactions will be tried to synchronize indefinitely; this may lead to performance issues")
	}

	return &synchronizeTxStatuses{
		logger:               logging.Child(logger, "synchronize_tx_statuses"),
		provenTxRepo:         provenTxRepo,
		keyValueRepo:         keyValueRepo,
		syncTxStatusesConfig: syncTxStatusesConfig,
		services:             services,
		transactionRepo:      transactionRepo,
		checkNoSendDuration:  time.Duration(must.ConvertToInt64FromUnsigned(syncTxStatusesConfig.CheckNoSendPeriodHours)) * time.Hour,
	}
}

func (s *synchronizeTxStatuses) SynchronizeTxStatuses(ctx context.Context) (txStatuses []wdk.TxSynchronizedStatus, resultErr error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-SynchronizeTxStatuses")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	lockAcquired := s.lock.TryLock()
	if !lockAcquired {
		s.logger.Warn("synchronizeTxStatuses is already running, skipping this run")
		return nil, nil
	}
	defer s.lock.Unlock()

	checkedForCurrentBlock, currentHeight, err := s.alreadyCheckedForCurrentBlock(ctx)
	if err != nil {
		s.logger.Warn("failed to check if already checked for this block", slog.Any("err", err))
		// We still want to proceed with the synchronization, so we log the error and continue
	} else {
		if checkedForCurrentBlock {
			return nil, nil
		}

		defer func() {
			if resultErr != nil {
				return
			}
			if err := s.setLastBlockHeight(ctx, currentHeight); err != nil {
				resultErr = fmt.Errorf("successfully synchronized tx statuses, but failed to set last block height: %w", err)
			}
		}()
	}

	statuses, err := s.getStatusesReadyToSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get statuses ready to sync: %w", err)
	}

	var txsToSync []*entity.KnownTxForStatusSync
	paging := queryopts.Paging{Limit: syncTxStatusesPerPage, Sort: "asc"}
	for range syncTxStatusMaxPages {
		txsPage, err := s.provenTxRepo.FindKnownTxIDsByStatuses(ctx, statuses, queryopts.WithPage(paging))
		if err != nil {
			return nil, fmt.Errorf("provenTxRepo.FindKnownTxIDsByStatuses failed: %w", err)
		}

		txsToSync = append(txsToSync, txsPage...)

		if len(txsPage) < syncTxStatusesPerPage {
			break
		}

		paging.Next()
	}

	if len(txsToSync) == 0 {
		s.logger.Info("no transactions need synchronization", slog.Any("height", currentHeight))
		return nil, nil
	}

	s.logger.Info("synchronizing transaction statuses", logging.Number("count", len(txsToSync)), logging.Number("height", currentHeight))

	var failedAttempts []string
	for _, txToSync := range txsToSync {
		if err = ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled, aborting synchronizeTxStatuses: %w", err)
		}

		s.logger.Debug("synchronizing", slog.String("txID", txToSync.TxID), slog.Uint64("attempts", txToSync.Attempts))

		merkleResult, err := s.services.MerklePath(ctx, txToSync.TxID)
		if err != nil {
			s.logger.Warn(
				"failed to get merkle path for transaction",
				slog.Any("err", err),
				slog.String("txID", txToSync.TxID),
				slog.Uint64("attempts", txToSync.Attempts),
				slog.String("status", string(txToSync.Status)),
				slog.Any("height", currentHeight),
			)

			failedAttempts = append(failedAttempts, txToSync.TxID)
			continue
		}

		if merkleResult.BlockHeader == nil || merkleResult.MerklePath == nil {
			s.logger.Info(
				"merkle path result is empty, this may be normal if the transaction is not yet mined",
				slog.String("txID", txToSync.TxID),
				slog.String("status", string(txToSync.Status)),
				slog.Any("height", currentHeight),
			)

			failedAttempts = append(failedAttempts, txToSync.TxID)
			continue
		}

		transactionIDs, err := s.transactionRepo.FindTransactionIDsByTxID(ctx, txToSync.TxID)
		if err != nil {
			return nil, fmt.Errorf("failed to find transaction IDs by txID %s: %w", txToSync.TxID, err)
		}

		notes := slices.Map(transactionIDs, func(transactionID uint) history.Builder {
			return history.NewBuilder().NotifyTxOfProof(transactionID)
		})

		err = s.provenTxRepo.UpdateKnownTxAsMined(ctx, &entity.KnownTxAsMined{
			TxID:        txToSync.TxID,
			BlockHeight: merkleResult.BlockHeader.Height,
			MerklePath:  merkleResult.MerklePath.Bytes(),
			BlockHash:   merkleResult.BlockHeader.Hash,
			MerkleRoot:  merkleResult.BlockHeader.MerkleRoot,
			Notes:       notes,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update proven txs as mined: %w", err)
		}

		txStatuses = append(txStatuses, wdk.TxSynchronizedStatus{
			TxID:        txToSync.TxID,
			Status:      wdk.ProvenTxStatusCompleted,
			BlockHeight: merkleResult.BlockHeader.Height,
			BlockHash:   merkleResult.BlockHeader.Hash,
			MerklePath:  merkleResult.MerklePath,
			MerkleRoot:  merkleResult.BlockHeader.MerkleRoot,
		})
	}

	err = s.provenTxRepo.IncreaseKnownTxAttemptsForTxIDs(ctx, failedAttempts)
	if err != nil {
		return nil, fmt.Errorf("failed to increase attempts for txs: %w", err)
	}

	//NOTE: In TS, there is a periodic "review status" job that gets all the "invalid" proven tx transactions and
	//updates matching (user) transactions to "failed" and tidies outputs
	//TODO: Consider if we want to do the same or do it right away here
	updatedTxs, err := s.provenTxRepo.SetStatusForKnownTxsAboveAttempts(ctx, s.syncTxStatusesConfig.MaxAttempts, wdk.ProvenTxStatusInvalid)
	if err != nil {
		return nil, fmt.Errorf("failed to set status for txs above attempts: %w", err)
	}

	for _, updatedTx := range updatedTxs {
		txStatuses = append(txStatuses, wdk.TxSynchronizedStatus{
			TxID:   updatedTx.TxID,
			Status: wdk.ProvenTxStatusInvalid,
		})
	}

	return txStatuses, nil
}

func (s *synchronizeTxStatuses) alreadyCheckedForCurrentBlock(ctx context.Context) (bool, uint, error) {
	header, err := s.services.FindChainTipHeader(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to find chain tip header: %w", err)
	}

	lastHeight, ok, err := s.getLastBlockHeight(ctx)
	if err != nil {
		return false, 0, err
	}

	if ok && lastHeight == header.Height {
		s.logger.Debug("already checked for this block, skipping alreadyCheckedForThisBlock", slog.Any("height", header.Height))
		return true, header.Height, nil
	}

	return false, header.Height, nil
}

func (s *synchronizeTxStatuses) getStatusesReadyToSync(ctx context.Context) ([]wdk.ProvenTxReqStatus, error) {
	lastCheckNoSend, ok, err := s.keyValueRepo.Get(ctx, noSendLastCheck)
	if err != nil {
		return nil, fmt.Errorf("failed to get last check no send: %w", err)
	}

	if !ok {
		return s.statusesWithNoSend(ctx)
	}

	tim, err := time.Parse(time.RFC3339, string(lastCheckNoSend))
	if err != nil {
		s.logger.Warn("failed to parse last check no send, ignoring and proceeding without no send status", logging.Error(err))
		if err := s.setCurrentTimeAsLastCheckNoSend(ctx); err != nil {
			return nil, fmt.Errorf("failed to set current time as last check no send: %w", err)
		}

		return statusesReadyToSync, nil
	}

	if time.Since(tim) > s.checkNoSendDuration {
		return s.statusesWithNoSend(ctx)
	}

	return statusesReadyToSync, nil
}

func (s *synchronizeTxStatuses) statusesWithNoSend(ctx context.Context) ([]wdk.ProvenTxReqStatus, error) {
	if err := s.setCurrentTimeAsLastCheckNoSend(ctx); err != nil {
		return nil, fmt.Errorf("failed to set current time as last check no send: %w", err)
	}

	return append(stdslices.Clone(statusesReadyToSync), wdk.ProvenTxStatusNoSend), nil
}

func (s *synchronizeTxStatuses) setCurrentTimeAsLastCheckNoSend(ctx context.Context) error {
	newTimestamp := time.Now().Format(time.RFC3339)
	if err := s.keyValueRepo.Set(ctx, noSendLastCheck, []byte(newTimestamp)); err != nil {
		return fmt.Errorf("failed to set last check no send: %w", err)
	}

	return nil
}

type LastHeightValue struct {
	BlockHeight uint `json:"blockHeight"`
}

func (s *synchronizeTxStatuses) getLastBlockHeight(ctx context.Context) (uint, bool, error) {
	obj, ok, err := s.keyValueRepo.Get(ctx, lastBlockHeightKey)
	if err != nil {
		return 0, false, fmt.Errorf("failed to get last block height: %w", err)
	}

	if !ok {
		// It seems that it is the first time we are checking the block height
		return 0, false, nil
	}

	var lastHeight LastHeightValue
	if err := json.Unmarshal(obj, &lastHeight); err != nil {
		return 0, false, fmt.Errorf("failed to unmarshal last block height: %w", err)
	}

	if lastHeight.BlockHeight == 0 {
		return 0, false, fmt.Errorf("last block height is zero, this should not happen")
	}

	return lastHeight.BlockHeight, true, nil
}

func (s *synchronizeTxStatuses) setLastBlockHeight(ctx context.Context, blockHeight uint) error {
	lastHeight := LastHeightValue{BlockHeight: blockHeight}
	data, err := json.Marshal(lastHeight)
	if err != nil {
		return fmt.Errorf("failed to marshal last block height: %w", err)
	}

	if err := s.keyValueRepo.Set(ctx, lastBlockHeightKey, data); err != nil {
		return fmt.Errorf("failed to set last block height: %w", err)
	}

	return nil
}
