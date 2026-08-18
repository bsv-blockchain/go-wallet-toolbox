package actions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/slices"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Broadcaster lifecycle statuses pushed by external event streams (Arcade SSE).
// Names follow github.com/bsv-blockchain/arcade models.Status / docs/sse.md.
//
// MINED and IMMUTABLE frames may include merklePath (BRC-74 BUMP hex) plus
// blockHash/blockHeight so storage can complete the tx without polling. Catchup
// may omit merklePath — resolveExternalMerklePath falls back to services.
const (
	externalTxStatusReceived            = "RECEIVED"
	externalTxStatusSentToNetwork       = "SENT_TO_NETWORK"
	externalTxStatusAcceptedByNetwork   = "ACCEPTED_BY_NETWORK"
	externalTxStatusSeenOnNetwork       = "SEEN_ON_NETWORK"
	externalTxStatusSeenMultipleNodes   = "SEEN_MULTIPLE_NODES"    // canonical Arcade name
	externalTxStatusSeenOnMultipleNodes = "SEEN_ON_MULTIPLE_NODES" // legacy docs alias
	externalTxStatusStumpProcessing     = "STUMP_PROCESSING"
	// ARC-only intermediates that some hosts still emit.
	externalTxStatusAnnouncedToNetwork = "ANNOUNCED_TO_NETWORK"
	externalTxStatusStored             = "STORED"
	externalTxStatusMined              = "MINED"
	externalTxStatusImmutable          = "IMMUTABLE"
	externalTxStatusRejected           = "REJECTED"
	externalTxStatusDoubleSpend        = "DOUBLE_SPEND_ATTEMPTED"
	// PENDING_RETRY / UNKNOWN are intentionally not advanced (no network proof).
)

// isExternalBroadcastedStatus reports whether an external status is enough
// evidence that the broadcaster accepted/propagated the tx (advance unsent/
// sending → unmined and activate UserUTXO rows). Without these mappings, live
// Arcade streams leave utxo_status="" and funding fails with "not enough funds".
func isExternalBroadcastedStatus(status string) bool {
	switch status {
	case externalTxStatusReceived,
		externalTxStatusSentToNetwork,
		externalTxStatusAcceptedByNetwork,
		externalTxStatusSeenOnNetwork,
		externalTxStatusSeenMultipleNodes,
		externalTxStatusSeenOnMultipleNodes,
		externalTxStatusStumpProcessing,
		externalTxStatusAnnouncedToNetwork,
		externalTxStatusStored:
		return true
	default:
		return false
	}
}

// isExternalRejectedStatus reports terminal-ish rejection signals that need
// network re-check before failing the wallet tx.
func isExternalRejectedStatus(status string) bool {
	return status == externalTxStatusRejected || status == externalTxStatusDoubleSpend
}

// externalBroadcastStatusHistoryNote is the "what" of the TxNote recorded for external status events.
const externalBroadcastStatusHistoryNote = "externalBroadcastStatus"

// ProcessExternalTxStatusUpdate applies a broadcaster-pushed lifecycle update.
//
// Safety rules (the external stream must never corrupt the wallet state):
//   - an unknown txid or an already-terminal stored status is a no-op,
//   - MINED/IMMUTABLE applies through the existing UpdateKnownTxAsMined flow, with the
//     event-supplied merklePath (BUMP hex) validated against the chain tracker (or
//     fetched from the services when the event carries none — catchup may omit path),
//   - RECEIVED / SENT_TO_NETWORK / ACCEPTED_BY_NETWORK / SEEN_* / STUMP_PROCESSING
//     advance in-flight (unsent/sending) transactions to the same post-broadcast
//     status broadcastTxs sets after a successful broadcast (unmined),
//   - REJECTED / DOUBLE_SPEND_ATTEMPTED is re-verified through confirmDoubleSpends
//     before any terminal failure, preserving the false-double-spend guarantees.
//
// Returns the synchronized statuses for any tx whose stored state changed.
func (p *process) ProcessExternalTxStatusUpdate(ctx context.Context, ev wdk.BroadcastStatusEvent) (txStatuses []wdk.TxSynchronizedStatus, resultErr error) {
	ctx, span := tracing.StartTracing(ctx, "StorageActions-ProcessExternalTxStatusUpdate")
	defer func() {
		tracing.EndTracing(span, resultErr)
	}()

	logger := p.logger.With(
		slog.String("txID", ev.TxID),
		slog.String("externalStatus", ev.Status),
	)

	statuses, err := p.knownTxRepo.FindKnownTxStatuses(ctx, ev.TxID)
	if err != nil {
		return nil, fmt.Errorf("failed to find known tx status for txID %s: %w", ev.TxID, err)
	}

	current, known := statuses[ev.TxID]
	if !known {
		logger.DebugContext(ctx, "Ignoring external status event for a transaction unknown to this storage")
		return nil, nil
	}

	if isTerminalProvenTxStatus(current) {
		logger.DebugContext(
			ctx, "Ignoring external status event for a transaction already in a terminal status",
			slog.String("storedStatus", string(current)),
		)
		return nil, nil
	}

	switch {
	case ev.Status == externalTxStatusMined || ev.Status == externalTxStatusImmutable:
		return p.applyExternalMined(ctx, &ev)
	case isExternalBroadcastedStatus(ev.Status):
		return p.advanceToBroadcastedFromExternalEvent(ctx, &ev, current)
	case isExternalRejectedStatus(ev.Status):
		return p.applyExternalRejected(ctx, &ev, current)
	default:
		logger.WarnContext(ctx, "Ignoring external status event with an unsupported status")
		return nil, nil
	}
}

