package validate_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestValidRelinquishOutputArgs_Success(t *testing.T) {
	tests := map[string]struct {
		args *wdk.RelinquishOutputArgs
	}{
		"valid args": {
			args: &wdk.RelinquishOutputArgs{
				Output: fixtures.MockOutpoint,
				Basket: "validbasket",
			},
		},
		"valid: basket at min length": {
			args: &wdk.RelinquishOutputArgs{
				Output: fixtures.MockOutpoint,
				Basket: "a",
			},
		},
		"valid: basket at max length": {
			args: &wdk.RelinquishOutputArgs{
				Output: fixtures.MockOutpoint,
				Basket: strings.Repeat("a", 300),
			},
		},
		"valid: empty basket": {
			args: &wdk.RelinquishOutputArgs{
				Output: fixtures.MockOutpoint,
				Basket: "",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.ValidRelinquishOutputArgs(test.args)
			require.NoError(t, err)
		})
	}
}

func TestValidRelinquishOutputArgs_Error_InvalidOutpoint(t *testing.T) {
	tests := map[string]struct {
		args *wdk.RelinquishOutputArgs
	}{
		"missing dot": {
			args: &wdk.RelinquishOutputArgs{
				Output: "deadbeefcafebabe0",
				Basket: "validbasket",
			},
		},
		"index not numeric": {
			args: &wdk.RelinquishOutputArgs{
				Output: "deadbeefcafebabe.notanumber",
				Basket: "validbasket",
			},
		},
		"empty output": {
			args: &wdk.RelinquishOutputArgs{
				Output: "",
				Basket: "validbasket",
			},
		},
		"double dot": {
			args: &wdk.RelinquishOutputArgs{
				Output: "deadbeefcafebabe.1.0",
				Basket: "validbasket",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.ValidRelinquishOutputArgs(test.args)
			require.Error(t, err)
		})
	}
}

func TestValidRelinquishOutputArgs_Error_InvalidBasket(t *testing.T) {
	args := &wdk.RelinquishOutputArgs{
		Output: fixtures.MockOutpoint,
		Basket: strings.Repeat("a", 301),
	}

	err := validate.ValidRelinquishOutputArgs(args)
	require.Error(t, err)
}
