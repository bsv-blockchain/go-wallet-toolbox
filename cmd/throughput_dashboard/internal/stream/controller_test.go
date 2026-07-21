package stream_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/stretchr/testify/require"
)

type fakeWallet struct {
	calls atomic.Uint64
	fail  bool
}

func (f *fakeWallet) CreateAction(ctx context.Context, _ sdk.CreateActionArgs, _ string) (*sdk.CreateActionResult, error) {
	f.calls.Add(1)
	if f.fail {
		return nil, errors.New("boom")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Millisecond):
	}
	return &sdk.CreateActionResult{}, nil
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
	// Stop cancels in-flight CreateAction; those count as failed, not crashes.
	require.Equal(t, stats.Attempted, stats.Succeeded+stats.Failed)
	require.Equal(t, stats.Attempted, fake.calls.Load())
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