// isTerminalProvenTxStatus reports whether the stored status must never be changed by an
// external status event (completed / failed / double spend).
func isTerminalProvenTxStatus(status wdk.ProvenTxReqStatus) bool {
	switch status { //nolint:exhaustive // default case handles remaining statuses
	case wdk.ProvenTxStatusCompleted,
		wdk.ProvenTxStatusInvalid,
		wdk.ProvenTxStatusDoubleSpend:
		return true
	default:
		return false
	}
}

// applyExternalMined completes the transaction through the existing UpdateKnownTxAsMined
// flow (KnownTx + Transaction + UserUTXO updates stay atomic).
func (p *process) applyExternalMined(ctx context.Context, ev *wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error) {
	merklePath, blockHeader, err := p.resolveExternalMerklePath(ctx, ev)
	if err != nil {
		return nil, err
	}
	if merklePath == nil {
		p.logger.InfoContext(
			ctx, "Merkle path for externally mined tx is not available yet - leaving the tx to the polling safety net",
			slog.String("txID", ev.TxID),
		)
		return nil, nil
	}

	transactionIDs, err := p.txRepo.FindTransactionIDsByTxID(ctx, ev.TxID)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction IDs by txID %s: %w", ev.TxID, err)
	}

	notes := slices.Map(transactionIDs, func(transactionID uint) history.Builder {
		return history.NewBuilder().NotifyTxOfProof(transactionID)
	})
	notes = append(notes, externalStatusNote(ev).WithNewStatus(string(wdk.ProvenTxStatusCompleted)))

	err = p.knownTxRepo.UpdateKnownTxAsMined(ctx, &entity.KnownTxAsMined{
		TxID:        ev.TxID,
		BlockHeight: blockHeader.Height,
		MerklePath:  merklePath.Bytes(),
		BlockHash:   blockHeader.Hash,
		MerkleRoot:  blockHeader.MerkleRoot,
		Notes:       notes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update known tx %s as mined: %w", ev.TxID, err)
	}

	reference, labels, err := p.referenceAndLabelsForTxID(ctx, ev.TxID)
	if err != nil {
		return nil, err
	}

	return []wdk.TxSynchronizedStatus{{
		TxID:        ev.TxID,
		Status:      wdk.ProvenTxStatusCompleted,
		Reference:   reference,
		BlockHeight: blockHeader.Height,
		BlockHash:   blockHeader.Hash,
		MerklePath:  merklePath,
		MerkleRoot:  blockHeader.MerkleRoot,
		Labels:      labels,
	}}, nil
}

