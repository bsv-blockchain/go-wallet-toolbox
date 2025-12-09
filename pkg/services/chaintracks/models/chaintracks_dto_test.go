package models_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/stretchr/testify/require"
)

const (
	correctTwoHashedBaseHeaders = "00c046275b8a91ef8cecacb3ec7ed44652ada0da54434680b3ee1f130000000000000000a629bb863cd0e0e8ee103f98a0116e2f583de339f72347de53c2593f2444cad0d2f3f168faa22418426b621b000000388a0da2e9107ca07ab9db3df8a0979f372a318c3182aa2802000000000000000039f253b785ad25c117eb04fe633d36975d8397368ee6f9126470c37a7a4cdf15fbf4f16855982418f6ba4787"
)

func TestDTO_ToBaseBlockHeaders(t *testing.T) {
	tests := map[string]struct {
		hashed    string
		expected  []models.BaseBlockHeader
		expectErr bool
	}{
		"two headers hashed": {
			hashed: correctTwoHashedBaseHeaders,
			expected: []models.BaseBlockHeader{
				{
					Version:      0x2746c000,
					PreviousHash: "0000000000000000131feeb380464354daa0ad5246d47eecb3acec8cef918a5b",
					MerkleRoot:   "d0ca44243f59c253de4723f739e33d582f6e11a0983f10eee8e0d03c86bb29a6",
					Time:         1760687058,
					Bits:         0x1824a2fa,
					Nonce:        459434818,
				},
				{
					Version:      0x38000000,
					PreviousHash: "00000000000000000228aa82318c312a379f97a0f83ddbb97aa07c10e9a20d8a",
					MerkleRoot:   "15df4c7a7ac3706412f9e68e3697835d97363d63fe04eb17c125ad85b753f239",
					Time:         1760687355,
					Bits:         0x18249855,
					Nonce:        2269625078,
				},
			},
		},
		"single header hashed": {
			hashed: correctTwoHashedBaseHeaders[:2*80], // first 80 bytes (160 hex chars)
			expected: []models.BaseBlockHeader{
				{
					Version:      0x2746c000,
					PreviousHash: "0000000000000000131feeb380464354daa0ad5246d47eecb3acec8cef918a5b",
					MerkleRoot:   "d0ca44243f59c253de4723f739e33d582f6e11a0983f10eee8e0d03c86bb29a6",
					Time:         1760687058,
					Bits:         0x1824a2fa,
					Nonce:        459434818,
				},
			},
		},
		"empty hashed": {
			hashed:   "",
			expected: []models.BaseBlockHeader{},
		},
		"invalid hashed - odd length": {
			hashed:    "00c046275b8a91ef8cecacb3ec7ed44652ada0da54434680b3ee1f130000000000000000a629bb863cd0e0e8ee103f98a0116e2f583de339f72347de53c2593f2444cad0d2f3f168faa22418426b621",
			expected:  nil,
			expectErr: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			hexEncodedBlockHeaders := models.HashedBaseHeaders(test.hashed)

			// when:
			actual, err := hexEncodedBlockHeaders.ToBaseBlockHeaders()

			// then:
			if test.expectErr {
				require.Error(t, err)
			} else {
				require.Len(t, actual, len(test.expected))

				for i := range actual {
					require.Equal(t, test.expected[i], *actual[i])
				}
			}
		})
	}
}
