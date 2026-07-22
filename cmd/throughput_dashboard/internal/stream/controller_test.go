package stream_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/stretchr/testify/require"
)

type fakeWallet struct {
	calls atomic.Uint64
	fail  bool

	mu       sync.Mutex
	args     []sdk.CreateActionArgs
	origins  []string
	seenHash map[string]struct{}
	delay    time.Duration
}

func (f *fakeWallet) CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) {
	f.calls.Add(1)

	f.mu.Lock()
	f.args = append(f.args, args)
	f.origins = append(f.origins, originator)
	if f.seenHash == nil {
		f.seenHash = make(map[string]struct{})
	}
	if len(args.Outputs) == 1 {
		// Record locking script bytes for uniqueness checks.
		f.seenHash[string(args.Outputs[0].LockingScript)] = struct{}{}
	}
	delay := f.delay
	f.mu.Unlock()

	if f.fail {
		return nil, errors.New("boom")
	}
	if delay <= 0 {
		delay = time.Millisecond
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(delay):
	}
	return &sdk.CreateActionResult{}, nil
}

func (f *fakeWallet) snapshot() (args []sdk.CreateActionArgs, origins []string, uniqueScripts int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	args = append([]sdk.CreateActionArgs(nil), f.args...)
	origins = append([]string(nil), f.origins...)
	return args, origins, len(f.seenHash)
}

func TestControllerStartStop(t *testing.T) {
	fake := &fakeWallet{}
	ctrl := stream.NewController(fake, stream.Options{TPS: 50, Workers: 4, Originator: "test"}, nil)

	ctx := context.Background()
	require.NoError(t, ctrl.Start(ctx, stream.Options{}))
	require.True(t, ctrl.Running())
	require.Error(t, ctrl.Start(ctx, stream.Options{}), "second start should fail")

	time.Sleep(200 * time.Millisecond)
	ctrl.Stop()
	require.False(t, ctrl.Running())

	stats := ctrl.Stats()
	require.Positive(t, stats.Attempted)
	require.Positive(t, stats.Succeeded)
	// Stop does not cancel in-flight createActions; all started calls finish successfully.
	require.Equal(t, stats.Attempted, stats.Succeeded)
	require.Zero(t, stats.Failed)
	require.Equal(t, stats.Attempted, fake.calls.Load())
	require.Equal(t, stats.Iteration, stats.Attempted)
}

func TestControllerStopDoesNotCancelInFlightCreateAction(t *testing.T) {
	// Slow createAction so Stop races with in-flight work.
	started := make(chan struct{})
	slow := &signalingWallet{delay: 300 * time.Millisecond, started: started}
	ctrl := stream.NewController(slow, stream.Options{TPS: 100, Workers: 1, Originator: "test"}, nil)

	require.NoError(t, ctrl.Start(context.Background(), stream.Options{}))
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for createAction to start")
	}
	// Stop while at least one createAction is mid-flight; none may be aborted.
	ctrl.Stop()
	require.False(t, ctrl.Running())

	stats := ctrl.Stats()
	require.Positive(t, stats.Attempted)
	require.Equal(t, stats.Attempted, stats.Succeeded)
	require.Zero(t, stats.Failed)
	require.Equal(t, stats.Attempted, slow.calls.Load())
}

func TestControllerCountsFailures(t *testing.T) {
	fake := &fakeWallet{fail: true}
	ctrl := stream.NewController(fake, stream.Options{TPS: 80, Workers: 4, Originator: "test"}, nil)

	require.NoError(t, ctrl.Start(context.Background(), stream.Options{}))
	time.Sleep(150 * time.Millisecond)
	ctrl.Stop()

	stats := ctrl.Stats()
	require.Positive(t, stats.Attempted)
	require.Equal(t, stats.Attempted, stats.Failed)
	require.Equal(t, uint64(0), stats.Succeeded)
}

func TestControllerStopWhenNotRunningIsNoop(t *testing.T) {
	ctrl := stream.NewController(&fakeWallet{}, stream.Options{TPS: 1, Workers: 1}, nil)
	ctrl.Stop()
	require.False(t, ctrl.Running())
}

func TestControllerCreateActionArgs(t *testing.T) {
	fake := &fakeWallet{}
	ctrl := stream.NewController(fake, stream.Options{TPS: 40, Workers: 2, Originator: "demo-origin"}, nil)

	require.NoError(t, ctrl.Start(context.Background(), stream.Options{}))
	time.Sleep(120 * time.Millisecond)
	ctrl.Stop()

	args, origins, uniqueScripts := fake.snapshot()
	require.NotEmpty(t, args)

	for i, a := range args {
		require.Lenf(t, a.Outputs, 1, "call %d: single OP_RETURN output", i)
		out := a.Outputs[0]
		require.EqualValuesf(t, 0, out.Satoshis, "call %d: satoshis must be 0", i)
		require.NotEmptyf(t, out.LockingScript, "call %d: locking script", i)

		asm := script.NewFromBytes(out.LockingScript).ToASM()
		require.Containsf(t, asm, "OP_RETURN", "call %d", i)

		require.NotNilf(t, a.Options, "call %d: options required", i)
		require.NotNilf(t, a.Options.AcceptDelayedBroadcast, "call %d", i)
		require.Truef(t, *a.Options.AcceptDelayedBroadcast, "call %d: AcceptDelayedBroadcast", i)

		require.Equalf(t, "demo-origin", origins[i], "call %d: originator", i)
	}

	// Each action should get a distinct locking script (unique hash/iteration).
	require.Equal(t, len(args), uniqueScripts)
	require.Equal(t, uint64(len(args)), ctrl.Stats().Iteration)
}

