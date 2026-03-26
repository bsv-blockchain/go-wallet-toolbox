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
}
