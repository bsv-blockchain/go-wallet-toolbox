package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	// BackgroundBroadcasterWorkerCount is the default number of workers that
	// process broadcast items.
	//
	// Worker count is the ceiling on network acceptance for delayed broadcast,
	// and therefore on any throughput that depends on outputs becoming
	// spendable: an output is only marked spendable once its transaction is
	// accepted by the network (see actions.updateSingleTx). At ~300ms per post
	// this default sustains only ~33 tx/s, which a high-rate workload will
	// overflow — its queued transactions then fall back to the far slower
	// send_waiting cron and risk being aborted by the abandon sweep. Deployments
	// expecting that load raise it via Sizing (the throughput strategy does).
	//
	// The default stays small because every storage provider starts this pool
	// eagerly, including the many short-lived ones a test suite creates.
	BackgroundBroadcasterWorkerCount = 10

	// BackgroundBroadcasterChannelSize is the default buffer size for the
	// broadcast channel. Add() drops to the cron fallback once it is full.
	BackgroundBroadcasterChannelSize = 1000

	// defaultMaxParentWait bounds how long a child waits for its unconfirmed
	// parent to be posted to the broadcaster before it is posted anyway. It is a
	// safety net against a parent that is never broadcast here (e.g. an external
	// unconfirmed source): the child is not held forever. In steady state the
	// parent posts within milliseconds and the child is released immediately, so
	// this deadline is rarely reached.
	defaultMaxParentWait = 30 * time.Second

	// parentWaitSweepInterval is how often parked children are re-checked for an
	// expired parent-wait deadline.
	parentWaitSweepInterval = time.Second
)

// Sizing configures the broadcaster's capacity. Zero values select the
// defaults above.
type Sizing struct {
	Workers     int
	ChannelSize int
	// MaxParentWait bounds how long a child is held waiting for its unconfirmed
	// parent to be posted before being force-posted. Zero selects the default.
	MaxParentWait time.Duration
}

func (s Sizing) workers() int {
	if s.Workers <= 0 {
		return BackgroundBroadcasterWorkerCount
	}
	return s.Workers
}

func (s Sizing) channelSize() int {
	if s.ChannelSize <= 0 {
		return BackgroundBroadcasterChannelSize
	}
	return s.ChannelSize
}

func (s Sizing) maxParentWait() time.Duration {
	if s.MaxParentWait <= 0 {
		return defaultMaxParentWait
	}
	return s.MaxParentWait
}

type broadcaster interface {
	BackgroundBroadcast(ctx context.Context, beef *transaction.Beef, txIDs []string) ([]wdk.ReviewActionResult, error)
}

type BackgroundBroadcaster struct {
	sizing           Sizing
	ctx              context.Context
	cancel           context.CancelFunc
	broadcastChannel chan broadcastItem
	wg               sync.WaitGroup
	logger           *slog.Logger
	broadcastHandler broadcaster

	// optional notification channel
	txBroadcastedChannel chan<- wdk.CurrentTxStatus

	// Dependency ordering: Arcade forwards to Teranode in the order it receives
	// transactions, so a child that spends an unconfirmed parent must be POSTed
	// after that parent — otherwise Teranode rejects the child (its input isn't
	// in the UTXO set yet). Because parent and child arrive as independent
	// concurrent items drained by many workers, we gate here: a child is held
	// until every unconfirmed parent it spends has been posted.
	depMu   sync.Mutex
	posted  map[string]struct{}          // txids whose Arcade POST returned successfully
	waiting map[string][]broadcastItem   // children parked under an unposted parent txid

	stopOnce sync.Once
}

type broadcastItem struct {
	beef  *transaction.Beef
	txIDs []string
	// deadline is when the item may be posted regardless of unposted parents.
	deadline time.Time
}

func NewBackgroundBroadcaster(ctx context.Context, parentLogger *slog.Logger, broadcastHandler broadcaster, txBroadcastedChannel chan<- wdk.CurrentTxStatus, sizing Sizing) *BackgroundBroadcaster {
	bbContext, cancel := context.WithCancel(ctx)
	logger := logging.Child(parentLogger, "BackgroundBroadcaster")
	return &BackgroundBroadcaster{
		sizing:               sizing,
		ctx:                  bbContext,
		cancel:               cancel,
		broadcastChannel:     make(chan broadcastItem, sizing.channelSize()),
		logger:               logger,
		broadcastHandler:     broadcastHandler,
		txBroadcastedChannel: txBroadcastedChannel,
		posted:               make(map[string]struct{}),
		waiting:              make(map[string][]broadcastItem),
	}
}

func (bb *BackgroundBroadcaster) Start() {
	for i := 0; i < bb.sizing.workers(); i++ {
		bb.wg.Add(1)
		go bb.worker()
	}
	bb.wg.Add(1)
	go bb.parentWaitSweeper()
}

func (bb *BackgroundBroadcaster) Stop() {
	bb.stopOnce.Do(func() {
		bb.cancel()
		bb.wg.Wait()
		close(bb.broadcastChannel)
	})
}

func (bb *BackgroundBroadcaster) Add(beef *transaction.Beef, txIDs []string) (added bool) {
	bb.logger.InfoContext(bb.ctx, "Adding new beef to delayed broadcast", "txIDs", txIDs)
	item := broadcastItem{beef: beef, txIDs: txIDs, deadline: time.Now().Add(bb.sizing.maxParentWait())}
	select {
	case bb.broadcastChannel <- item:
		return true
	default:
		return false
	}
}

func (bb *BackgroundBroadcaster) worker() {
	defer bb.wg.Done()

	for {
		select {
		case <-bb.ctx.Done():
			return
		case item, ok := <-bb.broadcastChannel:
			if !ok {
				return
			}
			bb.process(item)
		}
	}
}

