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

	// defaultPostedRetention is how long a successfully posted txid is remembered
	// so that a later child spending it need not wait. It only has to cover the
	// gap between a parent's POST and the arrival of a child that spends it —
	// normally milliseconds — so a generous window still bounds the posted set to
	// the transactions of that window rather than the process lifetime. Forgetting
	// a parent too early costs a child at most MaxParentWait of latency, never
	// correctness.
	defaultPostedRetention = 2 * time.Hour

	// parentWaitSweepInterval is how often parked children are re-checked for an
	// expired parent-wait deadline (and the posted set pruned).
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
	// PostedRetention bounds how long posted txids are remembered for the benefit
	// of children that arrive later. Zero selects the default.
	PostedRetention time.Duration
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

func (s Sizing) postedRetention() time.Duration {
	if s.PostedRetention <= 0 {
		return defaultPostedRetention
	}
	return s.PostedRetention
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
	depMu sync.Mutex
	// posted maps a txid whose Arcade POST returned successfully to when that
	// happened; postedOrder holds the same txids in insertion (i.e. ascending
	// time) order so the sweeper can prune expired entries from its front instead
	// of scanning the whole map.
	posted      map[string]time.Time
	postedOrder []postedTxID
	waiting     map[string][]broadcastItem // children parked under an unposted parent txid

	// requeueQueue holds items whose parent has been posted (or whose wait
	// expired) until the dispatcher can put them back on broadcastChannel.
	requeueMu    sync.Mutex
	requeueQueue []broadcastItem
	requeueWake  chan struct{}

	stopOnce sync.Once
}

// postedTxID is a posted txid together with the time it was posted, kept for
// retention pruning.
type postedTxID struct {
	txID string
	at   time.Time
}

type broadcastItem struct {
	beef  *transaction.Beef
	txIDs []string
	// deadline is when a parked item may be posted regardless of unposted
	// parents. It stays zero until the item is first parked: the parent wait must
	// measure time spent waiting for a parent, not time spent queued behind other
	// work. Starting it at Add() time would let a backlog (posts slower than
	// creates — the high-TPS regime this gating exists for) expire the wait before
	// a worker ever inspects the item, force-posting the child out of order.
	deadline time.Time
	// force skips parent gating entirely. Only the sweeper sets it, on items whose
	// parent-wait deadline has passed.
	force bool
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
		posted:               make(map[string]time.Time),
		waiting:              make(map[string][]broadcastItem),
		requeueWake:          make(chan struct{}, 1),
	}
}

