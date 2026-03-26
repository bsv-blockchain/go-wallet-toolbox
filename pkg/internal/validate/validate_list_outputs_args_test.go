package validate_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestListOutputsArgs_Success(t *testing.T) {
	tests := map[string]*wdk.ListOutputsArgs{
		"valid args only paging": {
			Limit: 10,
		},
		"valid args with tag query mode: all": {
			Limit:        100,
			TagQueryMode: to.Ptr(defs.QueryModeAll),
		},
		"valid args with tag query mode: any": {
			Limit:        100,
			TagQueryMode: to.Ptr(defs.QueryModeAny),
		},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.ListOutputsArgs(args)
			require.NoError(t, err)
		})
	}
}

func TestListOutputsArgs_Error(t *testing.T) {
	tests := map[string]*wdk.ListOutputsArgs{
		"invalid txid": {
			Limit:      10,
			KnownTxids: []string{"invalidhex"},
		},
		"zero limit": {
			Limit: 0,
		},
		"wrong tag query": {
			Limit:        10,
			TagQueryMode: to.Ptr(defs.QueryMode("invalid")),
		},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			err := validate.ListOutputsArgs(args)
			require.Error(t, err)
		})
	}
}