// resolveExternalMerklePath returns the merkle path and block header to apply for a
// MINED/IMMUTABLE event. The event-supplied path is parsed and its computed root is
// validated for the height via the chain tracker; when the event carries no path - or
// no block hash, which KnownTx must never store empty - the proof is fetched from the
// services instead (same source as synchronizeTxStatuses).
// A (nil, nil, nil) result means the proof is not available yet.
func (p *process) resolveExternalMerklePath(ctx context.Context, ev *wdk.BroadcastStatusEvent) (*transaction.MerklePath, *wdk.MerklePathBlockHeader, error) {
	if ev.MerklePath == "" {
		return p.merklePathFromServices(ctx, ev.TxID)
	}

	if ev.BlockHash == "" {
		p.logger.WarnContext(
			ctx, "External MINED event carries a merkle path but no block hash - falling back to the services",
			slog.String("txID", ev.TxID),
		)
		return p.merklePathFromServices(ctx, ev.TxID)
	}

	merklePath, err := transaction.NewMerklePathFromHex(ev.MerklePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse merkle path from external status event for tx %s: %w", ev.TxID, err)
	}

	if ev.BlockHeight != 0 && merklePath.BlockHeight != ev.BlockHeight {
		return nil, nil, fmt.Errorf(
			"merkle path block height %d does not match event block height %d for tx %s",
			merklePath.BlockHeight, ev.BlockHeight, ev.TxID,
		)
	}

	txID := ev.TxID
	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute merkle root from external merkle path for tx %s: %w", ev.TxID, err)
	}

	rootHash, err := chainhash.NewHashFromHex(merkleRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse computed merkle root %s for tx %s: %w", merkleRoot, ev.TxID, err)
	}

	valid, err := p.services.IsValidRootForHeight(ctx, rootHash, merklePath.BlockHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate merkle root for height %d for tx %s: %w", merklePath.BlockHeight, ev.TxID, err)
	}
	if !valid {
		return nil, nil, fmt.Errorf(
			"merkle root %s from external status event is not valid for height %d (tx %s)",
			merkleRoot, merklePath.BlockHeight, ev.TxID,
		)
	}

	return merklePath, &wdk.MerklePathBlockHeader{
		Height:     merklePath.BlockHeight,
		Hash:       ev.BlockHash,
		MerkleRoot: merkleRoot,
	}, nil
}

// merklePathFromServices fetches the merkle proof for a txID from the configured services
// (same source as synchronizeTxStatuses). A (nil, nil, nil) result means the proof is not
// available yet - the polling safety net keeps the tx alive.
func (p *process) merklePathFromServices(ctx context.Context, txID string) (*transaction.MerklePath, *wdk.MerklePathBlockHeader, error) {
	merkleResult, err := p.services.MerklePath(ctx, txID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch merkle path for externally mined tx %s: %w", txID, err)
	}
	if merkleResult.MerklePath == nil || merkleResult.BlockHeader == nil {
		return nil, nil, nil
	}
	return merkleResult.MerklePath, merkleResult.BlockHeader, nil
}

// advanceToBroadcastedFromExternalEvent advances an in-flight (unsent/sending) transaction
// to the same post-broadcast status broadcastTxs sets after a successful broadcast:
// KnownTx -> unmined, Transaction -> unproven, spendable outputs get UTXOs.
// Transactions already at or past that status are left untouched.
func (p *process) advanceToBroadcastedFromExternalEvent(ctx context.Context, ev *wdk.BroadcastStatusEvent, current wdk.ProvenTxReqStatus) ([]wdk.TxSynchronizedStatus, error) {
	if current != wdk.ProvenTxStatusUnsent && current != wdk.ProvenTxStatusSending {
		p.logger.DebugContext(
			ctx, "External status event does not advance the transaction - leaving the stored status untouched",
			slog.String("txID", ev.TxID),
			slog.String("storedStatus", string(current)),
			slog.String("externalStatus", ev.Status),
		)
		return nil, nil
	}

	notes := []history.Builder{externalStatusNote(ev).WithNewStatus(string(wdk.ProvenTxStatusUnmined))}
	applied, err := p.knownTxRepo.AdvanceKnownTxToBroadcasted(ctx, ev.TxID, wdk.ProvenTxReqBeyondBroadcastStageStatuses, notes)
	if err != nil {
		return nil, fmt.Errorf("failed to advance tx %s to the post-broadcast state: %w", ev.TxID, err)
	}
	if !applied {
		p.logger.DebugContext(
			ctx, "Transaction advanced beyond the broadcast stage concurrently - leaving the stored state untouched",
			slog.String("txID", ev.TxID),
			slog.String("externalStatus", ev.Status),
		)
		return nil, nil
	}

	reference, labels, err := p.referenceAndLabelsForTxID(ctx, ev.TxID)
	if err != nil {
		return nil, err
	}

	return []wdk.TxSynchronizedStatus{{
		TxID:      ev.TxID,
		Status:    wdk.ProvenTxStatusUnmined,
		Reference: reference,
		Labels:    labels,
	}}, nil
}

