package validate_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestValidBasketConfiguration_Success(t *testing.T) {
	tests := map[string]struct {
		config *wdk.BasketConfiguration
	}{
		"valid name": {
			config: &wdk.BasketConfiguration{
				Name: "ValidName",
			},
		},
		"exact 300 bytes": {
			config: &wdk.BasketConfiguration{
				Name: primitives.StringUnder300(strings.Repeat("a", 300)),
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.ValidBasketConfiguration(test.config)
			require.NoError(t, err)
		})
	}
}

func TestValidBasketConfiguration_Error(t *testing.T) {
	tests := map[string]struct {
		config      *wdk.BasketConfiguration
		expectedErr string
	}{
		"empty name": {
			config: &wdk.BasketConfiguration{
				Name: "",
			},
			expectedErr: "invalid Basket name: at least 1 length",
		},
		"name too long": {
			config: &wdk.BasketConfiguration{
				Name: primitives.StringUnder300(strings.Repeat("a", 301)),
			},
			expectedErr: "invalid Basket name: no more than 300 length",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.ValidBasketConfiguration(test.config)
			require.Error(t, err)
			assert.EqualError(t, err, test.expectedErr)
		})
	}
}
