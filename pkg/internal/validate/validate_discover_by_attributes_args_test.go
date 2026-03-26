package validate_test

import (
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
)

func TestDiscoverByAttributesArgs_Success(t *testing.T) {
	tests := map[string]sdk.DiscoverByAttributesArgs{
		"valid args with single attribute": {
			Attributes: map[string]string{"key": "value"},
		},
		"valid args with multiple attributes": {
			Attributes: map[string]string{"key1": "val1", "key2": "val2"},
		},
		"valid args with limit": {
			Attributes: map[string]string{"key": "value"},
			Limit:      to.Ptr(uint32(100)),
		},
		"valid args with offset": {
			Attributes: map[string]string{"key": "value"},
			Offset:     to.Ptr(uint32(50)),
		},
		"valid args with max limit": {
			Attributes: map[string]string{"key": "value"},
			Limit:      to.Ptr(uint32(validate.MaxPaginationLimit)),
		},
		"valid args with max offset": {
			Attributes: map[string]string{"key": "value"},
			Offset:     to.Ptr(uint32(validate.MaxPaginationOffset)),
		},
		"valid args with attribute key at max length (50)": {
			Attributes: map[string]string{strings.Repeat("a", 50): "value"},
		},
		"valid args with attribute key at min length (1)": {
			Attributes: map[string]string{"a": "value"},
		},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.DiscoverByAttributesArgs(args)
			require.NoError(t, err)
		})
	}
}

func TestDiscoverByAttributesArgs_Error(t *testing.T) {
	tests := map[string]struct {
		args        sdk.DiscoverByAttributesArgs
		expectedErr string
	}{
		"empty attributes": {
			args:        sdk.DiscoverByAttributesArgs{},
			expectedErr: "attributes must be provided",
		},
		"nil attributes map": {
			args: sdk.DiscoverByAttributesArgs{
				Attributes: nil,
			},
			expectedErr: "attributes must be provided",
		},
		"limit below minimum": {
			args: sdk.DiscoverByAttributesArgs{
				Attributes: map[string]string{"key": "value"},
				Limit:      to.Ptr(uint32(0)),
			},
			expectedErr: "limit must be greater than 0",
		},
		"limit exceeds maximum": {
			args: sdk.DiscoverByAttributesArgs{
				Attributes: map[string]string{"key": "value"},
				Limit:      to.Ptr(uint32(validate.MaxPaginationLimit + 1)),
			},
			expectedErr: "limit exceeds max allowed value",
		},
		"offset exceeds maximum": {
			args: sdk.DiscoverByAttributesArgs{
				Attributes: map[string]string{"key": "value"},
				Offset:     to.Ptr(uint32(validate.MaxPaginationOffset + 1)),
			},
			expectedErr: "offset is too large",
		},
		"attribute key too long (51 chars)": {
			args: sdk.DiscoverByAttributesArgs{
				Attributes: map[string]string{strings.Repeat("a", 51): "value"},
			},
			expectedErr: "must be between 1 and 50",
		},
		"attribute key empty string": {
			args: sdk.DiscoverByAttributesArgs{
				Attributes: map[string]string{"": "value"},
			},
			expectedErr: "must be between 1 and 50",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.DiscoverByAttributesArgs(tc.args)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