// applyExternalRejected routes a REJECTED event through the confirmDoubleSpends
// verification machinery (network re-check) before any terminal failure:
//   - the network knows the tx -> false positive -> treat as a successful broadcast,
//   - no positive evidence (no competing txs) or the network state is inconclusive ->
//     make the tx retryable again (see requeueExternallyRejectedTx),
//   - confirmed double spend -> existing terminal failure path.
func (p *process) applyExternalRejected(ctx context.Context, ev *wdk.BroadcastStatusEvent, current wdk.ProvenTxReqStatus) ([]wdk.TxSynchronizedStatus, error) {
	aggregated := synthesizeRejectedVerdict(ev)
	p.confirmDoubleSpends(ctx, aggregated)
	verdict := aggregated[ev.TxID]

	switch verdict.Status {
	case wdk.AggregatedPostedTxIDSuccess:
		// False positive - the network knows the tx; make sure it's recorded as broadcast.
		return p.advanceToBroadcastedFromExternalEvent(ctx, ev, current)
	case wdk.AggregatedPostedTxIDServiceError:
		return p.requeueExternallyRejectedTx(ctx, ev, verdict, current)
	case wdk.AggregatedPostedTxIDDoubleSpend:
		return p.failExternallyRejectedTx(ctx, ev, verdict)
	case wdk.AggregatedPostedTxIDInvalidTx:
		fallthrough // unreachable: synthesizeRejectedVerdict always produces DoubleSpend; confirmDoubleSpends never emits InvalidTx
	default:
		return nil, fmt.Errorf("unexpected verdict status %s while processing external REJECTED event for tx %s", verdict.Status, ev.TxID)
	}
}

// requeueExternallyRejectedTx makes an unconfirmed external rejection actually retryable.
//
// Before this, the tx was left in its stored post-broadcast status (unmined) - a status
// the send_waiting task never scans ({unsent, sending} only) and that the proof-poll
// timeout could not reap either: a rejected tx is NOT_FOUND on the network, so the sync
// task filtered it out before its attempts counter ever grew. "Retryable" was therefore a
// state no retry path could reach, and the tx was dead while the wallet reported it
// in-flight (seen in production with Arcade's ancestor-limit rejections).
//
// The guarded demotion to unsent puts it back in send_waiting's queue (each retry lands
// after more ancestors mined, so an ancestor-limit rejection eventually clears); the
// rebroadcast budget still bounds the loop, failing terminally as invalid once exhausted.
// Transactions already queued (unsent/sending) or advanced (completed/terminal) are left
// untouched by the fromStatuses guard.
func (p *process) requeueExternallyRejectedTx(ctx context.Context, ev *wdk.BroadcastStatusEvent, verdict *wdk.AggregatedPostedTxID, current wdk.ProvenTxReqStatus) ([]wdk.TxSynchronizedStatus, error) {
	// unconfirmed is deliberately NOT requeueable: that status means a merkle proof
	// has already arrived, so a REJECTED event for it is stale by definition.
	// Requeueing would drop the proof, re-post a mined transaction, and - once the
	// rebroadcast budget ran out - fail it terminally as invalidTx. Marking a mined
	// transaction invalid is exactly the false-failure class the double-spend
	// confirmation path exists to prevent.
	fromStatuses := []wdk.ProvenTxReqStatus{
		wdk.ProvenTxStatusUnmined,
		wdk.ProvenTxStatusCallback,
		wdk.ProvenTxStatusUnknown,
	}
	notes := p.notesForPostBEEF(wdk.ProvenTxStatusUnsent, verdict, nil, nil, nil)

	newStatus, applied, err := p.knownTxRepo.RequeueKnownTxForRebroadcast(ctx, ev.TxID, p.maxRebroadcastAttempts, fromStatuses, notes)
	if err != nil {
		return nil, fmt.Errorf("failed to requeue externally rejected tx %s for rebroadcast: %w", ev.TxID, err)
	}
	if !applied {
		p.logger.InfoContext(
			ctx, "External REJECTED event could not be confirmed - tx is not in a requeueable status, leaving it untouched",
			slog.String("txID", ev.TxID),
			slog.String("storedStatus", string(current)),
		)
		return nil, nil
	}

	p.logger.InfoContext(
		ctx, "External REJECTED event could not be confirmed - requeueing the tx for rebroadcast",
		slog.String("txID", ev.TxID),
		slog.String("storedStatus", string(current)),
		slog.String("newStatus", string(newStatus)),
	)

	reference, labels, err := p.referenceAndLabelsForTxID(ctx, ev.TxID)
	if err != nil {
		return nil, err
	}

	return []wdk.TxSynchronizedStatus{{
		TxID:      ev.TxID,
		Status:    newStatus,
		Reference: reference,
		Labels:    labels,
	}}, nil
}

