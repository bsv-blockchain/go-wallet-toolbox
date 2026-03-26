package primitives_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestOutpointString(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		// when:
		err := primitives.OutpointString("48656c6c6f.1").Validate()

		// then:
		require.NoError(t, err)
	})

	errorcases := map[string]struct {
		value primitives.OutpointString
	}{
		"missing dot separator": {
			value: "48656c6c6f1",
		},
		"too many dots": {
			value: "48656c6c.6f.1",
		},
		"non-numeric output index": {
			value: "48656c6c6f.abc",
		},
	}
	for name, test := range errorcases {
		t.Run(name, func(t *testing.T) {
			// when:
			err := test.value.Validate()

			// then:
			require.Error(t, err)
		})
	}
}
