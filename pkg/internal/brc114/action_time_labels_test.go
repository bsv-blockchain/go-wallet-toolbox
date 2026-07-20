package brc114_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/brc114"
)

func TestParseActionTimeLabels_NoTimeLabels(t *testing.T) {
	parsed, err := brc114.ParseActionTimeLabels([]string{"foo", "bar"})
	require.NoError(t, err)
	assert.False(t, parsed.TimeFilterRequested)
	assert.Nil(t, parsed.From)
	assert.Nil(t, parsed.To)
	assert.Equal(t, []string{"foo", "bar"}, parsed.RemainingLabels)
}

func TestParseActionTimeLabels_FromOnly(t *testing.T) {
	parsed, err := brc114.ParseActionTimeLabels([]string{"action time from 0"})
	require.NoError(t, err)
	require.True(t, parsed.TimeFilterRequested)
	require.NotNil(t, parsed.From)
	assert.Equal(t, int64(0), *parsed.From)
	assert.Nil(t, parsed.To)
	assert.Empty(t, parsed.RemainingLabels)
}

func TestParseActionTimeLabels_ToOnly(t *testing.T) {
	parsed, err := brc114.ParseActionTimeLabels([]string{"action time to 9"})
	require.NoError(t, err)
	require.True(t, parsed.TimeFilterRequested)
	assert.Nil(t, parsed.From)
	require.NotNil(t, parsed.To)
	assert.Equal(t, int64(9), *parsed.To)
	assert.Empty(t, parsed.RemainingLabels)
}

func TestParseActionTimeLabels_FromAndTo(t *testing.T) {
	parsed, err := brc114.ParseActionTimeLabels([]string{
		"run-label",
		"action time from 1704067200000",
		"action time to 1704067202000",
		"other",
	})
	require.NoError(t, err)
	require.True(t, parsed.TimeFilterRequested)
	require.NotNil(t, parsed.From)
	require.NotNil(t, parsed.To)
	assert.Equal(t, int64(1704067200000), *parsed.From)
	assert.Equal(t, int64(1704067202000), *parsed.To)
	assert.Equal(t, []string{"run-label", "other"}, parsed.RemainingLabels)
}

func TestParseActionTimeLabels_PreservesOrderOfRemaining(t *testing.T) {
	parsed, err := brc114.ParseActionTimeLabels([]string{
		"a",
		"action time from 1",
		"b",
		"action time to 9",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, parsed.RemainingLabels)
	require.NotNil(t, parsed.From)
	require.NotNil(t, parsed.To)
	assert.Equal(t, int64(1), *parsed.From)
	assert.Equal(t, int64(9), *parsed.To)
}

func TestParseActionTimeLabels_OrdinaryActionTimeLabelRemains(t *testing.T) {
	// "action time 123" (no from/to) is a response form, not a control — keep it.
	parsed, err := brc114.ParseActionTimeLabels([]string{"action time 123", "run"})
	require.NoError(t, err)
	assert.False(t, parsed.TimeFilterRequested)
	assert.Equal(t, []string{"action time 123", "run"}, parsed.RemainingLabels)
}

func TestParseActionTimeLabels_MaxSafeIntegerOK(t *testing.T) {
	label := brc114.ActionTimeFromPrefix + "9007199254740991"
	parsed, err := brc114.ParseActionTimeLabels([]string{label})
	require.NoError(t, err)
	require.NotNil(t, parsed.From)
	assert.Equal(t, brc114.MaxSafeInteger, *parsed.From)
}

func TestParseActionTimeLabels_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
	}{
		{"duplicate from", []string{"action time from 0", "action time from 1"}},
		{"duplicate to", []string{"action time to 1", "action time to 2"}},
		{"from > to", []string{"action time from 2", "action time to 1"}},
		{"from == to", []string{"action time from 5", "action time to 5"}},
		{"non-numeric from", []string{"action time from abc"}},
		{"empty from value", []string{"action time from "}},
		{"leading plus", []string{"action time from +1"}},
		{"negative to", []string{"action time to -1"}},
		{"whitespace in value", []string{"action time from 1 2"}},
		{"too large", []string{"action time from 9999999999999999999999999"}},
		{"above max safe integer", []string{"action time from 9007199254740992"}},
		{"trailing junk", []string{"action time from 123abc"}},
		{"decimal", []string{"action time from 1.5"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := brc114.ParseActionTimeLabels(tc.labels)
			require.Error(t, err)
		})
	}
}

func TestMakeActionTimeLabel(t *testing.T) {
	assert.Equal(t, "action time 0", brc114.MakeActionTimeLabel(0))
	assert.Equal(t, "action time 1704067200000", brc114.MakeActionTimeLabel(1704067200000))
}

func TestFromMillis(t *testing.T) {
	got := brc114.FromMillis(1704067200000)
	assert.Equal(t, int64(1704067200000), got.UnixMilli())
	assert.Equal(t, time.UTC, got.Location())
}
