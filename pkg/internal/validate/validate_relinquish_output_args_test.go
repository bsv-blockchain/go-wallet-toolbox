package validate

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/stretchr/testify/require"
)

func TestValidRelinquishOutputArgs(t *testing.T) {
	err := ValidRelinquishOutputArgs(&sdk.RelinquishOutputArgs{
		Output: fixtures.MockOutpoint,
	})
	require.NoError(t, err)
}

func TestWrongRelinquishOutputArgs(t *testing.T) {
	tests := map[string]struct {
		args *sdk.RelinquishOutputArgs
	}{
		"invalid outpoint: missing dot": {
			args: &sdk.RelinquishOutputArgs{
				Output: "notavalidoutpoint",
			},
		},
		"invalid outpoint: index not numeric": {
			args: &sdk.RelinquishOutputArgs{
				Output: "deadbeefcafebabe.notanumber",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidRelinquishOutputArgs(tt.args)
			require.Error(t, err)
		})
	}
}
