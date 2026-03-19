package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
)

func TestWalletServices_HashOutputScript_Success(t *testing.T) {
	tests := []struct {
		name       string
		scriptHex  string
		expectedLE string
	}{
		{
			name:       "Valid P2PKH Script",
			scriptHex:  "76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac",
			expectedLE: "db46d31e84e16e7fb031b3ab375131a7bb65775c0818dc17fe0d4444efb3d0aa",
		},
		{
			name:       "Empty Script",
			scriptHex:  "",
			expectedLE: "55b852781b9995a44c939b64e441ae2724b96f99c8f4fb9a141cfc9842c4b0e3",
		},
		{
			name:       "Short Script",
			scriptHex:  "00",
			expectedLE: "1da0af1706a31185763837b33f1d90782c0a78bbe644a59c987ab3ff9c0b346e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given:
			given := testservices.GivenServices(t)
			services := given.Services().New()

			// when:
			result, err := services.HashOutputScript(tt.scriptHex)

			// then:
			require.NoError(t, err)
			assert.Equal(t, tt.expectedLE, result)
		})
	}
}

func TestWalletServices_HashOutputScript_Errors(t *testing.T) {
	tests := []struct {
		name      string
		scriptHex string
	}{
		{
			name:      "Invalid Hex Input",
			scriptHex: "zzzz",
		},
		{
			name:      "Odd-length Hex Input",
			scriptHex: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given:
			given := testservices.GivenServices(t)
			services := given.Services().New()

			// when:
			_, err := services.HashOutputScript(tt.scriptHex)

			// then:
			assert.Error(t, err)
		})
	}
}
