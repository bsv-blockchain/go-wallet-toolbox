package whatsonchain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

func waitOrCancel(ctx context.Context, delay time.Duration, txid string) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled while waiting for tx %s: %w", txid, ctx.Err())
	case <-time.After(delay):
		return nil
	}
}

func classifyBroadcastStatus(status BroadcastStatus) (wdk.PostedTxIDResultStatus, []string) {
	switch status {
	case StatusSuccess:
		return wdk.PostedTxIDResultSuccess, nil
	case StatusAlreadyBroadcasted:
		return wdk.PostedTxIDResultAlreadyKnown, []string{"Transaction already in mempool"}
	case StatusDoubleSpend:
		return wdk.PostedTxIDResultDoubleSpend, []string{"Double spend detected"}
	case StatusMissingInputs:
		return wdk.PostedTxIDResultMissingInputs, []string{"Missing inputs detected"}
	case StatusError:
		return wdk.PostedTxIDResultError, []string{"Broadcast status error"}
	default:
		return wdk.PostedTxIDResultError, []string{fmt.Sprintf("Unknown error: unexpected BroadcastStatus value '%v'", status)}
	}
}

func containsI(subject string, contains ...string) bool {
	subject = strings.ToLower(subject)
	for _, c := range contains {
		if strings.Contains(subject, strings.ToLower(c)) {
			return true
		}
	}
	return false
}

func firstNonNilError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
