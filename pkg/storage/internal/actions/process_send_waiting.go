package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	sendWaitingMaxPages     = 10
	sendWaitingItemsPerPage = 1000

	// sweepBatchDurationSeed is the per-batch estimate used before any batch of
	// this run has been timed.
	sweepBatchDurationSeed = 1 * time.Second
	// sweepMinReserve floors the reserve so a run of fast batches cannot shrink
	// the estimate to a value that leaves no room for a slow outlier.
	sweepMinReserve = 2 * time.Second
	// sweepEWMAWeight is the weight of the newest sample in the running average.
	sweepEWMAWeight = 0.3
)

var statusesOfWaitingTxs = []wdk.ProvenTxReqStatus{
	wdk.ProvenTxStatusUnsent,
	wdk.ProvenTxStatusSending,
}

func (p *process) SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-SendWaitingTransactions")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	log := p.logger.With("action", "sendWaitingTransactions").With(slog.Duration("minTransactionAge", minTransactionAge))
	log.InfoContext(ctx, "Attempting to send waiting transactions")

	lockAcquired := p.sendWaitingLock.TryLock()
	if !lockAcquired {
		log.WarnContext(ctx, "SendWaitingTransactions is already running, skipping this run")
		return nil, nil
	}
	defer p.sendWaitingLock.Unlock()

	// Report any queue overflow still held back by the throttle. These are the very
	// transactions this sweep is about to pick up, so cause and effect land together.
	p.flushBroadcasterOverflow(ctx)

	until := queryopts.Until{
		Time: time.Now().Add(-minTransactionAge),
	}

	batchesToBroadcast, collectErr := p.collectWaitingBatches(ctx, log, until)
	if collectErr != nil {
		err = collectErr
		return nil, err
	}

	if len(batchesToBroadcast) == 0 {
		log.InfoContext(ctx, "No transactions found to send")
		return nil, nil
	}

	log.InfoContext(ctx, "Found transactions to send", "batchesCount", len(batchesToBroadcast))

	results := &wdk.ProcessActionResult{}
	var batchErrs []error
	budget := newSweepBudget(ctx)
	processed := 0

	for _, batch := range batchesToBroadcast {
		batchName, txIDs := batch.name, batch.txIDs

		if !budget.allows(ctx) {
			// The monitor budgets the whole sweep, not the individual batch. Stopping
			// here defers the remainder to the next run as one summary line, instead of
			// letting every remaining batch fail on its first query and emit an error.
			// Because the batches are ordered oldest-first, what is deferred is the
			// newest tail - never a parent whose child has already gone out.
			log.WarnContext(
				ctx, "send_waiting budget exhausted; deferring remaining batches to the next run",
				slog.Int("processed", processed),
				slog.Int("deferred", len(batchesToBroadcast)-processed),
			)
			break
		}

		log.InfoContext(ctx, "Processing batch", "batchName", batchName, "txIDs", txIDs)

		batchStart := time.Now()
		res, batchErr := p.broadcastDelayedTransaction(ctx, log, txIDs)
		budget.observe(time.Since(batchStart))
		processed++

		if batchErr != nil {
			// Continue-on-error: a single bad batch must not strand the rest. A context
			// error is not a batch failure - it says the run ran out of budget - so it is
			// reported as a deferral and kept out of the joined error, which the monitor
			// turns into a "Task failed".
			if isContextErr(batchErr) {
				log.WarnContext(
					ctx, "Waiting batch deferred to the next run: sweep is out of budget",
					slog.String("batchName", batchName), slog.Int("txCount", len(txIDs)),
				)
				continue
			}
			log.ErrorContext(
				ctx, "Failed to broadcast waiting batch",
				slog.String("batchName", batchName), slog.Int("txCount", len(txIDs)), "error", batchErr,
			)
			batchErrs = append(batchErrs, batchErr)
			continue
		}
		if res != nil {
			results.SendWithResults = append(results.SendWithResults, res.SendWithResults...)
			results.NotDelayedResults = append(results.NotDelayedResults, res.NotDelayedResults...)
		}
	}

	// TODO: Keep in mind that the transactions above max attempts will be reviewed in another "reviewStatus" periodic task.

	// Return the assembled result together with any hard batch errors (joined). Soft, per-tx
	// failures (e.g. a service error that leaves a tx still "sending") are reported inside the
	// result's per-tx entries, not as a returned error.
	err = errors.Join(batchErrs...)
	return results, err
}

// waitingBatch is one unit of broadcast work: the transactions of a single submitted
// batch, or a lone transaction that was submitted without one.
type waitingBatch struct {
	name  string
	txIDs []string
}

