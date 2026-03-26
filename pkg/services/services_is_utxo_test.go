package services_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	ts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

func TestWalletServices_IsUtxo_SuccessCases(t *testing.T) {
	const (
		scriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"
		txidHex    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		txIndex    = uint32(0)
	)

	outpoint := &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex(txidHex),
		Index: txIndex,
	}

	cases := []struct {
		name       string
		jsonBody   string
		expectUTXO bool
	}{
		{
			name:       "is utxo",
			jsonBody:   `{"result":[{"tx_hash":"` + txidHex + `", "tx_pos":0, "height":800000, "value":123456}]}`,
			expectUTXO: true,
		},
		{
			name:       "not utxo - tx mismatch",
			jsonBody:   `{"result":[{"tx_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "tx_pos":1, "height":700000, "value":5000}]}`,
			expectUTXO: false,
		},
		{
			name:       "not utxo - empty result",
			jsonBody:   `{"result":[]}`,
			expectUTXO: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := ts.GivenServices(t)
			fixture.WhatsOnChain().WillRespondWithUtxoStatus(http.StatusOK, scriptHash, tc.jsonBody)

			svc := fixture.Services().New()
			isUtxo, err := svc.IsUtxo(t.Context(), scriptHash, outpoint)

			require.NoError(t, err)
			require.Equal(t, tc.expectUTXO, isUtxo)
		})
	}
}

func TestWalletServices_IsUtxo_ErrorCases(t *testing.T) {
	const scriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	outpoint := &transaction.Outpoint{
		Txid:  *testabilities.MustHashFromHex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Index: 0,
	}

	cases := []struct {
		name   string
		setup  func(ts.ServicesFixture)
		script string
	}{
		{
			name:   "api error in payload",
			script: scriptHash,
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithUtxoStatus(http.StatusOK, scriptHash,
					`{"result":[],"error":"invalid script"}`)
			},
		},
		{
			name:   "http 500 error",
			script: scriptHash,
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithUtxoStatus(http.StatusInternalServerError, scriptHash,
					`internal error`)
			},
		},
		{
			name:   "unreachable provider",
			script: scriptHash,
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
			},
		},
		{
			name:   "invalid script hash format",
			script: "badscript",
			setup:  func(f ts.ServicesFixture) {},
		},
		{
			name:   "empty script hash",
			script: "",
			setup:  func(f ts.ServicesFixture) {},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := ts.GivenServices(t)
			tc.setup(fixture)

			svc := fixture.Services().New()
			isUtxo, err := svc.IsUtxo(t.Context(), tc.script, outpoint)

			require.Error(t, err)
			require.False(t, isUtxo)
		})
	}
}
