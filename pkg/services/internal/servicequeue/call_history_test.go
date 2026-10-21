package servicequeue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestCallHistory(t *testing.T) {
	// given:
	history := servicequeue.NewCallHistory()
	ok := "ok"
	queue := servicequeue.NewQueue(logging.NewTestLogger(t), history, "Do",
		servicequeue.NewService("ok", func(context.Context) (*string, error) { return &ok, nil }),
		servicequeue.NewService("notFound", func(context.Context) (*string, error) { return nil, wdk.ErrNotFoundError }),
		servicequeue.NewService("empty", func(context.Context) (*string, error) { return nil, nil }),
		servicequeue.NewService("failing", func(context.Context) (*string, error) { return nil, errors.New("boom") }),
		servicequeue.NewService("panicking", func(context.Context) (*string, error) { panic("boom") }),
	)

	// when:
	for range 33 {
		_, err := queue.All(t.Context())
		require.NoError(t, err)
	}
	snapshot := history.Snapshot(true)

	// then:
	providers := snapshot["Do"].HistoryByProvider
	require.Len(t, providers, 5)

	okHistory := providers["ok"]
	assert.Len(t, okHistory.Calls, 32, "calls are capped")
	assert.True(t, okHistory.Calls[0].Success)
	assert.Equal(t, 33, okHistory.TotalCounts.Success)

	assert.Equal(t, "not_found", providers["notFound"].Calls[0].Result)
	assert.Equal(t, "empty", providers["empty"].Calls[0].Result)
	for _, name := range []string{"notFound", "empty"} {
		assert.Equal(t, 33, providers[name].TotalCounts.Failure, name)
		assert.Zero(t, providers[name].TotalCounts.Error, name)
	}

	for _, name := range []string{"failing", "panicking"} {
		counts := providers[name].TotalCounts
		assert.Equal(t, 33, counts.Failure, name)
		assert.Equal(t, 33, counts.Error, name)
		assert.Contains(t, providers[name].Calls[0].Error, "boom", name)
	}

	// when:
	_, err := queue.All(t.Context())
	require.NoError(t, err)
	afterReset := history.Snapshot(false)["Do"].HistoryByProvider["ok"]

	// then:
	require.Len(t, afterReset.ResetCounts, 2, "reset starts a new interval")
	assert.Equal(t, 1, afterReset.ResetCounts[0].Success)
	assert.Equal(t, 33, afterReset.ResetCounts[1].Success)
	assert.Equal(t, 34, afterReset.TotalCounts.Success)
}