// collectWaitingBatches pages through the waiting transactions and groups them the
// way they were submitted: transactions sharing a batch go out together, and a
// transaction without one becomes a single-member batch keyed by its own txID.
//
// The result is ordered by creation time, and stays that way through broadcasting:
// Arcade forwards to Teranode in receive order, so a child spending an unconfirmed
// parent has to be posted after it. The query returns rows oldest-first, a batch takes
// the position of its oldest member, and members keep their order within it. A batch
// spanning a long stretch of time could in principle still straddle a lone transaction
// that belongs in its middle, but a batch is created by a single processAction call,
// so its members share a creation instant.
//
// A page that fails after earlier pages succeeded does not sink the whole sweep -
// broadcasting what was already read makes progress, and the remainder is picked
// up by the next run.
func (p *process) collectWaitingBatches(ctx context.Context, log *slog.Logger, until queryopts.Until) ([]waitingBatch, error) {
	// Oldest first, with the primary key breaking ties: created_at ties for every member of a
	// batch, because one processAction call writes them all, and an unstable order under OFFSET
	// paging can drop a row from one page and repeat it on the next.
	paging := queryopts.Paging{Limit: sendWaitingItemsPerPage, Sort: "asc", ThenBy: "tx_id"}
	var batchesToBroadcast []waitingBatch
	positionOf := make(map[string]int)

	for range sendWaitingMaxPages {
		if ctx.Err() != nil {
			break
		}

		txIDsPage, pageErr := p.knownTxRepo.FindKnownTxIDsByStatuses(
			ctx,
			statusesOfWaitingTxs,
			queryopts.WithUntil(until),
			queryopts.WithPage(paging),
		)
		if pageErr != nil {
			if isContextErr(pageErr) || len(batchesToBroadcast) > 0 {
				log.WarnContext(
					ctx, "Failed to read a page of waiting transactions; continuing with the pages already read",
					slog.Int("batchesSoFar", len(batchesToBroadcast)), "error", pageErr,
				)
				break
			}
			return nil, fmt.Errorf("failed to find known txs by statuses: %w", pageErr)
		}

		for _, item := range txIDsPage {
			name := item.TxID
			if item.Batch != nil {
				name = *item.Batch
			}

			if position, seen := positionOf[name]; seen {
				batchesToBroadcast[position].txIDs = append(batchesToBroadcast[position].txIDs, item.TxID)
				continue
			}

			positionOf[name] = len(batchesToBroadcast)
			batchesToBroadcast = append(batchesToBroadcast, waitingBatch{name: name, txIDs: []string{item.TxID}})
		}

		if len(txIDsPage) < sendWaitingItemsPerPage {
			break
		}

		paging.Next()
	}

	return batchesToBroadcast, nil
}

// sweepBudget decides whether another batch still fits in the run's deadline.
//
// The monitor gives the entire sweep one budget, so a batch started too late dies
// mid-flight - and a batch that dies after ClaimKnownTxsForBroadcast is left
// marked sending/was_broadcast without ever having been posted. Keeping a reserve
// shrinks that window.
type sweepBudget struct {
	deadline    time.Time
	hasDeadline bool
	// avgBatch is an EWMA of observed batch durations. A fixed reserve would be
	// wrong at both ends: a single-tx batch runs in a fraction of a second, while
	// a chained batch costs one HTTP post per member, because PostFromBEEF walks
	// the batch transaction by transaction.
	avgBatch time.Duration
}

func newSweepBudget(ctx context.Context) *sweepBudget {
	deadline, hasDeadline := ctx.Deadline()
	return &sweepBudget{
		deadline:    deadline,
		hasDeadline: hasDeadline,
		avgBatch:    sweepBatchDurationSeed,
	}
}

// allows reports whether another batch should be started.
func (b *sweepBudget) allows(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	if !b.hasDeadline {
		return true
	}
	return time.Until(b.deadline) >= b.reserve()
}

func (b *sweepBudget) reserve() time.Duration {
	return max(2*b.avgBatch, sweepMinReserve)
}

func (b *sweepBudget) observe(batchDuration time.Duration) {
	b.avgBatch = time.Duration(sweepEWMAWeight*float64(batchDuration) + (1-sweepEWMAWeight)*float64(b.avgBatch))
}

// isContextErr reports whether err is the run running out of budget (or being
// cancelled) rather than anything wrong with the work itself.
func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// broadcastDelayedTransaction broadcasts a single waiting batch and returns its result and error
// instead of logging them away. A hard failure (broadcastTxs returns an error) is surfaced to the
// caller so it can be aggregated; per-tx problematic outcomes are left in the returned result.
func (p *process) broadcastDelayedTransaction(ctx context.Context, log *slog.Logger, txIDs []string) (*wdk.ProcessActionResult, error) {
	log.InfoContext(ctx, "Attempting to broadcast transactions", "txIDs", txIDs)

	// nil release: the sweep introduces no transaction of its own. These are queued
	// transactions whose broadcast is being retried, so a failure must leave them queued for
	// the next run rather than abandon them.
	result, err := p.broadcastTxs(ctx, txIDs, false, nil)
	if err != nil {
		return nil, fmt.Errorf("broadcast of waiting batch %v failed: %w", txIDs, err)
	}

	for _, res := range result.NotDelayedResults {
		if res.Status != wdk.ReviewActionResultStatusSuccess {
			log.WarnContext(ctx, "Problematic broadcast result", "txID", res.TxID, "status", res.Status)
		}
	}

	return result, nil
}
