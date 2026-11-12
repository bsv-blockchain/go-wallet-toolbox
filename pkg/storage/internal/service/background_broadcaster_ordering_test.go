package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/service"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// orderRecordingBroadcaster records the order in which txIDs are posted, with an
// optional per-call delay to widen the race window.
type orderRecordingBroadcaster struct {
	mu    sync.Mutex
	order []string
	delay time.Duration
	// statuses overrides the stored status of a txid; every other txid reads as
	// unsent - a transaction of this wallet still waiting to be sent.
	statuses map[string]wdk.ProvenTxReqStatus
	// failing makes the post of these txids fail.
	failing map[string]bool
}

func (o *orderRecordingBroadcaster) BackgroundBroadcast(ctx context.Context, _ *transaction.Beef, txIDs []string) ([]wdk.ReviewActionResult, error) {
	if o.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(o.delay):
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, txID := range txIDs {
		if o.failing[txID] {
			return nil, errors.New("post failed")
		}
	}
	o.order = append(o.order, txIDs...)
	return nil, nil
}

func (o *orderRecordingBroadcaster) KnownTxStatuses(_ context.Context, txIDs []string) (map[string]wdk.ProvenTxReqStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	statuses := make(map[string]wdk.ProvenTxReqStatus, len(txIDs))
	for _, txID := range txIDs {
		status, ok := o.statuses[txID]
		if !ok {
			status = wdk.ProvenTxStatusUnsent
		}
		statuses[txID] = status
	}
	return statuses, nil
}

func (o *orderRecordingBroadcaster) setStatus(txID string, status wdk.ProvenTxReqStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.statuses == nil {
		o.statuses = make(map[string]wdk.ProvenTxReqStatus)
	}
	o.statuses[txID] = status
}

func waitForLog(t testing.TB, logs interface{ String() string }, fragment string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !strings.Contains(logs.String(), fragment) {
		select {
		case <-deadline:
			t.Fatalf("expected log containing %q, got:\n%s", fragment, logs.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (o *orderRecordingBroadcaster) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.order...)
}

func (o *orderRecordingBroadcaster) waitFor(t testing.TB, n int) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		o.mu.Lock()
		got := len(o.order)
		o.mu.Unlock()
		if got >= n {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected %d posts, got %d", n, got)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// A child transaction spending an unconfirmed parent must be broadcast to Arcade
// AFTER its parent, even when it was enqueued first — so Arcade's receive-order
// contract forwards the parent to Teranode first.
func TestBackgroundBroadcaster_PostsUnconfirmedParentBeforeChild(t *testing.T) {
	t.Parallel()

	rec := &orderRecordingBroadcaster{delay: 10 * time.Millisecond}
	logger, _ := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil, service.Sizing{Workers: 1})
	bb.Start()
	defer bb.Stop(context.Background())

	parent := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(50)
	parentID := parent.ID().String()
	childID := child.ID().String()

	childBeef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)
	parentBeef, err := transaction.NewBeefFromTransaction(parent.TX())
	require.NoError(t, err)

	// Worst case: enqueue the child BEFORE the parent.
	require.True(t, bb.Add(childBeef, []string{childID}))
	require.True(t, bb.Add(parentBeef, []string{parentID}))

	rec.waitFor(t, 2)
	require.Equal(t, []string{parentID, childID}, rec.snapshot(),
		"parent must be posted to Arcade before its child")
}

// A batch that carries both an unconfirmed parent and the child spending it goes
// upstream in one request, parent first — it must not park waiting for a parent it
// is itself responsible for posting.
func TestBackgroundBroadcaster_PostsParentAndChildOfSameBatchWithoutWaiting(t *testing.T) {
	t.Parallel()

	rec := &orderRecordingBroadcaster{}
	logger, _ := loggerForTestBroadcaster()
	// A long parent wait would dominate the test if the batch parked, so reaching
	// the assertion at all proves the batch was posted without waiting.
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil,
		service.Sizing{Workers: 1, MaxParentWait: time.Minute})
	bb.Start()
	defer bb.Stop(context.Background())

	parent := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(50)
	parentID := parent.ID().String()
	childID := child.ID().String()

	// The child's beef carries its unconfirmed parent as a raw transaction.
	beef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)

	// Worst case: the child is named before its parent in the same batch.
	require.True(t, bb.Add(beef, []string{childID, parentID}))

	rec.waitFor(t, 2)
	require.Equal(t, []string{parentID, childID}, rec.snapshot(),
		"a same-batch parent must be posted first, not waited for")
}

