package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChainWork(t *testing.T) {
	tests := map[string]struct {
		bits              uint32
		previousChainWork string

		expectedWork      string
		expectedChainWork string
	}{
		"for genesis and the next block": {
			bits:              0x1d00ffff,
			previousChainWork: "0000000000000000000000000000000000000000000000000000000100010001",
			expectedWork:      "0000000000000000000000000000000000000000000000000000000100010001",
			expectedChainWork: "0000000000000000000000000000000000000000000000000000000200020002",
		},
		"for block 921963": {
			bits:              0x1823062c,
			previousChainWork: "0000000000000000000000000000000000000000016986d1fc1fad6fcecba393",
			expectedWork:      "0000000000000000000000000000000000000000000000074f2b116f18ddc958",
			expectedChainWork: "0000000000000000000000000000000000000000016986d94b4abedee7a96ceb",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			work := ChainWorkFromBits(test.bits)
			assert.Equal(t, test.expectedWork, work.To64PadHex())
			chainWork := work.AddChainWork(ChainWorkFromHex(test.previousChainWork))
			assert.Equal(t, test.expectedChainWork, chainWork.To64PadHex())
		})
	}
}

func TestCmpChainWork(t *testing.T) {
	tests := map[string]struct {
		cw1               ChainWork
		cw2               ChainWork
		expectedCmpResult int
	}{
		"genesis and other block": {
			cw1:               ChainWorkFromHex("0000000000000000000000000000000000000000000000000000000100010001"),
			cw2:               ChainWorkFromHex("0000000000000000000000000000000000000016986d94b4abedee7a96ceb"),
			expectedCmpResult: -1,
		},
		"equal chain works": {
			cw1: ChainWorkFromHex("000000000000000000000000000000000000000000000000000100010001"),
			cw2: ChainWorkFromHex("000000000000000000000000000000000000000000000000000100010001"),
		},
		"other block and genesis": {
			cw1:               ChainWorkFromHex("0000000000000000000000000000000000000016986d94b4abedee7a96ceb"),
			cw2:               ChainWorkFromHex("0000000000000000000000000000000000000000000000000100010001"),
			expectedCmpResult: 1,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.cw1.CmpChainWork(test.cw2)
			assert.Equal(t, test.expectedCmpResult, result)
		})
	}
}
