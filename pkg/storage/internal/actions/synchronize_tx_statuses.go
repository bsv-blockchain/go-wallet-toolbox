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
	lastBlockKey          = "synchronize_tx_statuses_last_block"
	noSendLastCheck       = "synchronize_tx_statuses_last_check_no_send"
)

var statusesReadyToSync = []wdk.ProvenTxReqStatus{
	wdk.ProvenTxStatusCallback,
	wdk.ProvenTxStatusUnmined,
	wdk.ProvenTxStatusSending,
	wdk.ProvenTxStatusUnknown,
	wdk.ProvenTxStatusUnconfirmed,
	wdk.ProvenTxStatusReorg,
}

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

	var heightForCheck uint
	var hashForCheck string

	header, err := s.services.FindChainTipHeader(ctx)
	if err != nil {
		// log warning but continue with sync anyway.
		s.logger.Warn("failed to find chain tip header, continuing without block tracking", slog.Any("err", err))
	} else {
		heightForCheck = header.Height - s.syncTxStatusesConfig.BlocksDelay
		hashForCheck = header.Hash
	}

	return s.synchronizeTxStatusesInternal(ctx, heightForCheck, hashForCheck)
}

func (s *synchronizeTxStatuses) SynchronizeTxStatusesForTip(ctx context.Context, tipHeight uint32, tipHash string) (txStatuses []wdk.TxSynchronizedStatus, resultErr error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-SynchronizeTxStatusesForTip")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	heightForCheck := uint(tipHeight) - s.syncTxStatusesConfig.BlocksDelay
	hashForCheck := tipHash

	return s.synchronizeTxStatusesInternal(ctx, heightForCheck, hashForCheck)
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

type LastBlockValue struct {
	BlockHeight uint   `json:"blockHeight"`
	BlockHash   string `json:"blockHash"`
}

func (s *synchronizeTxStatuses) getLastBlock(ctx context.Context) (*LastBlockValue, bool, error) {
	obj, ok, err := s.keyValueRepo.Get(ctx, lastBlockKey)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get last block height: %w", err)
	}

	if !ok {
		// It seems that it is the first time we are checking the block height
		return nil, false, nil
	}

	var lastBlock LastBlockValue
	if err := json.Unmarshal(obj, &lastBlock); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal last block height: %w", err)
	}

	if lastBlock.BlockHeight == 0 {
		return nil, false, fmt.Errorf("last block height is zero, this should not happen")
	}

	return &lastBlock, true, nil
}

func (s *synchronizeTxStatuses) setLastBlock(ctx context.Context, height uint, hash string) error {
	lastBlock := LastBlockValue{BlockHeight: height, BlockHash: hash}
	data, err := json.Marshal(lastBlock)
	if err != nil {
		return fmt.Errorf("failed to marshal last block: %w", err)
	}

	if err := s.keyValueRepo.Set(ctx, lastBlockKey, data); err != nil {
		return fmt.Errorf("failed to set last block: %w", err)
	}

	return nil
}

// filterTxsByConfirmationDepth filters transactions to only those that have at least BlocksDelay confirmations.
// This prevents unnecessary MerklePath calls for transactions that are not yet sufficiently confirmed.
// If the status service is unavailable, it returns an empty slice to skip synchronization.
func (s *synchronizeTxStatuses) filterTxsByConfirmationDepth(ctx context.Context, txs []*entity.KnownTxForStatusSync) ([]*entity.KnownTxForStatusSync, error) {
	if len(txs) == 0 {
		return txs, nil
	}

	txIDs := slices.Map(txs, func(tx *entity.KnownTxForStatusSync) string {
		return tx.TxID
	})

	statusResult, err := s.services.GetStatusForTxIDs(ctx, txIDs)
	if err != nil {
		s.logger.Warn("failed to get status for txIDs, skipping synchronization",
			slog.Any("err", err),
			slog.Int("count", len(txs)),
		)
		// Return empty slice to skip synchronization when we can't get the status
		return nil, nil
	}

	depthByTxID := make(map[string]int, len(statusResult.Results))
	for _, result := range statusResult.Results {
		if result.Depth == nil {
			continue
		}

		depthByTxID[result.TxID] = *result.Depth
	}

	filtered := slices.Filter(txs, func(tx *entity.KnownTxForStatusSync) bool {
		depth, ok := depthByTxID[tx.TxID]
		if !ok {
			s.logger.Debug("transaction depth not found or nil, skipping",
				slog.String("txID", tx.TxID),
				slog.String("status", string(tx.Status)),
			)
			return false
		}

		if depth < 0 || uint(depth) < s.syncTxStatusesConfig.BlocksDelay {
			s.logger.Debug("transaction does not have enough confirmations yet",
				slog.String("txID", tx.TxID),
				slog.Int("depth", depth),
				slog.Uint64("requiredDepth", uint64(s.syncTxStatusesConfig.BlocksDelay)),
			)
			return false
		}

		return true
	})

	s.logger.Debug("filtered transactions by confirmation depth",
		slog.Int("total", len(txs)),
		slog.Int("filtered", len(filtered)),
		slog.Uint64("requiredDepth", uint64(s.syncTxStatusesConfig.BlocksDelay)),
	)

	return filtered, nil
}

