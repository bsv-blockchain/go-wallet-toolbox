package wdk_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestChainBaseBlockHeader_Bytes_PositivePaths(t *testing.T) {
	tests := map[string]struct {
		block         *wdk.ChainBaseBlockHeader
		expectedBytes []byte
		expectedHash  string
	}{
		"valid block header - height 2000, with version 1, time 1233046715, bits 486604799, nonce 2999858432 and known hashes": {
			block: &wdk.ChainBaseBlockHeader{
				Version:      1,
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
				MerkleRoot:   "10f072e631081ad6bcddeabb90bc34d787fe7d7116fe0298ff26c50c5e21bfea",
				Time:         1233046715,
				Bits:         486604799,
				Nonce:        2999858432,
			},
			expectedHash: "00000000dfd5d65c9d8561b4b8f60a63018fe3933ecb131fb37f905f87da951a",
			expectedBytes: []byte{
				1, 0, 0, 0, 38, 162, 46, 60, 45, 25, 212, 154, 28, 28, 184, 166,
				142, 106, 183, 116, 64, 195, 14, 89, 116, 64, 74, 42, 128, 109, 73, 161,
				0, 0, 0, 0, 234, 191, 33, 94, 12, 197, 38, 255, 152, 2, 254, 22,
				113, 125, 254, 135, 215, 52, 188, 144, 187, 234, 221, 188, 214, 26, 8, 49,
				230, 114, 240, 16, 187, 204, 126, 73, 255, 255, 0, 29, 0, 53, 206, 178,
			},
		},
		"valid block header - height 4000, with version 1, time 1234438726, bits 486604799, nonce 481589526 and known hashes": {
			block: &wdk.ChainBaseBlockHeader{
				Version:      1,
				PreviousHash: "00000000690d22ab76cbb5eca33cb018e36aebe4648e6ed79791aefe0f936e07",
				MerkleRoot:   "bd19840186d17a3ac904eca54400d7e95a06ff62f4682181a2a893702abd4377",
				Time:         1234438726,
				Bits:         486604799,
				Nonce:        481589526,
			},
			expectedHash: "00000000922e2aa9e84a474350a3555f49f06061fd49df50a9352f156692a842",
			expectedBytes: []byte{
				1, 0, 0, 0, 7, 110, 147, 15, 254, 174, 145, 151, 215, 110, 142, 100,
				228, 235, 106, 227, 24, 176, 60, 163, 236, 181, 203, 118, 171, 34, 13, 105,
				0, 0, 0, 0, 119, 67, 189, 42, 112, 147, 168, 162, 129, 33, 104, 244,
				98, 255, 6, 90, 233, 215, 0, 68, 165, 236, 4, 201, 58, 122, 209, 134,
				1, 132, 25, 189, 70, 10, 148, 73, 255, 255, 0, 29, 22, 121, 180, 28,
			},
		},
		"valid block header - height 6000, with version 1, time 1235927934, bits 486604799, nonce 3598075177 and known hashes": {
			block: &wdk.ChainBaseBlockHeader{
				Version:      1,
				PreviousHash: "00000000828cb497379bedf1d0657c297b388ee2dc0edcd2e6998b30a17272bf",
				MerkleRoot:   "dee533d8d0ac0b1f0ccb49b80f075fee63802a9fab9114a90e9c5ae866695731",
				Time:         1235927934,
				Bits:         486604799,
				Nonce:        3598075177,
			},
			expectedHash: "00000000dbbb79792303bdd1c6c4d7ab9c21bba0667213c2eca955e11230c5a5",
			expectedBytes: []byte{
				1, 0, 0, 0, 191, 114, 114, 161, 48, 139, 153, 230, 210, 220, 14, 220,
				226, 142, 56, 123, 41, 124, 101, 208, 241, 237, 155, 55, 151, 180, 140, 130,
				0, 0, 0, 0, 49, 87, 105, 102, 232, 90, 156, 14, 169, 20, 145, 171,
				159, 42, 128, 99, 238, 95, 7, 15, 184, 73, 203, 12, 31, 11, 172, 208,
				216, 51, 229, 222, 126, 195, 170, 73, 255, 255, 0, 29, 41, 69, 118, 214,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			actualBytes, err := tc.block.Bytes()

			// then:
			require.NoError(t, err)
			assert.Equal(t, tc.expectedBytes, actualBytes)
			assertBlockHash(t, tc.expectedHash, actualBytes)
		})
	}
}

