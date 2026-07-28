package service_test

import (
	"context"
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
	o.order = append(o.order, txIDs...)
	o.mu.Unlock()
	return nil, nil
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
	defer bb.Stop()

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