func (bb *BackgroundBroadcaster) Start() {
	for i := 0; i < bb.sizing.workers(); i++ {
		bb.wg.Add(1)
		go bb.worker()
	}
	bb.wg.Add(1)
	go bb.parentWaitSweeper()
	bb.wg.Add(1)
	go bb.requeueDispatcher()
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
	item := broadcastItem{beef: beef, txIDs: parentsFirst(beef, txIDs)}
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

// process posts the item once its unconfirmed parents have been posted, parking
// it otherwise. Only the sweeper's force flag bypasses the gate, so an item is
// never posted out of order merely because it queued for a long time.
func (bb *BackgroundBroadcaster) process(item broadcastItem) {
	if !item.force && bb.park(&item) {
		return
	}

	if err := bb.broadcast(&item); err != nil {
		bb.logger.ErrorContext(bb.ctx, "Failed to broadcast transaction", "error", err, "txIDs", item.txIDs)
		return
	}
	bb.logger.InfoContext(bb.ctx, "Successfully broadcasted transaction", "txIDs", item.txIDs)
	bb.markPostedAndRelease(item.txIDs)
}

// park holds the item under the first unconfirmed parent it spends that has not
// been posted yet, reporting whether it was parked. The parent-wait deadline
// starts on the first park, so an item re-parked under a second parent still
// waits at most MaxParentWait in total.
func (bb *BackgroundBroadcaster) park(item *broadcastItem) bool {
	bb.depMu.Lock()
	parent := bb.firstUnpostedParentLocked(*item)
	if parent == "" {
		bb.depMu.Unlock()
		return false
	}
	if item.deadline.IsZero() {
		item.deadline = time.Now().Add(bb.sizing.maxParentWait())
	}
	bb.waiting[parent] = append(bb.waiting[parent], *item)
	bb.depMu.Unlock()

	bb.logger.DebugContext(bb.ctx, "holding child until parent is posted", "parent", parent, "txIDs", item.txIDs)
	return true
}

// firstUnpostedParentLocked returns the txid of the first unconfirmed parent that
// this item spends and that has not yet been posted, or "" if it is ready.
//
// Note that posted is local to this broadcaster's delayed pool, not global
// network state: a parent posted by the non-delayed path, by the send_waiting
// cron fallback (which is what happens when this pool's channel is full), or by a
// previous process lifetime is absent from it. A child of such a parent waits out
// MaxParentWait and is then force-posted, which costs latency, never correctness.
//
// Caller must hold depMu.
func (bb *BackgroundBroadcaster) firstUnpostedParentLocked(item broadcastItem) string {
	if item.beef == nil {
		return ""
	}
	subjects := make(map[string]struct{}, len(item.txIDs))
	for _, txID := range item.txIDs {
		subjects[txID] = struct{}{}
	}
	for _, txID := range item.txIDs {
		if parent := bb.firstUnpostedParentOfLocked(item.beef, txID, subjects); parent != "" {
			return parent
		}
	}
	return ""
}

// firstUnpostedParentOfLocked returns the first gate-worthy parent of a single
// subject transaction. A parent is gate-worthy only when it is present in the
// beef as a raw, un-mined transaction (transaction.RawTx): a mined parent
// (RawTxAndBumpIndex) is already in the UTXO set, and a txid-only / absent parent
// is not something this broadcaster is responsible for. A parent that is itself a
// subject of the same item is not gate-worthy either — parentsFirst ordered it
// ahead of its children, so it goes upstream first within the same request.
// Caller must hold depMu.
func (bb *BackgroundBroadcaster) firstUnpostedParentOfLocked(beef *transaction.Beef, txID string, subjects map[string]struct{}) string {
	tx := beefTx(beef, txID)
	if tx == nil {
		return ""
	}
	for _, in := range tx.Inputs {
		if in.SourceTXID == nil {
			continue
		}
		parentID := in.SourceTXID.String()
		if _, ok := subjects[parentID]; ok {
			continue // posted by this very item, ahead of this child
		}
		if _, ok := bb.posted[parentID]; ok {
			continue
		}
		parentBeefTx := beef.Transactions[*in.SourceTXID]
		if parentBeefTx == nil || parentBeefTx.DataFormat != transaction.RawTx {
			continue // absent, txid-only, or mined (bump) parent → no gating
		}
		return parentID
	}
	return ""
}

// parentsFirst returns txIDs reordered so that a transaction spending another
// transaction of the same batch comes after it, preserving the input order
// otherwise. PostFromBEEF posts a batch in slice order and Arcade forwards
// upstream in receive order, so an unordered batch could post a child before the
// parent it shares a request with. The input slice is not modified.
func parentsFirst(beef *transaction.Beef, txIDs []string) []string {
	if beef == nil || len(txIDs) < 2 {
		return append([]string(nil), txIDs...)
	}

	batch := make(map[string]struct{}, len(txIDs))
	for _, txID := range txIDs {
		batch[txID] = struct{}{}
	}

	ordered := make([]string, 0, len(txIDs))
	visited := make(map[string]struct{}, len(txIDs))

	var visit func(txID string)
	visit = func(txID string) {
		if _, seen := visited[txID]; seen {
			return
		}
		visited[txID] = struct{}{} // marked before recursing, so a cycle cannot loop forever
		for _, parentID := range batchParents(beef, txID, batch) {
			visit(parentID)
		}
		ordered = append(ordered, txID)
	}
	for _, txID := range txIDs {
		visit(txID)
	}
	return ordered
}

// batchParents returns the txids spent by txID that are also subjects of the same
// batch.
func batchParents(beef *transaction.Beef, txID string, batch map[string]struct{}) []string {
	tx := beefTx(beef, txID)
	if tx == nil {
		return nil
	}
	var parents []string
	for _, in := range tx.Inputs {
		if in.SourceTXID == nil {
			continue
		}
		parentID := in.SourceTXID.String()
		if _, ok := batch[parentID]; ok {
			parents = append(parents, parentID)
		}
	}
	return parents
}

// beefTx looks a subject transaction up in a beef by its hex txid.
func beefTx(beef *transaction.Beef, txID string) *transaction.Transaction {
	hash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		return nil
	}
	return beef.FindTransactionByHash(hash)
}