// The parent wait must measure time spent waiting for a parent, not time spent
// queued: a child that sat in the channel behind other work for longer than
// MaxParentWait must still be posted after its parent.
func TestBackgroundBroadcaster_ParentWaitDoesNotExpireWhileQueued(t *testing.T) {
	t.Parallel()

	const (
		postDuration  = 100 * time.Millisecond
		maxParentWait = 30 * time.Millisecond
	)

	rec := &orderRecordingBroadcaster{delay: postDuration}
	logger, _ := loggerForTestBroadcaster()
	// The sweeper is parked out of the way: this test is about the wait not being
	// consumed while queued, not about what happens once it runs out.
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil, service.Sizing{
		Workers:       1,
		MaxParentWait: maxParentWait,
		SweepInterval: time.Minute,
	})
	bb.Start()
	defer bb.Stop(context.Background())

	// An unrelated transaction occupies the single worker while parent and child
	// queue up behind it.
	filler := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	fillerBeef, err := transaction.NewBeefFromTransaction(filler.TX())
	require.NoError(t, err)

	parent := testvectors.GivenTX().WithInput(200).WithP2PKHOutput(199)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(150)
	childBeef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)
	parentBeef, err := transaction.NewBeefFromTransaction(parent.TX())
	require.NoError(t, err)

	require.True(t, bb.Add(fillerBeef, []string{filler.ID().String()}))
	require.True(t, bb.Add(childBeef, []string{child.ID().String()}))
	require.True(t, bb.Add(parentBeef, []string{parent.ID().String()}))

	// Queue for longer than MaxParentWait before the worker gets to the child.
	time.Sleep(2 * maxParentWait)

	rec.waitFor(t, 3)
	require.Equal(t,
		[]string{filler.ID().String(), parent.ID().String(), child.ID().String()},
		rec.snapshot(),
		"a queued child must still wait for its parent")
}

// A child whose unconfirmed parent was never handed to this pool but already
// went out by another path — the non-delayed path, the cron fallback, an
// internalize, or another wallet — must be posted without waiting for it.
func TestBackgroundBroadcaster_DoesNotWaitForParentAlreadyOut(t *testing.T) {
	t.Parallel()

	parent := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(50)
	childID := child.ID().String()

	rec := &orderRecordingBroadcaster{}
	rec.setStatus(parent.ID().String(), wdk.ProvenTxStatusUnmined)
	logger, _ := loggerForTestBroadcaster()
	// Waits far beyond the test's patience: reaching the assertion proves the
	// child was not held at all.
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil, service.Sizing{
		Workers:           1,
		MaxParentWait:     time.Minute,
		UnknownParentWait: time.Minute,
	})
	bb.Start()
	defer bb.Stop(context.Background())

	// The beef carries the unconfirmed parent, but only the child is broadcast here.
	childBeef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)

	require.True(t, bb.Add(childBeef, []string{childID}))

	rec.waitFor(t, 1)
	require.Equal(t, []string{childID}, rec.snapshot())
}

// A parent of this wallet that is still waiting to be sent but was never handed
// to this pool has overflowed to the send_waiting cron. Its child must not be
// posted ahead of it: once the grace runs out, the child is left to the cron too.
func TestBackgroundBroadcaster_DefersChildOfParentLeftToCron(t *testing.T) {
	t.Parallel()

	rec := &orderRecordingBroadcaster{}
	logger, logs := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil, service.Sizing{
		Workers:           1,
		MaxParentWait:     time.Minute,
		UnknownParentWait: 20 * time.Millisecond,
		SweepInterval:     10 * time.Millisecond,
	})
	bb.Start()
	defer bb.Stop(context.Background())

	parent := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(50)
	childBeef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)

	require.True(t, bb.Add(childBeef, []string{child.ID().String()}))

	waitForLog(t, logs, "parent wait ran out")
	require.Empty(t, rec.snapshot(), "the child must not be posted ahead of its parent")
}

