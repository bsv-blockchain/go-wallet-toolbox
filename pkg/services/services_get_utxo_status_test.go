package services_test

import (
	"net/http"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	ts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

func TestWalletServices_GetUtxoStatus_SuccessCases(t *testing.T) {
	const (
		scriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"
		txidHex    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		txIndex    = uint32(0)
		height     = int64(800000)
		value      = uint64(123456)
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
			name:       "utxo found",
			jsonBody:   `{"result":[{"tx_hash":"` + txidHex + `", "tx_pos":0, "height":800000, "value":123456}]}`,
			expectUTXO: true,
		},
		{
			name:       "no matching outpoint",
			jsonBody:   `{"result":[{"tx_hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "tx_pos":1, "height":700000, "value":5000}]}`,
			expectUTXO: false,
		},
		{
			name:       "empty result array",
			jsonBody:   `{"result":[]}`,
			expectUTXO: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fixture := ts.GivenServices(t)
			fixture.WhatsOnChain().WillRespondWithUtxoStatus(http.StatusOK, scriptHash, tc.jsonBody)

			svc := fixture.Services().New()

			// when:
			result, err := svc.GetUtxoStatus(t.Context(), scriptHash, outpoint)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, whatsonchain.ServiceName, result.Name)

			if tc.expectUTXO {
				require.True(t, result.IsUtxo)
				require.Len(t, result.Details, 1)
				require.Equal(t, txidHex, result.Details[0].TxID)
				require.Equal(t, txIndex, result.Details[0].Index)
				require.Equal(t, height, result.Details[0].Height)
				require.Equal(t, value, result.Details[0].Satoshis)
			} else {
				require.False(t, result.IsUtxo)
			}
		})
	}
}

func TestWalletServices_GetUtxoStatus_ErrorCases(t *testing.T) {
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
			name:   "api error in response body",
			script: scriptHash,
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithUtxoStatus(http.StatusOK, scriptHash,
						`{"result":[],"error":"invalid script"}`)
			},
		},
		{
			name:   "http status error",
			script: scriptHash,
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithUtxoStatus(http.StatusInternalServerError, scriptHash,
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
			script: "invalid",
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
			// given:
			fixture := ts.GivenServices(t)
			tc.setup(fixture)

			svc := fixture.Services().New()

			// when:
			result, err := svc.GetUtxoStatus(t.Context(), tc.script, outpoint)

			// then:
			require.Error(t, err)
			require.Nil(t, result)
		})
	}
}
