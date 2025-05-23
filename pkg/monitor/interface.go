package monitor

import "context"

// MonitoredStorage defines the minimum storage functionality used by the monitor.
type MonitoredStorage interface {
	SynchronizeTransactionStatuses(ctx context.Context) error
}
