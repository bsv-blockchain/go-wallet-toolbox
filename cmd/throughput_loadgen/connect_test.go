package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryWithBackoffSucceedsAfterFailures(t *testing.T) {
	var calls atomic.Int32
	err := retryWithBackoff(
		context.Background(),
		2*time.Second,
		10*time.Millisecond,
		50*time.Millisecond,
		func() error {
			n := calls.Add(1)
			if n < 3 {
				return errors.New("not ready")
			}
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load())
}

func TestRetryWithBackoffExhaustsWindow(t *testing.T) {
	err := retryWithBackoff(
		context.Background(),
		80*time.Millisecond,
		20*time.Millisecond,
		20*time.Millisecond,
		func() error { return errors.New("down") },
		nil,
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "after")
	require.ErrorContains(t, err, "down")
}

func TestRetryWithBackoffRespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	err := retryWithBackoff(
		ctx,
		5*time.Second,
		100*time.Millisecond,
		100*time.Millisecond,
		func() error { return errors.New("down") },
		nil,
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "aborted")
}
