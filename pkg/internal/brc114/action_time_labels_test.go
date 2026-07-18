package brc114_test

import (
	"testing"

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

func TestParseActionTimeLabels_FromOnly(t *testing.T) {
	parsed, err := brc114.ParseActionTimeLabels([]string{"action time from 0"})
	require.NoError(t, err)
	require.True(t, parsed.TimeFilterRequested)
	require.NotNil(t, parsed.From)
	assert.Equal(t, int64(0), *parsed.From)
	assert.Nil(t, parsed.To)
	assert.Empty(t, parsed.RemainingLabels)
}

func TestParseActionTimeLabels_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
	}{
		{"duplicate from", []string{"action time from 0", "action time from 1"}},
		{"duplicate to", []string{"action time to 1", "action time to 2"}},
		{"from >= to", []string{"action time from 2", "action time to 1"}},
		{"from == to", []string{"action time from 5", "action time to 5"}},
		{"non-numeric from", []string{"action time from abc"}},
		{"negative to", []string{"action time to -1"}},
		{"too large", []string{"action time from 9999999999999999999999999"}},
		{"trailing junk", []string{"action time from 123abc"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := brc114.ParseActionTimeLabels(tc.labels)
			require.Error(t, err)
		})
	}
}

func TestMakeActionTimeLabel(t *testing.T) {
	assert.Equal(t, "action time 1704067200000", brc114.MakeActionTimeLabel(1704067200000))
}
