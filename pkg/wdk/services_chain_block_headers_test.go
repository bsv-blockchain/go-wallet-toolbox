package wdk_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
)

func TestChainBaseBlockHeader_Bytes_PostiviePaths(t *testing.T) {
	tests := map[string]struct {
		block    *wdk.ChainBaseBlockHeader
		expected []byte
	}{
		"valid block header with version 1 and known hashes": {
			block: &wdk.ChainBaseBlockHeader{
				Version:      1,
				PreviousHash: "00000000e8817bc0ab40272591666f084b92130eee0e5172c04512dd6409fcb7",
				MerkleRoot:   "e4fec85d2803c115a10ed8276514ba76e769c1ca85a8c577b655fcdecc323ef1",
				Time:         1232367378,
				Bits:         0,
				Nonce:        366454553,
			},
			expected: []byte{
				0, 0, 0, 1, 183, 252, 9, 100, 221, 18, 69, 192,
				114, 81, 14, 238, 14, 19, 146, 75, 8, 111, 102, 145,
				37, 39, 64, 171, 192, 123, 129, 232, 0, 0, 0, 0,
				241, 62, 50, 204, 222, 252, 85, 182, 119, 197, 168, 133,
				202, 193, 105, 231, 118, 186, 20, 101, 39, 216, 14, 161,
				21, 193, 3, 40, 93, 200, 254, 228, 73, 116, 111, 18,
				0, 0, 0, 0, 21, 215, 167, 25,
			},
		},
		"valid block header with version 1024 and custom hashes": {
			block: &wdk.ChainBaseBlockHeader{
				Version:      1024,
				PreviousHash: "000000001546f288e1540d55b0a6b70f86c3fe0b29ca39ec7878c41f1f16ec5d",
				MerkleRoot:   "00000000edfa5bfffd21cc8ce76e46b79dc00196e61cdc62fd595316136f8a83",
				Time:         1232367968,
				Bits:         342,
				Nonce:        99540172,
			},
			expected: []byte{
				0, 0, 4, 0, 93, 236, 22, 31, 31, 196, 120, 120,
				236, 57, 202, 41, 11, 254, 195, 134, 15, 183, 166, 176,
				85, 13, 84, 225, 136, 242, 70, 21, 0, 0, 0, 0,
				131, 138, 111, 19, 22, 83, 89, 253, 98, 220, 28, 230,
				150, 1, 192, 157, 183, 70, 110, 231, 140, 204, 33, 253,
				255, 91, 250, 237, 0, 0, 0, 0, 73, 116, 113, 96,
				0, 0, 1, 86, 5, 238, 220, 204,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			actual, err := tc.block.Bytes()

			// then:
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
