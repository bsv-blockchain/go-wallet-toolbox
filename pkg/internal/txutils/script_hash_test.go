package txutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashOutputScript_SuccessCases(t *testing.T) {
	// given:
	tests := []struct {
		name       string
		scriptHex  string
		expectedLE string
	}{
		{
			name:       "Valid P2PKH",
			scriptHex:  "76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac",
			expectedLE: "db46d31e84e16e7fb031b3ab375131a7bb65775c0818dc17fe0d4444efb3d0aa",
		},
		{
			name:       "Empty script",
			scriptHex:  "",
			expectedLE: "55b852781b9995a44c939b64e441ae2724b96f99c8f4fb9a141cfc9842c4b0e3",
		},
		{
			name:       "Short script 0x00",
			scriptHex:  "00",
			expectedLE: "1da0af1706a31185763837b33f1d90782c0a78bbe644a59c987ab3ff9c0b346e",
		},
		{
			name:       "Valid P2SH",
			scriptHex:  "a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba87",
			expectedLE: "e7e41b1311c9fc8248e8f6e87cc382ca4b1af9c3189bb896712c3aebdf018639",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when:
			got, err := HashOutputScript(tt.scriptHex)

			// then:
			require.NoError(t, err)
			assert.Equal(t, tt.expectedLE, got)
		})
	}
}

func TestHashOutputScript_ErrorCases(t *testing.T) {
	// given:
	tests := []struct {
		name      string
		scriptHex string
	}{
		{
			name:      "Invalid hex input",
			scriptHex: "zzzz",
		},
		{
			name:      "Odd-length hex input",
			scriptHex: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when:
			_, err := HashOutputScript(tt.scriptHex)

			// then:
			assert.Error(t, err)
		})
	}
}
