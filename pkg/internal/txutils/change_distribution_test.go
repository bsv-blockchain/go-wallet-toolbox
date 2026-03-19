package txutils

import (
	"slices"
	"testing"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

func mockZeroRandomizer(_ uint64) uint64 {
	return 0
}

func mockMaxRandomizer(max uint64) uint64 {
	return max
}

func mockConstRandomizer(factors ...uint64) Randomizer {
	maxFactor := slices.Max(factors)
	i := 0
	return func(max uint64) uint64 {
		index := i % len(factors)
		i++
		return max * factors[index] / maxFactor
	}
}

func TestChangeDistribution(t *testing.T) {
	tests := map[string]struct {
		initialValue satoshi.Value
		randomizer   func(uint64) uint64
		count        uint64
		amount       satoshi.Value
		expected     []satoshi.Value
	}{
		"single output": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        1,
			amount:       5500,
			expected:     []satoshi.Value{5500},
		},
		"single output with one satoshi": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        1,
			amount:       1,
			expected:     []satoshi.Value{1},
		},
		"zero outputs": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        0,
			amount:       5500,
			expected:     []satoshi.Value(nil),
		},
		"zero amount": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       0,
			expected:     []satoshi.Value(nil),
		},
		"zero amount & zero count": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        0,
			amount:       0,
			expected:     []satoshi.Value(nil),
		},
		"not saturated: reminder + (count-1) * initialValue": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       5500,
			expected:     []satoshi.Value{500, 1000, 1000, 1000, 1000, 1000},
		},
		"not saturated: initialValue/4 + (count-1) * initialValue": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       5250,
			expected:     []satoshi.Value{250, 1000, 1000, 1000, 1000, 1000},
		},
		"equally saturated: (count) * initialValue": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       6000,
			expected:     []satoshi.Value{1000, 1000, 1000, 1000, 1000, 1000},
		},
		"saturated: equal distribution +1": {
			initialValue: 1000,
			randomizer:   mockMaxRandomizer,
			count:        6,
			amount:       6001,
			expected:     []satoshi.Value{1000, 1000, 1000, 1000, 1000, 1001},
		},
		"saturated: equal distribution": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       7200,
			expected:     []satoshi.Value{1200, 1200, 1200, 1200, 1200, 1200},
		},
		"saturated: not equal distribution": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       7201,
			expected:     []satoshi.Value{1201, 1200, 1200, 1200, 1200, 1200},
		},
		"saturated: not equal distribution - mockMaxRandomizer": {
			initialValue: 1000,
			randomizer:   mockMaxRandomizer,
			count:        6,
			amount:       7205,
			expected:     []satoshi.Value{1200, 1200, 1200, 1200, 1200, 1205},
		},
		"saturated: not equal distribution - constRandomizer": {
			initialValue: 1000,
			randomizer:   mockConstRandomizer(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10),
			count:        6,
			amount:       7205,
			expected:     []satoshi.Value{1305, 1260, 1220, 1180, 1140, 1100},
		},
		"saturated: zero initialValue": {
			initialValue: 0,
			randomizer:   mockMaxRandomizer,
			count:        6,
			amount:       7201,
			expected:     []satoshi.Value{1200, 1200, 1200, 1200, 1200, 1201},
		},

		"not saturated, minimal value: 1 + (count-1) * initialValue": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       5001,
			expected:     []satoshi.Value{1, 1000, 1000, 1000, 1000, 1000},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			dist := NewChangeDistribution(test.initialValue, test.randomizer)

			// when:
			values := dist.Distribute(test.count, test.amount)

			// then:
			require.Equal(t, test.expected, seq.Collect(values))
		})
	}
}

func TestChangeDistributionPanics(t *testing.T) {
	tests := map[string]struct {
		initialValue satoshi.Value
		randomizer   func(uint64) uint64
		count        uint64
		amount       satoshi.Value
	}{
		"reduced count: (count-1) * initialValue": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       5000,
		},
		"not saturated, reduced count: (count-1) * initialValue": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       4999,
		},
		"not saturated, reduced count to one": {
			initialValue: 1000,
			randomizer:   mockZeroRandomizer,
			count:        6,
			amount:       1,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			dist := NewChangeDistribution(test.initialValue, test.randomizer)

			f := func() {
				// when:
				dist.Distribute(test.count, test.amount)
			}

			// then:
			require.Panics(t, f)
		})
	}
}

func TestChangeDistributionWithActualRandomizer(t *testing.T) {
	// given:
	initialValue := satoshi.Value(1000)
	count := uint64(1000)

	// and:
	random := randomizer.New()

	// and:
	dist := NewChangeDistribution(initialValue, random.Uint64)

	// when:
	values := dist.Distribute(count, satoshi.MustMultiply(2*count, initialValue))

	// then:
	var i uint64
	var equalsToInitial uint64
	for v := range values {
		assert.GreaterOrEqual(t, v, initialValue, "value was randomized wrongly - it should be greater or equal to initialValue")
		if satoshi.MustEqual(v, initialValue) {
			equalsToInitial++
		}
		i++
	}
	require.Less(t, equalsToInitial, count, "random should not return equal values (it's ~0% chance to get equal values)")
}
