package servicequeue_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
)

func upper(_ context.Context, items []string) ([]*string, error) {
	out := make([]*string, len(items))
	for i, it := range items {
		u := strings.ToUpper(it)
		out[i] = &u
	}
	return out, nil
}

func TestBatchQueue_All(t *testing.T) {
	items := []string{"a", "b", "c"}

	tests := map[string]struct {
		service   func(context.Context, []string) ([]*string, error)
		wantValue []string
		wantErr   string
	}{
		"one result per item, in item order": {
			service:   upper,
			wantValue: []string{"A", "B", "C"},
		},
		"a failed call is an error for every item": {
			service: func(context.Context, []string) ([]*string, error) { return nil, errors.New("down") },
			wantErr: "down",
		},
		"a panic is an error for every item": {
			service: func(context.Context, []string) ([]*string, error) { panic("boom") },
			wantErr: "boom",
		},
		"a missing answer is an empty result for every item": {
			service: func(context.Context, []string) ([]*string, error) { return nil, nil },
			wantErr: servicequeue.ErrEmptyResult.Error(),
		},
		"too few results is an error for every item": {
			service: func(context.Context, []string) ([]*string, error) { s := "x"; return []*string{&s}, nil },
			wantErr: "returned 1 results for 3 items",
		},
		"a nil result is an empty result for its item only": {
			service: func(ctx context.Context, items []string) ([]*string, error) {
				out, _ := upper(ctx, items)
				out[1] = nil
				return out, nil
			},
			wantValue: []string{"A", "", "C"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			history := servicequeue.NewCallHistory()
			queue := servicequeue.NewBatchQueue(logging.NewTestLogger(t), history, "Do",
				servicequeue.NewBatchService("first", upper),
				servicequeue.NewBatchService("second", test.service),
			)

			// when:
			perItem, err := queue.All(t.Context(), items)

			// then:
			require.NoError(t, err)
			require.Len(t, perItem, len(items))
			for i, results := range perItem {
				require.Len(t, results, 2)
				assert.Equal(t, "first", results[0].Name())
				assert.Equal(t, strings.ToUpper(items[i]), *results[0].MustGetValue())

				second := results[1]
				assert.Equal(t, "second", second.Name())
				if test.wantValue != nil && test.wantValue[i] != "" {
					assert.Equal(t, test.wantValue[i], *second.MustGetValue())
					continue
				}
				require.True(t, second.IsError())
				if test.wantErr != "" {
					require.ErrorContains(t, second.GetError(), test.wantErr)
				} else {
					require.ErrorIs(t, second.GetError(), servicequeue.ErrEmptyResult)
				}
			}

			// and: each service call is recorded once, not once per item
			snapshot := history.Snapshot(false)["Do"].HistoryByProvider
			assert.Len(t, snapshot["first"].Calls, 1)
			assert.Len(t, snapshot["second"].Calls, 1)
		})
	}

	t.Run("an empty queue has no services registered", func(t *testing.T) {
		queue := servicequeue.NewBatchQueue[string, *string](logging.NewTestLogger(t), nil, "Do")
		_, err := queue.All(t.Context(), items)
		require.ErrorIs(t, err, servicequeue.ErrNoServicesRegistered)
		assert.Zero(t, queue.Len())
	})
}
