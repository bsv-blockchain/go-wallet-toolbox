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
	AbortAction(ctx context.Context, userID int, args *wdk.AbortActionArgs) (*wdk.AbortActionResult, error)
}

// releaseReservedInputsTimeout bounds a compensating abort. It runs on a context detached
// from the request context, which may already be canceled.
const releaseReservedInputsTimeout = 10 * time.Second

// releaseReservedInputs aborts the action identified by reference so that the inputs it
// reserved become spendable again immediately, instead of after the fail_abandoned sweep.
//
// It is best-effort: problems are logged, never returned, so the original error reaches the
// caller unchanged. Callers must only use it for failures that provably happened before the
// action could reach the network; the abort itself re-checks that the action is still
// abortable, so anything with broadcast evidence is refused.
func releaseReservedInputs(ctx context.Context, logger *slog.Logger, aborter actionAborter, userID int, reference string, cause error) {
	if aborter == nil || reference == "" {
		return
	}

	logger = logger.With(
		logging.UserID(userID),
		logging.Reference(reference),
	)

	logger.InfoContext(ctx, "releasing inputs reserved by an action that cannot be completed",
		logging.Error(cause),
	)

	// The request context may already be canceled (that can be the very reason we are here),
	// so detach from it - the release must still run.
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseReservedInputsTimeout)
	defer cancel()

	result, err := aborter.AbortAction(releaseCtx, userID, &wdk.AbortActionArgs{
		Reference: primitives.Base64String(reference),
	})
	if err != nil {
		logger.WarnContext(releaseCtx, "failed to release reserved inputs, they stay reserved until the fail_abandoned sweep",
			logging.Error(err),
		)
		return
	}

	logger.InfoContext(releaseCtx, "reserved inputs released",
		slog.Bool("aborted", result.Aborted),
	)
}
