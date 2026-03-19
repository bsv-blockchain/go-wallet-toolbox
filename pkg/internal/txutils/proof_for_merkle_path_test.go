package txutils_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
)

func TestConvertTscProofToMerklePath(t *testing.T) {
	testCases := []struct {
		name        string
		txid        string
		index       int
		nodes       []string
		blockHeight uint32
		wantErr     bool
	}{
		{
			name:  "success - even index",
			txid:  "4d5e6f7a8b9c0d1e2f30415263748596a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c",
			index: 0,
			nodes: []string{
				strings.Repeat("a", 64),
				strings.Repeat("b", 64),
				strings.Repeat("c", 64),
			},
			blockHeight: 100,
			wantErr:     false,
		},
		{
			name:  "success - odd index",
			txid:  "5e4d3c2b1a0f9e8d7c6b5a4938271615141312110f0e0d0c0b0a090807060504",
			index: 1,
			nodes: []string{
				strings.Repeat("d", 64),
				strings.Repeat("e", 64),
				strings.Repeat("f", 64),
			},
			blockHeight: 200,
			wantErr:     false,
		},
		{
			name:  "success - duplicate node marker",
			txid:  "11f20e0d0c0b0a090807060504030201ffeeddccbbaa99887766554433221100",
			index: 1,
			nodes: []string{
				"*",
				strings.Repeat("b", 64),
			},
			blockHeight: 300,
			wantErr:     false,
		},
		{
			name:        "error - empty nodes list",
			txid:        "4d5e6f7a8b9c0d1e2f30415263748596a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c",
			index:       0,
			nodes:       []string{},
			blockHeight: 100,
			wantErr:     true,
		},
		{
			name:        "error - invalid txid",
			txid:        "invalid-txid",
			index:       0,
			nodes:       []string{strings.Repeat("a", 64)},
			blockHeight: 100,
			wantErr:     true,
		},
		{
			name:        "error - invalid node hash at level 0",
			txid:        "4d5e6f7a8b9c0d1e2f30415263748596a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c",
			index:       0,
			nodes:       []string{"invalid-node-hash"},
			blockHeight: 100,
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			txid := tc.txid
			index := tc.index
			nodes := tc.nodes
			blockHeight := tc.blockHeight

			// when:
			mp, err := txutils.ConvertTscProofToMerklePath(txid, index, nodes, blockHeight)

			// then:
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, mp)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, mp)
			require.Equal(t, blockHeight, mp.BlockHeight)

			if tc.name == "success - duplicate node marker" {
				require.NotNil(t, mp.Path[0][0].Duplicate, "first level sibling should be marked duplicate")
			}
		})
	}
}