func TestChainBaseBlockHeader_Bytes_NegativePaths(t *testing.T) {
	tests := map[string]struct {
		block *wdk.ChainBaseBlockHeader
	}{
		"invalid block header - the 'previous hash' value has length greater than 32": {
			block: &wdk.ChainBaseBlockHeader{
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226a",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
			},
		},
		"invalid block header - the 'previous hash' value has length less than 32": {
			block: &wdk.ChainBaseBlockHeader{
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
			},
		},
		"invalid block header - the 'merkle root' value has length greater than 32": {
			block: &wdk.ChainBaseBlockHeader{
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226a",
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
			},
		},
		"invalid block header - the 'merkle root' value has length less than 32": {
			block: &wdk.ChainBaseBlockHeader{
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea",
			},
		},
		"invalid block header - the 'merkle root', 'previous hash' values lengths are less than 32": {
			block: &wdk.ChainBaseBlockHeader{
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea",
			},
		},
		"invalid block header - the 'merkle root', 'previous hash' values lengths are greater than 32": {
			block: &wdk.ChainBaseBlockHeader{
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226aa",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226aa",
			},
		},
		"invalid block header - the 'previous hash' field is defined as a non-hex value": {
			block: &wdk.ChainBaseBlockHeader{
				PreviousHash: "#@A",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
			},
		},
		"invalid block header - the 'merkle root' field is defined as a non-hex value": {
			block: &wdk.ChainBaseBlockHeader{
				MerkleRoot:   "#@A",
				PreviousHash: "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
			},
		},
		"invalid block header - both 'merkle root' and 'previous hash' fields are defined as non-hex value": {
			block: &wdk.ChainBaseBlockHeader{
				MerkleRoot:   "#@A",
				PreviousHash: "#@!",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			actualBytes, err := tc.block.Bytes()

			// then:
			require.Error(t, err)
			assert.Nil(t, actualBytes)
		})
	}
}

func TestChainBaseBlockHeader_ToAndFromBytes(t *testing.T) {
	tests := map[string]struct {
		block *wdk.ChainBaseBlockHeader
	}{
		"valid block header - height 3000, with version 1, time 1233748223, bits 486604799, nonce 2650070842 and known hashes": {
			block: &wdk.ChainBaseBlockHeader{
				Version:      1,
				PreviousHash: "00000000690d22ab76cbb5eca33cb018e36aebe4648e6ed79791aefe0f936e07",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
				Time:         1233748223,
				Bits:         486604799,
				Nonce:        2650070842,
			},
		},
		"valid block header - height 5000, with version 1, time 1235123456, bits 486604799, nonce 1234567890 and known hashes": {
			block: &wdk.ChainBaseBlockHeader{
				Version:      1,
				PreviousHash: "00000000690d22ab76cbb5eca33cb018e36aebe4648e6ed79791aefe0f936e07",
				MerkleRoot:   "00000000a1496d802a4a4074590ec34074b76a8ea6b81c1c9ad4192d3c2ea226",
				Time:         1235123456,
				Bits:         486604799,
				Nonce:        1234567890,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			bytes, err := tc.block.Bytes()
			require.NoError(t, err)

			actualBlock, err := wdk.ChainBaseBlockHeaderFromBytes(bytes)

			// then:
			require.NoError(t, err)
			assert.Equal(t, tc.block, actualBlock)
		})
	}
}

func assertBlockHash(t *testing.T, expectedHash string, fingerprint []byte) {
	actualHash := chainhash.DoubleHashH(fingerprint)
	assert.Equal(t, expectedHash, actualHash.String(), "Double hash does not match expected value")
}