func TestControllerRestartAfterStop(t *testing.T) {
	fake := &fakeWallet{}
	ctrl := stream.NewController(fake, stream.Options{TPS: 60, Workers: 2, Originator: "test"}, nil)

	require.NoError(t, ctrl.Start(context.Background(), stream.Options{}))
	time.Sleep(80 * time.Millisecond)
	ctrl.Stop()
	require.False(t, ctrl.Running())
	first := ctrl.Stats().Attempted
	require.Positive(t, first)

	require.NoError(t, ctrl.Start(context.Background(), stream.Options{TPS: 40, Workers: 3}))
	require.True(t, ctrl.Running())
	time.Sleep(80 * time.Millisecond)
	ctrl.Stop()
	require.False(t, ctrl.Running())

	stats := ctrl.Stats()
	require.Greater(t, stats.Attempted, first)
	require.Equal(t, stats.Attempted, stats.Succeeded+stats.Failed)
	require.Equal(t, 40, stats.TPS)
	require.Equal(t, 3, stats.Workers)
}

func TestControllerStartOverridesOptions(t *testing.T) {
	ctrl := stream.NewController(&fakeWallet{}, stream.Options{TPS: 10, Workers: 8, Originator: "default"}, nil)
	require.NoError(t, ctrl.Start(context.Background(), stream.Options{TPS: 25, Workers: 5, Originator: "override"}))
	stats := ctrl.Stats()
	require.Equal(t, 25, stats.TPS)
	require.Equal(t, 5, stats.Workers)
	require.True(t, stats.Running)
	ctrl.Stop()
}

func TestControllerDefaults(t *testing.T) {
	ctrl := stream.NewController(&fakeWallet{}, stream.Options{}, nil)
	stats := ctrl.Stats()
	require.Equal(t, 10, stats.TPS)
	require.Equal(t, 8, stats.Workers)
	require.False(t, stats.Running)
}

func TestControllerRejectsExcessiveWorkers(t *testing.T) {
	ctrl := stream.NewController(&fakeWallet{}, stream.Options{TPS: 10, Workers: 8}, nil)
	err := ctrl.Start(context.Background(), stream.Options{Workers: stream.MaxWorkers + 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds max")
	require.False(t, ctrl.Running())
}

func TestControllerRejectsExcessiveTPS(t *testing.T) {
	ctrl := stream.NewController(&fakeWallet{}, stream.Options{TPS: 10, Workers: 8}, nil)
	err := ctrl.Start(context.Background(), stream.Options{TPS: stream.MaxTPS + 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds max")
	require.False(t, ctrl.Running())
}

func TestControllerSnapshotAndDelta(t *testing.T) {
	fake := &fakeWallet{}
	ctrl := stream.NewController(fake, stream.Options{TPS: 50, Workers: 2, Originator: "test"}, nil)

	require.NoError(t, ctrl.Start(context.Background(), stream.Options{}))
	time.Sleep(100 * time.Millisecond)
	ctrl.Stop()

	s, dA, dS, dF := ctrl.SnapshotAndDelta(0, 0, 0)
	require.Equal(t, s.Attempted, dA)
	require.Equal(t, s.Succeeded, dS)
	require.Equal(t, s.Failed, dF)

	s2, dA2, dS2, dF2 := ctrl.SnapshotAndDelta(s.Attempted, s.Succeeded, s.Failed)
	require.Equal(t, s.Attempted, s2.Attempted)
	require.Zero(t, dA2)
	require.Zero(t, dS2)
	require.Zero(t, dF2)
}

func TestControllerParentCancelStopsStream(t *testing.T) {
	fake := &fakeWallet{}
	ctrl := stream.NewController(fake, stream.Options{TPS: 50, Workers: 2, Originator: "test"}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ctrl.Start(ctx, stream.Options{}))
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Parent cancel stops production; in-flight createActions still finish.
	// Wait until run drains and marks itself not running.
	require.Eventually(t, func() bool { return !ctrl.Running() }, 2*time.Second, 10*time.Millisecond)

	stats := ctrl.Stats()
	require.Equal(t, stats.Attempted, stats.Succeeded)
	require.Zero(t, stats.Failed)
	// Stop after parent cancel is still a safe no-op / drain.
	ctrl.Stop()
	require.False(t, ctrl.Running())
}

// signalingWallet is like fakeWallet but signals when CreateAction starts.
type signalingWallet struct {
	calls   atomic.Uint64
	delay   time.Duration
	started chan struct{}
	once    sync.Once
}

func (f *signalingWallet) CreateAction(ctx context.Context, _ sdk.CreateActionArgs, _ string) (*sdk.CreateActionResult, error) {
	f.calls.Add(1)
	f.once.Do(func() {
		if f.started != nil {
			close(f.started)
		}
	})
	delay := f.delay
	if delay <= 0 {
		delay = time.Millisecond
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(delay):
	}
	return &sdk.CreateActionResult{}, nil
}