// failExternallyRejectedTx applies the confirmed double spend through the same terminal
// failure path the broadcast flow uses: Transaction -> failed, spent outputs restored,
// KnownTx -> doubleSpend (never clobbering a concurrently completed tx).
//
// All writes run in ONE storage transaction with the guarded KnownTx transition first:
// when the tx completed concurrently (e.g. during the multi-second network re-check of
// confirmDoubleSpends) NOTHING is written - failing the Transaction or resurrecting its
// spent inputs on a completed tx is exactly the false-double-spend corruption class this
// guards against.
func (p *process) failExternallyRejectedTx(ctx context.Context, ev *wdk.BroadcastStatusEvent, verdict *wdk.AggregatedPostedTxID) ([]wdk.TxSynchronizedStatus, error) {
	notes := p.notesForPostBEEF(wdk.ProvenTxStatusDoubleSpend, verdict, nil, nil, nil)
	skipStatuses := []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusCompleted, wdk.ProvenTxStatusDoubleSpend}
	applied, err := p.knownTxRepo.FailKnownTxAsDoubleSpend(ctx, ev.TxID, skipStatuses, notes)
	if err != nil {
		return nil, fmt.Errorf("failed to apply terminal double-spend failure for externally rejected tx %s: %w", ev.TxID, err)
	}
	if !applied {
		p.logger.InfoContext(
			ctx, "Externally rejected tx reached a protected status concurrently (e.g. completed) - leaving the stored state untouched",
			slog.String("txID", ev.TxID),
		)
		return nil, nil
	}

	reference, labels, err := p.referenceAndLabelsForTxID(ctx, ev.TxID)
	if err != nil {
		return nil, err
	}

	return []wdk.TxSynchronizedStatus{{
		TxID:      ev.TxID,
		Status:    wdk.ProvenTxStatusDoubleSpend,
		Reference: reference,
		Labels:    labels,
	}}, nil
}

// synthesizeRejectedVerdict shapes a REJECTED event like an aggregated broadcast verdict so
// the existing confirmDoubleSpends machinery (network re-check + evidence rules) can decide
// the outcome. Competing txs in the event are the positive double-spend evidence; without
// them the rejection is indistinguishable from propagation lag (kept retryable).
func synthesizeRejectedVerdict(ev *wdk.BroadcastStatusEvent) wdk.AggregatedPostFromBEEF {
	posted := &wdk.PostedTxID{
		TxID:         ev.TxID,
		Result:       wdk.PostedTxIDResultMissingInputs,
		DoubleSpend:  true,
		CompetingTxs: ev.CompetingTxs,
		Data:         ev.ExtraInfo,
		Notes:        wdk.HistoryNotes{externalStatusNote(ev).Note()},
	}
	if len(ev.CompetingTxs) > 0 {
		posted.Result = wdk.PostedTxIDResultDoubleSpend
	}

	competing := make(map[string]struct{}, len(ev.CompetingTxs))
	for _, competingTxID := range ev.CompetingTxs {
		competing[competingTxID] = struct{}{}
	}

	return wdk.AggregatedPostFromBEEF{
		ev.TxID: {
			TxID:             ev.TxID,
			TxIDResults:      []*wdk.PostedTxID{posted},
			Status:           wdk.AggregatedPostedTxIDDoubleSpend,
			DoubleSpendCount: 1,
			CompetingTxs:     competing,
		},
	}
}

// externalStatusNote builds the TxNote (history) entry recording an external status event.
func externalStatusNote(ev *wdk.BroadcastStatusEvent) history.Builder {
	note := history.NewBuilderFromNote(&wdk.HistoryNote{
		When: time.Now(),
		What: externalBroadcastStatusHistoryNote,
	}).WithAttribute("status", ev.Status)

	if ev.EventID != "" {
		note = note.WithAttribute("eventId", ev.EventID)
	}
	if ev.ExtraInfo != "" {
		note = note.WithAttribute("info", ev.ExtraInfo)
	}
	if len(ev.CompetingTxs) > 0 {
		note = note.WithAttribute("competingTxs", strings.Join(ev.CompetingTxs, ","))
	}

	return note
}

// referenceAndLabelsForTxID looks up the user-facing reference and labels for a txID, used
// to build the TxSynchronizedStatus results (same shape as synchronizeTxStatuses).
func (p *process) referenceAndLabelsForTxID(ctx context.Context, txID string) (string, []string, error) {
	references, err := p.txRepo.FindReferencesByTxIDs(ctx, []string{txID})
	if err != nil {
		return "", nil, fmt.Errorf("failed to find references for txID %s: %w", txID, err)
	}

	labels, err := p.txRepo.GetLabelsForTxIDs(ctx, []string{txID})
	if err != nil {
		return "", nil, fmt.Errorf("failed to find labels for txID %s: %w", txID, err)
	}

	return references[txID], labels[txID], nil
}
