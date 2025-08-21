package monitor

import (
	"context"
	"time"
)

// MonitoredStorage defines the minimum storage functionality used by the monitor.
type MonitoredStorage interface {
	SynchronizeTransactionStatuses(ctx context.Context) error
	SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) error
}
