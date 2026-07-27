package syncwallet_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/syncwallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// stubWallet blocks CreateAction on release (when set) and tracks the maximum
// number of concurrently running RPCs so tests can prove the in-flight bound.
type stubWallet struct {
	release chan struct{} // CreateAction waits on this when non-nil

	inFlight    atomic.Int64
	maxInFlight atomic.Int64
	listCalls   atomic.Int64
	closed      atomic.Bool
}

func (s *stubWallet) enter() func() {
	n := s.inFlight.Add(1)
	for {
		cur := s.maxInFlight.Load()
		if n <= cur || s.maxInFlight.CompareAndSwap(cur, n) {
			break
		}
	}
	return func() { s.inFlight.Add(-1) }
}

func (s *stubWallet) Balance(context.Context) (uint64, error) {
	defer s.enter()()
	return 0, nil
}

func (s *stubWallet) ListOutputs(context.Context, sdk.ListOutputsArgs, string) (*sdk.ListOutputsResult, error) {
	defer s.enter()()
	s.listCalls.Add(1)
	return &sdk.ListOutputsResult{}, nil
}

func (s *stubWallet) ListActions(context.Context, sdk.ListActionsArgs, string) (*sdk.ListActionsResult, error) {
	defer s.enter()()
	return &sdk.ListActionsResult{}, nil
}

func (s *stubWallet) CreateAction(context.Context, sdk.CreateActionArgs, string) (*sdk.CreateActionResult, error) {
	defer s.enter()()
	if s.release != nil {
		<-s.release
	}
	return &sdk.CreateActionResult{}, nil
}

func (s *stubWallet) FanOutFuel(context.Context, wdk.ShapedChange, string) (*sdk.CreateActionResult, error) {
	defer s.enter()()
	return &sdk.CreateActionResult{}, nil
}

func (s *stubWallet) InternalizeAction(context.Context, sdk.InternalizeActionArgs, string) (*sdk.InternalizeActionResult, error) {
	defer s.enter()()
	return &sdk.InternalizeActionResult{}, nil
}

func (s *stubWallet) Close() { s.closed.Store(true) }

func TestBounded_WaiterRespectsContextCancellation(t *testing.T) {
	stub := &stubWallet{release: make(chan struct{})}
	bounded := syncwallet.New(stub, 1)

	holderDone := make(chan error, 1)
	go func() {
		_, err := bounded.CreateAction(context.Background(), sdk.CreateActionArgs{}, "o")
		holderDone <- err
	}()
	// Wait until the holder owns the slot (it is inside CreateAction, blocked).
	require.Eventually(t, func() bool { return stub.inFlight.Load() == 1 }, time.Second, time.Millisecond)

	// A queued caller with a deadline must give up when the deadline fires,
	// not wait for the slot: this is what keeps sampler ticks bounded.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := bounded.ListOutputs(ctx, sdk.ListOutputsArgs{}, "o")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 500*time.Millisecond)
	require.Zero(t, stub.listCalls.Load(), "canceled waiter must not reach the wallet")

	close(stub.release)
	require.NoError(t, <-holderDone)
}

func TestBounded_PreCanceledContextFailsFast(t *testing.T) {
	bounded := syncwallet.New(&stubWallet{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := bounded.ListActions(ctx, sdk.ListActionsArgs{}, "o")
	require.ErrorIs(t, err, context.Canceled)
}

func TestBounded_EnforcesMaxInFlight(t *testing.T) {
	for _, capN := range []int{1, 4} {
		stub := &stubWallet{}
		bounded := syncwallet.New(stub, capN)

		var wg sync.WaitGroup
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				ctx := context.Background()
				switch n % 4 {
				case 0:
					_, _ = bounded.CreateAction(ctx, sdk.CreateActionArgs{}, "o")
				case 1:
					_, _ = bounded.ListOutputs(ctx, sdk.ListOutputsArgs{}, "o")
				case 2:
					_, _ = bounded.FanOutFuel(ctx, wdk.ShapedChange{}, "o")
				default:
					_, _ = bounded.Balance(ctx)
				}
			}(i)
		}
		wg.Wait()
		require.LessOrEqual(t, stub.maxInFlight.Load(), int64(capN),
			"no more than %d RPCs may run concurrently", capN)
	}
}

func TestBounded_DefaultCapacityWhenZero(t *testing.T) {
	stub := &stubWallet{}
	bounded := syncwallet.New(stub, 0)
	_, err := bounded.Balance(context.Background())
	require.NoError(t, err)
}

func TestBounded_CloseWaitsForCurrentRPCAndClosesOnce(t *testing.T) {
	stub := &stubWallet{release: make(chan struct{})}
	bounded := syncwallet.New(stub, 1)

	holderDone := make(chan struct{})
	go func() {
		_, _ = bounded.CreateAction(context.Background(), sdk.CreateActionArgs{}, "o")
		close(holderDone)
	}()
	require.Eventually(t, func() bool { return stub.inFlight.Load() == 1 }, time.Second, time.Millisecond)

	closeDone := make(chan struct{})
	go func() {
		bounded.Close()
		bounded.Close() // second call must be a no-op, not a deadlock
		close(closeDone)
	}()

	select {
	case <-closeDone:
		t.Fatal("Close returned while an RPC was still in flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(stub.release)
	<-holderDone
	select {
	case <-closeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not complete after the RPC finished")
	}
	require.True(t, stub.closed.Load())
}
