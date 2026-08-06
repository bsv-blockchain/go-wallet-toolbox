package actions

import (
	"context"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// abortOnFailureTimeout bounds the compensating abort issued when an action cannot be
// carried past storage.CreateAction. It runs on a context detached from the (possibly
// already canceled) request context.
const abortOnFailureTimeout = 10 * time.Second

// abortActionAfterFailure releases the inputs reserved by an action that will never be
// signed or processed: storage keeps it in status 'unsigned' with no txid, so its inputs
// (custom inputs included) stay locked until the fail_abandoned sweep runs.
//
// It is best-effort - an abort failure is only logged, so the original error reaches the
// caller unchanged. Callers must not invoke it once the action was handed to
// storage.ProcessAction: from that point on the transaction may carry broadcast evidence
// and releasing its inputs could result in a double-spend.
func abortActionAfterFailure(ctx context.Context, logger *slog.Logger, storage WalletStorageAbortAction, reference string, cause error) {
	if reference == "" {
		return
	}

	// The request context is very likely the reason we are here (cancellation, timeout),
	// so detach from it - the compensation must still run.
	abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), abortOnFailureTimeout)
	defer cancel()

	logger.InfoContext(abortCtx, "aborting unsigned action to release reserved inputs",
		slog.String("reference", reference),
		logging.Error(cause),
	)

	result, abortErr := storage.AbortAction(abortCtx, wdk.AbortActionArgs{
		Reference: primitives.Base64String(reference),
	})
	if abortErr != nil {
		logger.ErrorContext(abortCtx, "failed to abort unsigned action, reserved inputs stay locked until the fail_abandoned sweep",
			slog.String("reference", reference),
			logging.Error(abortErr),
		)
		return
	}

	logger.InfoContext(abortCtx, "unsigned action aborted",
		slog.String("reference", reference),
		slog.Bool("aborted", result.Aborted),
	)
}
