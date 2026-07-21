package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"
)

// fakeActionCreator succeeds after a short sleep; never fails.
type fakeActionCreator struct {
	calls atomic.Uint64
}

func (f *fakeActionCreator) CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) {
	f.calls.Add(1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Millisecond):
	}
	return &sdk.CreateActionResult{}, nil
}

func TestRunLoadRateAndSuccess(t *testing.T) {
	fake := &fakeActionCreator{}
	cfg := Config{
		TPS:             100,
		Workers:         10,
		DurationSeconds: 1,
		Originator:      "test.local",
	}
	lockingScript := []byte{0x6a, 0x01, 0x00} // minimal OP_RETURN-ish script for the fake

	ctx := context.Background()
	stats := RunLoad(ctx, fake, cfg, lockingScript)

	require.GreaterOrEqual(t, stats.Attempted, uint64(80), "expected ~100 attempts in 1s at 100 TPS")
	require.LessOrEqual(t, stats.Attempted, uint64(150), "rate limiter should not far exceed TPS")
	require.Equal(t, stats.Attempted, stats.Succeeded, "fake never fails")
	require.Equal(t, uint64(0), stats.Failed)
	require.Equal(t, stats.Attempted, fake.calls.Load())
}
