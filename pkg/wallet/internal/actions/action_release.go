package actions

import (
	"context"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// releaseTimeout bounds a compensating release. It runs on a context detached from the
// request context, which may already be canceled.
const releaseTimeout = 10 * time.Second

// release is the compensation for an action that storage created (and whose inputs it
// reserved) but that this wallet cannot carry to completion.
//
// It is a state machine with three transitions, so error handling needs no per-failure
// bookkeeping:
//
//	arm     - storage holds an action with reserved inputs that only this flow can complete
//	disarm  - point of no return: the action was handed to ProcessAction, so it may already
//	          carry broadcast evidence and its inputs must stay spent
//	onError - releases the action if (and only if) it is still armed
//
// Entry points call onError from a single defer, so every failure - including ones added
// later - releases the inputs, unless it happens after the point of no return.
type release struct {
	logger  *slog.Logger
	storage WalletStorageAbortAction

	reference string
	armed     bool

	// onRelease runs after a fired release, e.g. to drop cached state tied to the reference.
	onRelease func()
}

func newRelease(logger *slog.Logger, storage WalletStorageAbortAction) *release {
	return &release{logger: logger, storage: storage}
}

// arm marks the action as releasable. Calling it without a reference is a no-op: without one
// nothing identifies the action to release.
func (r *release) arm(reference string) {
	if reference == "" {
		return
	}
	r.reference = reference
	r.armed = true
}

// disarm marks the point of no return.
func (r *release) disarm() {
	r.armed = false
}

// onError releases the action when the flow failed while still armed. It is best-effort:
// problems are logged, never returned, so the original error is unaffected.
func (r *release) onError(ctx context.Context, cause error) {
	if cause == nil || !r.armed || r.storage == nil {
		return
	}
	r.armed = false

	logger := r.logger.With(slog.String("reference", r.reference))
	logger.InfoContext(ctx, "aborting action that cannot be completed, to release its reserved inputs",
		logging.Error(cause),
	)

	// The request context is very likely the reason we are here (cancellation, timeout), so
	// detach from it - the compensation must still run.
	abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()

	result, err := r.storage.AbortAction(abortCtx, wdk.AbortActionArgs{
		Reference: primitives.Base64String(r.reference),
	})
	if err != nil {
		logger.ErrorContext(abortCtx, "failed to abort action, its inputs stay reserved until the fail_abandoned sweep",
			logging.Error(err),
		)
	} else {
		logger.InfoContext(abortCtx, "action aborted", slog.Bool("aborted", result.Aborted))
	}

	if r.onRelease != nil {
		r.onRelease()
	}
}
