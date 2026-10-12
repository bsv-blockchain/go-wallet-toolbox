## MONITOR: BSV Wallet Toolbox API Documentation

The documentation is split into various pages; this page covers the Monitor and related API.

To function properly, a wallet must be able to perform a number of housekeeping tasks:

- Ensure transactions are sent to the network without slowing application flow or when created while offline.
- Obtain and merge proofs when transactions are mined.
- Detect and propagate transactions that fail due to double-spend, reorgs, or other reasons.

These tasks are the responsibility of the Monitor.

### Quick start

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "time"

    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Minimal in-memory storage demonstrating the required interface.
type memoryStorage struct{}

func (memoryStorage) SynchronizeTransactionStatuses(ctx context.Context) error { return nil }
func (memoryStorage) SendWaitingTransactions(ctx context.Context, minTransactionAge time.Duration) error { return nil }
func (memoryStorage) AbortAbandoned(ctx context.Context) error { return nil }
func (memoryStorage) UnFail(ctx context.Context) error { return nil }

func main() {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    storage := memoryStorage{}

    daemon, err := monitor.NewDaemon(logger, storage)
    if err != nil { panic(err) }

    cfg := defs.DefaultMonitorConfig()
    if err := daemon.Start(cfg.Tasks.EnabledTasks()); err != nil { panic(err) }

    // Keep running to allow scheduled tasks to execute.
    select {}
}
```

### SynchronizeTransactionStatuses
- Reconciles locally stored transaction records with the latest on-chain state.
- Updates statuses (for example, pending → confirmed/failed) and stores proofs when available.
- Ensures the wallet’s view of each transaction matches the network.

```go
// One-off (assumes you have ctx and a storage implementing MonitoredStorage)
if err := storage.SynchronizeTransactionStatuses(ctx); err != nil {
    // handle error
}

// Schedule only this task (assumes you created daemon)
_ = daemon.Start(map[defs.MonitorTask]defs.TaskConfig{
    defs.CheckForProofsMonitorTask: {
        Enabled:          true,
        IntervalSeconds:  60,    // run every 60s
        StartImmediately: true,
    },
})
```

### SendWaitingTransactions
- Broadcasts transactions that are queued in a “waiting” state.
- Respects a minimum age threshold so very new items can settle before send attempts.
- On the first run, sends all waiting transactions; on subsequent runs, sends only those older than the threshold.

```go
// One-off with minimum age (requires import of time)
if err := storage.SendWaitingTransactions(ctx, 5*time.Minute); err != nil {
    // handle error
}

// Schedule only this task
_ = daemon.Start(map[defs.MonitorTask]defs.TaskConfig{
    defs.SendWaitingMonitorTask: {
        Enabled:          true,
        IntervalSeconds:  300,   // 5 minutes
        StartImmediately: true,  // send immediately on startup
    },
})
```

### AbortAbandoned
- Marks transactions as failed when they appear abandoned (stuck or no longer progressing).
- Frees up workflows and resources so these items can be retried or handled explicitly.
- Helps prevent indefinite “in-flight” states.

```go
// One-off
if err := storage.AbortAbandoned(ctx); err != nil {
    // handle error
}

// Schedule only this task
_ = daemon.Start(map[defs.MonitorTask]defs.TaskConfig{
    defs.FailAbandonedMonitorTask: {
        Enabled:         true,
        IntervalSeconds: 300, // 5 minutes
    },
})
```

### UnFail
- Rechecks transactions previously marked as failed to see if they actually succeeded later.
- If confirmation is detected, clears the failure and updates to the accurate status/proof.
- Corrects false negatives from transient errors or external broadcasts.

```go
// One-off
if err := storage.UnFail(ctx); err != nil {
    // handle error
}

// Schedule only this task
_ = daemon.Start(map[defs.MonitorTask]defs.TaskConfig{
    defs.UnFailMonitorTask: {
        Enabled:         true,
        IntervalSeconds: 600, // 10 minutes
    },
})
```

### Reorg handling
When chaintracks is enabled, the monitor subscribes to reorg events. Each event lists the hashes of the blocks that are no longer on the main chain. Set `monitor.reorg.mode` to choose how those blocks are handled.

**`immediate` (default)**
1. `HandleReorg` clears the merkle proof (path, root, height, block hash) of every known transaction proven in an orphaned block.
2. Each of those transactions is set to status `reorg` and gets a `reorgInvalidatedProof` history note.
3. The `check_for_proofs` task (`SynchronizeTransactionStatuses`) re-proves them like any other unproven transaction, and they return to `completed` once mined again.

**`delayed` (TypeScript `TaskReorg` behaviour)**
1. Orphaned block hashes are queued in memory and nothing is changed while the tip may still be unstable. The queue is de-duplicated and bounded by `max_queued_headers`; when it is full the oldest entry is dropped.
2. After `age_seconds`, `ReproveHeader` fetches a fresh merkle path for every transaction proven in that block and sorts it:
   - **updated**: the proof is now in another block and its root is confirmed by the chain tracker. The new proof is stored and a `tx_proven` event is emitted.
   - **unchanged**: the services still return the proof in the orphaned block.
   - **unavailable**: there is no proof yet, or it does not validate.
3. A block with unchanged or unavailable transactions is queued again for another `age_seconds`. After `max_retries` attempts it is dropped and the original proofs are kept.

```yaml
monitor:
  reorg:
    mode: delayed          # immediate | delayed
    age_seconds: 600       # wait before the first attempt and between retries
    max_retries: 3
    max_queued_headers: 4096
```

**Which mode to choose**
- `immediate` never keeps a proof from an orphaned block. If the tip flips back and forth it does extra work, but `check_for_proofs` repairs it. It suits most deployments.
- `delayed` matches the TypeScript toolbox and avoids touching proofs while a reorg settles. It can keep a stale proof until a replacement is confirmed, or for good after `max_retries`. Its queue is in memory, so a restart forgets pending blocks. Every instance handles the events it receives; this is safe because re-proving is idempotent.