// A parent that reaches the network by another path while its child is held
// releases the child when the wait runs out, instead of sending it to the cron.
func TestBackgroundBroadcaster_ReleasesChildWhenParentGoesOutWhileHeld(t *testing.T) {
	t.Parallel()

	rec := &orderRecordingBroadcaster{}
	logger, _ := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil, service.Sizing{
		Workers:           1,
		MaxParentWait:     time.Minute,
		UnknownParentWait: 50 * time.Millisecond,
		SweepInterval:     10 * time.Millisecond,
	})
	bb.Start()
	defer bb.Stop(context.Background())

	parent := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(50)
	childBeef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)

	require.True(t, bb.Add(childBeef, []string{child.ID().String()}))
	// The parent, sent by the non-delayed path, is accepted while the child waits.
	rec.setStatus(parent.ID().String(), wdk.ProvenTxStatusUnmined)

	rec.waitFor(t, 1)
	require.Equal(t, []string{child.ID().String()}, rec.snapshot())
}

// When the parent's own post fails, the parent stays waiting in the database for
// the send_waiting cron, and its children must follow it there rather than be
// posted ahead of it.
func TestBackgroundBroadcaster_DefersChildrenOfParentWhosePostFailed(t *testing.T) {
	t.Parallel()

	parent := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(50)
	parentID := parent.ID().String()

	rec := &orderRecordingBroadcaster{failing: map[string]bool{parentID: true}}
	logger, logs := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil, service.Sizing{
		Workers:           1,
		MaxParentWait:     time.Minute,
		UnknownParentWait: time.Minute,
	})
	bb.Start()
	defer bb.Stop(context.Background())

	childBeef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)
	parentBeef, err := transaction.NewBeefFromTransaction(parent.TX())
	require.NoError(t, err)

	require.True(t, bb.Add(childBeef, []string{child.ID().String()}))
	require.True(t, bb.Add(parentBeef, []string{parentID}))

	waitForLog(t, logs, "parent broadcast failed")
	require.Empty(t, rec.snapshot(), "the child must not be posted ahead of its parent")
}

// Remembered txids are pruned after their retention window, so they cannot grow
// with lifetime volume. A child of a forgotten parent is then cleared by the
// parent's stored status and posted without waiting.
func TestBackgroundBroadcaster_PostsChildOfForgottenParent(t *testing.T) {
	t.Parallel()

	parent := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(99)
	child := testvectors.GivenTX().WithInputFromUTXO(parent.TX(), 0).WithP2PKHOutput(50)

	rec := &orderRecordingBroadcaster{}
	logger, _ := loggerForTestBroadcaster()
	bb := service.NewBackgroundBroadcaster(t.Context(), logger, rec, nil, service.Sizing{
		Workers:           1,
		MaxParentWait:     time.Minute,
		UnknownParentWait: time.Minute,
		PostedRetention:   time.Millisecond,
		SweepInterval:     10 * time.Millisecond,
	})
	bb.Start()
	defer bb.Stop(context.Background())

	parentBeef, err := transaction.NewBeefFromTransaction(parent.TX())
	require.NoError(t, err)
	childBeef, err := transaction.NewBeefFromTransaction(child.TX())
	require.NoError(t, err)

	require.True(t, bb.Add(parentBeef, []string{parent.ID().String()}))
	rec.waitFor(t, 1)
	rec.setStatus(parent.ID().String(), wdk.ProvenTxStatusUnmined)

	// Let the sweeper forget the parent.
	time.Sleep(100 * time.Millisecond)

	require.True(t, bb.Add(childBeef, []string{child.ID().String()}))
	rec.waitFor(t, 2)
	require.Equal(t,
		[]string{parent.ID().String(), child.ID().String()},
		rec.snapshot(),
		"the child of a forgotten parent must still be posted")
}
