package actions

import (
	"context"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// actionAborter releases the inputs reserved by an action that will not be completed.
type actionAborter interface {
	// AbortAction releases an action identified by its reference. Used while the action has
	// no txid yet, i.e. before ProcessAction committed it.
	AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error)
	// AbortUnbroadcastTx releases an action that already carries a txid, parking its shared
	// KnownTx so no broadcast pipeline can pick the transaction up afterwards.
	AbortUnbroadcastTx(ctx context.Context, userID int, txID string) error
}

// releaseTimeout bounds a compensating release. It runs on a context detached from the
// request context, which may already be canceled.
const releaseTimeout = 10 * time.Second

// release is the compensation for an action that reserved inputs but cannot be completed.
//
// It is a state machine with exactly three transitions, so that error handling needs no
// per-failure bookkeeping:
//
//	armByReference / armByTxID - the action exists and holds reserved inputs, but provably
//	                             cannot reach the network yet
//	disarm                     - point of no return: from here the transaction may be posted,
//	                             so its inputs must stay spent
//	onError                    - fires the release if (and only if) it is still armed
//
// Entry points call onError from a single defer, which means every failure - including ones
// added later - releases the inputs, unless it happens after the point of no return.
//
// A nil *release is a valid, permanently disarmed release: callers that own no action of
// their own (the send_waiting sweep) pass nil.
type release struct {
	logger  *slog.Logger
	aborter actionAborter
	userID  int

	reference string
	txID      string
	armed     bool
}

func newRelease(logger *slog.Logger, aborter actionAborter, userID int) *release {
	return &release{logger: logger, aborter: aborter, userID: userID}
}

// armByReference arms the release for an action that has no txid yet.
func (r *release) armByReference(reference string) {
	if r == nil || reference == "" {
		return
	}
	r.reference = reference
	r.armed = true
}

// armByTxID arms the release for an action that already carries a txid, which additionally
// requires parking its shared KnownTx.
func (r *release) armByTxID(txID string) {
	if r == nil || txID == "" {
		return
	}
	r.txID = txID
	r.armed = true
}

// disarm marks the point of no return: the transaction may reach the network from here on,
// so releasing its inputs could result in a double spend.
func (r *release) disarm() {
	if r == nil {
		return
	}
	r.armed = false
}

// onError releases the reserved inputs when the operation failed while still armed. It is
// best-effort: problems are logged, never returned, so the original error is unaffected.
func (r *release) onError(ctx context.Context, cause error) {
	if r == nil || cause == nil || !r.armed || r.aborter == nil {
		return
	}
	r.armed = false

	logger := r.logger.With(logging.UserID(r.userID))
	if r.txID != "" {
		logger = logger.With(slog.String("txID", r.txID))
	} else {
		logger = logger.With(logging.Reference(r.reference))
	}

	logger.InfoContext(ctx, "releasing inputs reserved by an action that cannot be completed",
		logging.Error(cause),
	)

	// The request context may already be canceled (that can be the very reason we are here),
	// so detach from it - the release must still run.
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()

	if err := r.abort(releaseCtx); err != nil {
		logger.WarnContext(releaseCtx, "failed to release reserved inputs, they stay reserved until the fail_abandoned sweep",
			logging.Error(err),
		)
		return
	}

	logger.InfoContext(releaseCtx, "reserved inputs released")
}

func (r *release) abort(ctx context.Context) error {
	if r.txID != "" {
		return r.aborter.AbortUnbroadcastTx(ctx, r.userID, r.txID)
	}

	_, err := r.aborter.AbortAction(ctx, r.userID, &wdk.AbortActionArgs{
		Reference: primitives.Base64String(r.reference),
	})
	return err
}