func (s *synchronizeTxStatuses) synchronizeTxStatusesInternal(ctx context.Context, heightForCheck uint, hashForCheck string) ([]wdk.TxSynchronizedStatus, error) {
	lockAcquired := s.lock.TryLock()
	if !lockAcquired {
		s.logger.Warn("synchronizeTxStatuses is already running, skipping this run")
		return nil, nil
	}
	defer s.lock.Unlock()

	// check if already processed this block
	lastBlock, ok, err := s.getLastBlock(ctx)
	if err != nil {
		s.logger.Warn("failed to check if already checked for this block", slog.Any("err", err))
		// We still want to proceed with the synchronization, so we log the error and continue
	} else if ok && lastBlock.BlockHeight == heightForCheck && lastBlock.BlockHash == hashForCheck {
		s.logger.Debug("already checked for this block, skipping",
			slog.Uint64("height", uint64(heightForCheck)),
			slog.String("hash", hashForCheck))
		return nil, nil
	}

	txStatuses, err := s.doSynchronizeTxStatuses(ctx, heightForCheck)
	if err != nil {
		return nil, err
	}

	// Save block info after successful completion (only if we have valid height and hash)
	if heightForCheck > 0 && hashForCheck != "" {
		if err := s.setLastBlock(ctx, heightForCheck, hashForCheck); err != nil {
			return txStatuses, fmt.Errorf("successfully synchronized tx statuses, but failed to set last block: %w", err)
		}
	}

	return txStatuses, nil
}

func (s *synchronizeTxStatuses) doSynchronizeTxStatuses(ctx context.Context, heightForCheck uint) ([]wdk.TxSynchronizedStatus, error) {
	statuses, err := s.getStatusesReadyToSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get statuses ready to sync: %w", err)
	}

	var txStatuses []wdk.TxSynchronizedStatus
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
		s.logger.Info("no transactions need synchronization", slog.Any("height", heightForCheck))
		return nil, nil
	}

	txsToSync, err = s.filterTxsByConfirmationDepth(ctx, txsToSync)
	if err != nil {
		return nil, fmt.Errorf("failed to filter txs by confirmation depth: %w", err)
	}

	if len(txsToSync) == 0 {
		s.logger.Info("no transactions with sufficient confirmations to synchronize", slog.Any("height", heightForCheck), slog.Uint64("requiredDepth", uint64(s.syncTxStatusesConfig.BlocksDelay)))
		return nil, nil
	}

	txIDs := slices.Map(txsToSync, func(tx *entity.KnownTxForStatusSync) string {
		return tx.TxID
	})
	txReferencesLookup, err := s.transactionRepo.FindReferencesByTxIDs(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to find references for txIDs: %w", err)
	}

	s.logger.Info("synchronizing transaction statuses", logging.Number("count", len(txsToSync)), logging.Number("height", heightForCheck))

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
				slog.Any("height", heightForCheck),
			)

			failedAttempts = append(failedAttempts, txToSync.TxID)
			continue
		}

		if merkleResult.BlockHeader == nil || merkleResult.MerklePath == nil {
			s.logger.Info(
				"merkle path result is empty, this may be normal if the transaction is not yet mined",
				slog.String("txID", txToSync.TxID),
				slog.String("status", string(txToSync.Status)),
				slog.Any("height", heightForCheck),
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
			Reference:   txReferencesLookup[txToSync.TxID],
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

	// NOTE: In TS, there is a periodic "review status" job that gets all the "invalid" proven tx transactions and
	// updates matching (user) transactions to "failed" and tidies outputs
	// TODO: Consider if we want to do the same or do it right away here
	updatedTxs, err := s.provenTxRepo.SetStatusForKnownTxsAboveAttempts(ctx, s.syncTxStatusesConfig.MaxAttempts, wdk.ProvenTxStatusInvalid)
	if err != nil {
		return nil, fmt.Errorf("failed to set status for txs above attempts: %w", err)
	}

	for _, updatedTx := range updatedTxs {
		txStatuses = append(txStatuses, wdk.TxSynchronizedStatus{
			TxID:      updatedTx.TxID,
			Status:    wdk.ProvenTxStatusInvalid,
			Reference: txReferencesLookup[updatedTx.TxID],
		})
	}

	return txStatuses, nil
}