// process posts the item once its unconfirmed parents have been posted (or its
// wait deadline has passed), parking it otherwise.
func (bb *BackgroundBroadcaster) process(item broadcastItem) {
	if !time.Now().After(item.deadline) {
		bb.depMu.Lock()
		if parent := bb.firstUnpostedParentLocked(item); parent != "" {
			bb.waiting[parent] = append(bb.waiting[parent], item)
			bb.depMu.Unlock()
			bb.logger.DebugContext(bb.ctx, "holding child until parent is posted", "parent", parent, "txIDs", item.txIDs)
			return
		}
		bb.depMu.Unlock()
	}

	if err := bb.broadcast(&item); err != nil {
		bb.logger.ErrorContext(bb.ctx, "Failed to broadcast transaction", "error", err, "txIDs", item.txIDs)
		return
	}
	bb.logger.InfoContext(bb.ctx, "Successfully broadcasted transaction", "txIDs", item.txIDs)
	bb.markPostedAndRelease(item.txIDs)
}

// firstUnpostedParentLocked returns the txid of the first unconfirmed parent that
// this item spends and that has not yet been posted, or "" if it is ready.
// A parent is gate-worthy only when it is present in the item's beef as a raw,
// un-mined transaction (transaction.RawTx): a mined parent (RawTxAndBumpIndex)
// is already in the UTXO set, and a txid-only / absent parent is not something
// this broadcaster is responsible for. Caller must hold depMu.
func (bb *BackgroundBroadcaster) firstUnpostedParentLocked(item broadcastItem) string {
	if item.beef == nil {
		return ""
	}
	for _, txID := range item.txIDs {
		hash, err := chainhash.NewHashFromHex(txID)
		if err != nil {
			continue
		}
		tx := item.beef.FindTransactionByHash(hash)
		if tx == nil {
			continue
		}
		for _, in := range tx.Inputs {
			if in.SourceTXID == nil {
				continue
			}
			parentBeefTx := item.beef.Transactions[*in.SourceTXID]
			if parentBeefTx == nil || parentBeefTx.DataFormat != transaction.RawTx {
				continue // absent, txid-only, or mined (bump) parent → no gating
			}
			parentID := in.SourceTXID.String()
			if _, ok := bb.posted[parentID]; ok {
				continue
			}
			return parentID
		}
	}
	return ""
}

// markPostedAndRelease records the posted txids and re-queues any children that
// were waiting on them.
func (bb *BackgroundBroadcaster) markPostedAndRelease(txIDs []string) {
	bb.depMu.Lock()
	var released []broadcastItem
	for _, txID := range txIDs {
		bb.posted[txID] = struct{}{}
		if children, ok := bb.waiting[txID]; ok {
			released = append(released, children...)
			delete(bb.waiting, txID)
		}
	}
	bb.depMu.Unlock()
	bb.requeue(released)
}

// requeue puts released/expired items back on the broadcast channel without
// blocking the caller (a full channel would otherwise deadlock a worker).
func (bb *BackgroundBroadcaster) requeue(items []broadcastItem) {
	if len(items) == 0 {
		return
	}
	bb.wg.Add(1)
	go func() {
		defer bb.wg.Done()
		for _, it := range items {
			select {
			case bb.broadcastChannel <- it:
			case <-bb.ctx.Done():
				return
			}
		}
	}()
}

// parentWaitSweeper force-posts children whose parent-wait deadline has passed so
// a never-posted parent cannot hold them forever.
func (bb *BackgroundBroadcaster) parentWaitSweeper() {
	defer bb.wg.Done()
	ticker := time.NewTicker(parentWaitSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-bb.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			bb.depMu.Lock()
			var expired []broadcastItem
			for parent, children := range bb.waiting {
				kept := children[:0]
				for _, it := range children {
					if now.After(it.deadline) {
						expired = append(expired, it)
					} else {
						kept = append(kept, it)
					}
				}
				if len(kept) == 0 {
					delete(bb.waiting, parent)
				} else {
					bb.waiting[parent] = kept
				}
			}
			bb.depMu.Unlock()
			if len(expired) > 0 {
				bb.logger.WarnContext(bb.ctx, "parent-wait deadline reached, force-posting children", "count", len(expired))
			}
			bb.requeue(expired)
		}
	}
}

func (bb *BackgroundBroadcaster) broadcast(item *broadcastItem) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from panic during broadcast: %v", r)
		}
	}()

	results, err := bb.broadcastHandler.BackgroundBroadcast(bb.ctx, item.beef, item.txIDs)
	if err != nil {
		return fmt.Errorf("failed to broadcast beef: %w", err)
	}

	if bb.txBroadcastedChannel == nil || results == nil {
		return nil
	}

	for _, res := range results {
		msg := wdk.CurrentTxStatus{
			TxID:      res.TxID.String(),
			Status:    res.Status.ToStandardizedStatus(),
			Reference: res.Reference,
			Labels:    res.Labels,
		}

		if len(res.Errors) > 0 {
			broadcastError := &wdk.CurrentTxError{
				CompetingTxs: res.CompetingTxs,
				Errors:       map[string]error(res.Errors),
			}
			msg.Error = broadcastError
		}

		select {
		case bb.txBroadcastedChannel <- msg:
		case <-bb.ctx.Done():
			return fmt.Errorf("context done while sending tx status update: %w", bb.ctx.Err())
		default:
			bb.logger.WarnContext(bb.ctx, "TxBroadcasted channel in background broadcaster is full, dropping event")
		}
	}

	return nil
}