// markPostedAndRelease records the posted txids and re-queues any children that
// were waiting on them.
func (bb *BackgroundBroadcaster) markPostedAndRelease(txIDs []string) {
	now := time.Now()

	bb.depMu.Lock()
	var released []broadcastItem
	for _, txID := range txIDs {
		bb.posted[txID] = now
		bb.postedOrder = append(bb.postedOrder, postedTxID{txID: txID, at: now})
		if children, ok := bb.waiting[txID]; ok {
			released = append(released, children...)
			delete(bb.waiting, txID)
		}
	}
	bb.depMu.Unlock()

	bb.requeue(released)
}

// requeue hands released/expired items to the requeue dispatcher. It neither
// blocks (a full broadcast channel would otherwise deadlock the very workers
// meant to drain it) nor touches bb.wg: growing the WaitGroup from a worker races
// with Stop's Wait ("WaitGroup misuse: Add called concurrently with Wait"), so
// the only requeue goroutine is the long-lived one Start creates.
func (bb *BackgroundBroadcaster) requeue(items []broadcastItem) {
	if len(items) == 0 {
		return
	}

	bb.requeueMu.Lock()
	bb.requeueQueue = append(bb.requeueQueue, items...)
	bb.requeueMu.Unlock()

	select {
	case bb.requeueWake <- struct{}{}:
	default: // a wake-up is already pending and drains everything queued by then
	}
}

// requeueDispatcher puts requeued items back on the broadcast channel.
func (bb *BackgroundBroadcaster) requeueDispatcher() {
	defer bb.wg.Done()
	for {
		select {
		case <-bb.ctx.Done():
			return
		case <-bb.requeueWake:
			if !bb.drainRequeue() {
				return
			}
		}
	}
}

// drainRequeue moves everything queued so far onto the broadcast channel,
// reporting false if the broadcaster is shutting down.
func (bb *BackgroundBroadcaster) drainRequeue() bool {
	for {
		bb.requeueMu.Lock()
		batch := bb.requeueQueue
		bb.requeueQueue = nil
		bb.requeueMu.Unlock()

		if len(batch) == 0 {
			return true
		}
		for _, item := range batch {
			select {
			case bb.broadcastChannel <- item:
			case <-bb.ctx.Done():
				return false
			}
		}
	}
}

// parentWaitSweeper force-posts children whose parent-wait deadline has passed so
// a never-posted parent cannot hold them forever, and prunes the posted set so it
// cannot grow with lifetime transaction volume.
func (bb *BackgroundBroadcaster) parentWaitSweeper() {
	defer bb.wg.Done()
	ticker := time.NewTicker(parentWaitSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-bb.ctx.Done():
			return
		case <-ticker.C:
			bb.sweep(time.Now())
		}
	}
}

func (bb *BackgroundBroadcaster) sweep(now time.Time) {
	bb.depMu.Lock()
	expired := bb.takeExpiredLocked(now)
	bb.prunePostedLocked(now)
	bb.depMu.Unlock()

	if len(expired) == 0 {
		return
	}
	bb.logger.WarnContext(bb.ctx, "parent-wait deadline reached, force-posting children", "count", len(expired))
	bb.requeue(expired)
}

// takeExpiredLocked removes and returns the parked items whose parent-wait
// deadline has passed, flagged to skip gating on their next attempt. Caller must
// hold depMu.
func (bb *BackgroundBroadcaster) takeExpiredLocked(now time.Time) []broadcastItem {
	var expired []broadcastItem
	for parent, children := range bb.waiting {
		kept := children[:0]
		for _, item := range children {
			if now.After(item.deadline) {
				item.force = true
				expired = append(expired, item)
				continue
			}
			kept = append(kept, item)
		}
		if len(kept) == 0 {
			delete(bb.waiting, parent)
		} else {
			bb.waiting[parent] = kept
		}
	}
	return expired
}

// prunePostedLocked drops posted txids older than the retention window.
// postedOrder is in ascending time order, so pruning stops at the first entry
// still inside the window and costs only what it removes. Caller must hold depMu.
func (bb *BackgroundBroadcaster) prunePostedLocked(now time.Time) {
	cutoff := now.Add(-bb.sizing.postedRetention())

	pruned := 0
	for ; pruned < len(bb.postedOrder); pruned++ {
		entry := bb.postedOrder[pruned]
		if entry.at.After(cutoff) {
			break
		}
		// A re-posted txid has a newer map timestamp and a later order entry; drop
		// it only when this entry is the newest one for that txid.
		if at, ok := bb.posted[entry.txID]; ok && !at.After(entry.at) {
			delete(bb.posted, entry.txID)
		}
	}
	if pruned > 0 {
		bb.postedOrder = append(bb.postedOrder[:0], bb.postedOrder[pruned:]...)
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
