package storage_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/brc114"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestListActions_BRC114_InvalidTimeLabels(t *testing.T) {
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	invalid := [][]string{
		{"action time from 0", "action time from 1"},
		{"action time to 1", "action time to 2"},
		{"action time from 2", "action time to 1"},
		{"action time from abc"},
		{"action time to -1"},
		{"action time from 9999999999999999999999999"},
		{"action time from 123abc"},
	}

	for _, labels := range invalid {
		t.Run(fmt.Sprintf("%v", labels), func(t *testing.T) {
			args := wdk.ListActionsArgs{
				Labels: primitives.ToStringUnder300Slice(labels),
				Limit:  10,
			}
			_, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)
			require.Error(t, err)
		})
	}
}

func TestListActions_BRC114_TimeFilteringAndResponseInjection(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	runLabel := fmt.Sprintf("brc114-%d", time.Now().UnixNano())
	// Space fixtures by 1s so second-precision stores still preserve order/boundaries.
	baseMs := int64(1704067200000) // 2024-01-01T00:00:00.000Z
	times := []int64{baseMs, baseMs + 1000, baseMs + 2000}

	type inserted struct {
		txID        string
		createdAtMs int64
		label       string
	}
	var actions [3]inserted

	for i, ts := range times {
		label := fmt.Sprintf("%s-%c", runLabel, 'a'+i)
		_, signedTx := given.Action(activeStorage).
			WithSatoshisToInternalize(uint64(50_000+i*1_000)).
			WithSatoshisToSend(uint64(1_000+i)).
			WithLabels(runLabel, label).
			Processed()

		txID := signedTx.TxID().String()
		createdAt := time.UnixMilli(ts).UTC()
		require.NoError(t, activeStorage.Database.DB.Model(&models.Transaction{}).
			Where("tx_id = ?", txID).
			Updates(map[string]any{
				"created_at": createdAt,
				"updated_at": createdAt,
			}).Error)

		actions[i] = inserted{
			txID:        txID,
			createdAtMs: ts,
			label:       label,
		}
	}

	a, b, c := actions[0], actions[1], actions[2]

	// Baseline without time filter: all three, no computed action-time labels.
	t.Run("baseline_no_time_filter", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels:        []primitives.StringUnder300{primitives.StringUnder300(runLabel)},
			IncludeLabels: to.Ptr(primitives.BooleanDefaultFalse(true)),
			Limit:         100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{a.txID, b.txID, c.txID}, actionTxIDs(result))
		for _, act := range result.Actions {
			assert.False(t, hasReservedTimeControlLabel(act.Labels))
			assert.Empty(t, injectedTimeLabels(act.Labels))
		}
	})

	// Time filter with includeLabels=false: still filters, but labels omitted.
	t.Run("from_zero_include_labels_false", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(brc114.ActionTimeFromPrefix + "0"),
			},
			IncludeLabels: to.Ptr(primitives.BooleanDefaultFalse(false)),
			Limit:         100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{a.txID, b.txID, c.txID}, actionTxIDs(result))
		for _, act := range result.Actions {
			assert.Empty(t, act.Labels)
		}
	})

	// Time filter with includeLabels=true: injects "action time {ms}".
	t.Run("from_zero_injects_time_labels", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(brc114.ActionTimeFromPrefix + "0"),
			},
			IncludeLabels: to.Ptr(primitives.BooleanDefaultFalse(true)),
			Limit:         100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{a.txID, b.txID, c.txID}, actionTxIDs(result))

		byTxID := map[string]wdk.WalletAction{}
		for _, act := range result.Actions {
			assert.False(t, hasReservedTimeControlLabel(act.Labels))
			require.Len(t, injectedTimeLabels(act.Labels), 1)
			byTxID[act.TxID] = act
		}
		assert.Contains(t, byTxID[a.txID].Labels, brc114.MakeActionTimeLabel(a.createdAtMs))
		assert.Contains(t, byTxID[b.txID].Labels, brc114.MakeActionTimeLabel(b.createdAtMs))
		assert.Contains(t, byTxID[c.txID].Labels, brc114.MakeActionTimeLabel(c.createdAtMs))
	})

	// Pagination still works under time filtering; includes belong only to returned page.
	t.Run("pagination_with_time_filter", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(brc114.ActionTimeFromPrefix + "0"),
			},
			IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
			IncludeOutputs: to.Ptr(primitives.BooleanDefaultFalse(true)),
			Limit:          2,
			Offset:         2,
		})
		require.NoError(t, err)
		assert.EqualValues(t, 3, result.TotalActions)
		require.Len(t, result.Actions, 1)
		// Labels and outputs are only for the returned page of actions.
		assert.NotEmpty(t, injectedTimeLabels(result.Actions[0].Labels))
		assert.NotEmpty(t, result.Actions[0].Outputs)
	})

	// from inclusive: b and c.
	t.Run("from_inclusive", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(fmt.Sprintf("%s%d", brc114.ActionTimeFromPrefix, b.createdAtMs)),
			},
			IncludeLabels: to.Ptr(primitives.BooleanDefaultFalse(true)),
			Limit:         100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{b.txID, c.txID}, actionTxIDs(result))
	})

	// to exclusive: a and b.
	t.Run("to_exclusive", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(fmt.Sprintf("%s%d", brc114.ActionTimeToPrefix, c.createdAtMs)),
			},
			IncludeLabels: to.Ptr(primitives.BooleanDefaultFalse(true)),
			Limit:         100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{a.txID, b.txID}, actionTxIDs(result))
	})

	// from inclusive + to exclusive: only b.
	t.Run("from_and_to_range", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(fmt.Sprintf("%s%d", brc114.ActionTimeFromPrefix, b.createdAtMs)),
				primitives.StringUnder300(fmt.Sprintf("%s%d", brc114.ActionTimeToPrefix, c.createdAtMs)),
			},
			IncludeLabels: to.Ptr(primitives.BooleanDefaultFalse(true)),
			Limit:         100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{b.txID}, actionTxIDs(result))
	})

	// Time labels are stripped before labelQueryMode=all matching.
	t.Run("label_query_mode_all_ignores_time_control_labels", func(t *testing.T) {
		baseline, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels:         []primitives.StringUnder300{primitives.StringUnder300(runLabel)},
			IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
			LabelQueryMode: to.Ptr(defs.QueryModeAll),
			Limit:          100,
		})
		require.NoError(t, err)

		withTime, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(brc114.ActionTimeFromPrefix + "0"),
			},
			IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
			LabelQueryMode: to.Ptr(defs.QueryModeAll),
			Limit:          100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, actionTxIDs(baseline), actionTxIDs(withTime))
	})

	// Ordinary label AND still works.
	t.Run("label_query_mode_all_with_ordinary_label", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(runLabel),
				primitives.StringUnder300(a.label),
			},
			IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
			LabelQueryMode: to.Ptr(defs.QueryModeAll),
			Limit:          100,
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{a.txID}, actionTxIDs(result))
	})

	// labelQueryMode=any + ordinary labels + time: intersection of time range and any-label match.
	t.Run("label_query_mode_any_with_time", func(t *testing.T) {
		result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), wdk.ListActionsArgs{
			Labels: []primitives.StringUnder300{
				primitives.StringUnder300(a.label),
				primitives.StringUnder300(c.label),
				primitives.StringUnder300(fmt.Sprintf("%s%d", brc114.ActionTimeFromPrefix, b.createdAtMs)),
			},
			IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
			LabelQueryMode: to.Ptr(defs.QueryModeAny),
			Limit:          100,
		})
		require.NoError(t, err)
		// a is before from → excluded by time; b not matched by labels; only c.
		assert.ElementsMatch(t, []string{c.txID}, actionTxIDs(result))
	})
}

func actionTxIDs(r *wdk.ListActionsResult) []string {
	out := make([]string, 0, len(r.Actions))
	for _, a := range r.Actions {
		out = append(out, a.TxID)
	}
	return out
}

func hasReservedTimeControlLabel(labels []string) bool {
	for _, l := range labels {
		if strings.HasPrefix(l, brc114.ActionTimeFromPrefix) || strings.HasPrefix(l, brc114.ActionTimeToPrefix) {
			return true
		}
	}
	return false
}

func injectedTimeLabels(labels []string) []string {
	var out []string
	for _, l := range labels {
		if strings.HasPrefix(l, brc114.ActionTimeFromPrefix) || strings.HasPrefix(l, brc114.ActionTimeToPrefix) {
			continue
		}
		if strings.HasPrefix(l, brc114.ActionTimeLabelPrefix) {
			rest := strings.TrimPrefix(l, brc114.ActionTimeLabelPrefix)
			if rest != "" && isAllDigitsBRC114(rest) {
				out = append(out, l)
			}
		}
	}
	return out
}

func isAllDigitsBRC114(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
