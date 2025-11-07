package chaintracks_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks"
	"github.com/stretchr/testify/require"
)

func TestDirtyHashes(t *testing.T) {
	tests := map[string]struct {
		hash      string
		expected  bool
	}{
		"invalid SegWit chain": {
			hash:     "00000000000000000019f112ec0a9982926f1258cdcc558dd7c3b7e5dc7fa148",
			expected: true,
		},
		"invalid ABC chain": {
			hash:     "0000000000000000004626ff6e3b936941d341c5932ece4357eeccac44e6d56c",
			expected: true,
		},
		"valid hash": {
			hash:     "0000000000000000000b4d0b8c6f3e2a1b2c3d4e5f60718293a4b5c6d7e8f901",
			expected: false,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			isDirty := chaintracks.IsDirtyHash(test.hash)
			require.Equal(t, test.expected, isDirty)
		})
	}
}


