package monitor

import (
	"context"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// MonitoredStorage defines the minimum storage functionality used by the monitor.
type MonitoredStorage interface {
	SynchronizeTransactionStatuses(ctx context.Context) ([]wdk.TxSynchronizedStatus, error)
	SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) (*wdk.ProcessActionResult, error)
	AbortAbandoned(ctx context.Context) error
	UnFail(ctx context.Context) error

	HandleReorg(ctx context.Context, orphanedBlockHashes []string) error
	ProcessNewTip(ctx context.Context, height uint32, hash string) ([]wdk.TxSynchronizedStatus, error)

	// ProcessExternalTxStatusUpdate applies a broadcaster-pushed lifecycle update
	// (e.g. from the Arcade SSE stream).
	ProcessExternalTxStatusUpdate(ctx context.Context, ev wdk.BroadcastStatusEvent) ([]wdk.TxSynchronizedStatus, error)

	// GetKeyValue / SetKeyValue expose the key_value table for small instance state
	// (e.g. the SSE replay cursor).
	GetKeyValue(ctx context.Context, key string) ([]byte, bool, error)
	SetKeyValue(ctx context.Context, key string, value []byte) error
}
