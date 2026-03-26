package primitives_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestPositiveIntegerDefault10Max10000(t *testing.T) {
	t.Run("too large", func(t *testing.T) {
		// when:
		err := primitives.PositiveIntegerDefault10Max10000(10001).Validate()

		// then:
		require.Error(t, err)
	})

	tests := map[string]struct {
		value primitives.PositiveIntegerDefault10Max10000
	}{
		"valid": {
			value: 100,
		},
		"valid zero value": {
			value: 0,
		},
		"max value": {
			value: 10000,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			err := test.value.Validate()

			// then:
			require.NoError(t, err)
		})
	}
}

func TestSatoshiValue(t *testing.T) {
	t.Run("too large", func(t *testing.T) {
		// when:
		err := primitives.SatoshiValue(primitives.MaxSatoshis + 1).Validate()

		// then:
		require.Error(t, err)
	})

	tests := map[string]struct {
		value primitives.SatoshiValue
	}{
		"valid": {
			value: 100,
		},
		"valid zero value": {
			value: 0,
		},
		"max value": {
			value: primitives.MaxSatoshis,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			err := test.value.Validate()

			// then:
			require.NoError(t, err)
		})
	}
}
