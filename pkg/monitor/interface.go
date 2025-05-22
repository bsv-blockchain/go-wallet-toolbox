package monitor

import "context"

// MinimalStorageInterface defines the minimum storage functionality used by the monitor.
type MinimalStorageInterface interface {
	SynchronizeTransactionStatuses(ctx context.Context) error
}
