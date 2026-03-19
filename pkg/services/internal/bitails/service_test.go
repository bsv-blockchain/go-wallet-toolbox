package bitails_test

import (
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestBitails_GetHeight(t *testing.T) {
	// given:
	const good = uint32(123_456)

	given := testabilities.Given(t)
	given.Bitails().WillReturnNetworkInfo(http.StatusOK, good)

	// when:
	got, err := given.NewBitailsService().CurrentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, good, got)
}

func TestBitails_GetHeight_ErrorCases(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		blocks      uint32
		expectValue uint32
	}{
		{"non-200", http.StatusBadGateway, 0, 0},
		{"zero height", http.StatusOK, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			given.Bitails().WillReturnNetworkInfo(tc.status, tc.blocks)

			// when:
			_, err := given.NewBitailsService().CurrentHeight(t.Context())

			// then:
			require.Error(t, err)
		})
	}
}

func TestBitails_FindChainTipHeader(t *testing.T) {
	headerHex := testabilities.TestFakeHeaderBinary
	rawHeader, err := hex.DecodeString(headerHex)
	require.NoError(t, err)

	blockHash := chainhash.DoubleHashH(rawHeader).String()
	height := testabilities.TestBlockHeight

	tests := []struct {
		name  string
		setup func(testabilities.BitailsServiceFixture)
		want  *wdk.ChainBlockHeader
	}{
		{
			name: "happy path",
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().WillReturnLatestBlock(blockHash, uint32(height)) //nolint:gosec // block height fits in uint32
				given.Bitails().WillReturnBlockHeader(blockHash, headerHex)
			},
			want: func() *wdk.ChainBlockHeader {
				want, err := bitails.ConvertHeader(rawHeader, uint32(height)) //nolint:gosec // block height fits in uint32
				require.NoError(t, err)
				return want
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			service := given.NewBitailsService()

			tc.setup(given)

			// when:
			got, err := service.FindChainTipHeader(t.Context())

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBitails_FindChainTipHeader_ErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(testabilities.BitailsServiceFixture)
	}{
		{
			name: "HTTP 500 (internal error)",
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().WillRespondWithInternalFailure()
			},
		},

		{
			name: "empty body from /block/latest",
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().WillReturnLatestBlock("", 0)
			},
		},
		{
			name: "service unreachable",
			setup: func(given testabilities.BitailsServiceFixture) {
				_ = given.Bitails().WillBeUnreachable()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			service := given.NewBitailsService()

			// and:
			tc.setup(given)

			// when:
			got, err := service.FindChainTipHeader(t.Context())

			// then:
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestBitails_MerklePath(t *testing.T) {
	// given:
	fixture := testabilities.Given(t)
	service := fixture.NewBitailsService()

	txID := testabilities.TestTxID
	blockHash := testabilities.TestTargetHash
	siblingHash := testabilities.TestSiblingHash
	height := testabilities.TestBlockHeight

	fixture.Bitails().WillReturnTscProof(txID, blockHash, 1, []string{siblingHash})
	fixture.Bitails().WillReturnBlockHeader(blockHash, testabilities.TestFakeHeaderBinary)
	fixture.Bitails().WillReturnTxInfo(txID, blockHash, int64(height))

	// when:
	ctx := t.Context()
	result, err := service.MerklePath(ctx, txID)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, bitails.ServiceName, result.Name)
	assert.NotNil(t, result.MerklePath)
	assert.NotNil(t, result.BlockHeader)

	require.Len(t, result.Notes, 1)
	assert.Contains(t, result.Notes[0].What, "getMerklePath")
	assert.WithinDuration(t, time.Now(), result.Notes[0].When, 2*time.Second)
}

func TestBitails_PostTX(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	rawTx := txSpec.TX().Bytes()

	tests := map[string]struct {
		setup        func(testabilities.BitailsServiceFixture)
		resultStatus wdk.PostedTxIDResultStatus
		alreadyKnown bool
	}{
		"success - matching txid": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnSuccess(givenTxID)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultSuccess,
		},
		"success - already in mempool": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnAlreadyInMempool(givenTxID, bitails.ErrAlreadyKnown)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultAlreadyKnown,
			alreadyKnown: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			bitailsService := given.NewBitailsService()

			// and:
			test.setup(given)

			// when:
			result, err := bitailsService.PostTX(t.Context(), rawTx)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, test.resultStatus, result.Result)
			assert.Equal(t, givenTxID, result.TxID)
			require.NoError(t, result.Error)
			assert.False(t, result.DoubleSpend)
			assert.Equal(t, test.alreadyKnown, result.AlreadyKnown)
			assert.Empty(t, result.CompetingTxs)
			assert.Len(t, result.Notes, 1)
		})
	}
}

func TestBitails_PostTX_ErrorCases(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	rawTx := txSpec.TX().Bytes()

	tests := map[string]struct {
		setup         func(testabilities.BitailsServiceFixture)
		resultStatus  wdk.PostedTxIDResultStatus
		doubleSpend   bool
		additionalErr bool
	}{
		"double spend": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnDoubleSpend(givenTxID, bitails.ErrDoubleSpend)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultDoubleSpend,
			doubleSpend:  true,
		},
		"missing inputs": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnMissingInputs(givenTxID, bitails.ErrMissingInputs)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultMissingInputs,
			doubleSpend:  false,
		},
		"mismatched txid": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnSuccess("othertxid987")
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
		"internal error": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnHttpError(http.StatusInternalServerError)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
		"network error - ECONNREFUSED": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnEconnRefused(givenTxID, bitails.ErrMissingInputs)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
		"network error - ECONNRESET": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnEconnReset(givenTxID, bitails.ErrMissingInputs)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus:  wdk.PostedTxIDResultError,
			additionalErr: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			bitailsService := given.NewBitailsService()

			// and:
			test.setup(given)

			// when:
			result, err := bitailsService.PostTX(t.Context(), rawTx)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, test.resultStatus, result.Result)
			assert.Equal(t, givenTxID, result.TxID)
			assert.Equal(t, test.doubleSpend, result.DoubleSpend)
			assert.False(t, result.AlreadyKnown)
			assert.Len(t, result.Notes, 1)

			if test.additionalErr {
				assert.Error(t, result.Error)
			}
		})
	}
}

func TestBitails_RawTx(t *testing.T) {
	// given:
	txSpec := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(90)
	tx := txSpec.TX()
	rawHex := hex.EncodeToString(tx.Bytes())
	txID := tx.TxID().String()

	given := testabilities.Given(t)
	service := given.NewBitailsService()
	given.Bitails().WillReturnRawTxHex(txID, rawHex)

	// when:
	result, err := service.RawTx(t.Context(), txID)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, txID, result.TxID)
	assert.Equal(t, tx.Bytes(), result.RawTx)
	assert.Equal(t, bitails.ServiceName, result.Name)
}

func TestBitails_RawTx_ErrorCases(t *testing.T) {
	given := testabilities.Given(t)
	service := given.NewBitailsService()

	txSpec := testvectors.GivenTX().WithInput(100).WithP2PKHOutput(90)
	tx := txSpec.TX()
	txID := tx.TxID().String()

	tests := []struct {
		name    string
		setup   func()
		wantErr string
	}{
		{
			name: "malformed hex",
			setup: func() {
				given.Bitails().WillReturnRawTxHex(txID, "zzzzz1234badhex")
			},
			wantErr: "decode hex failed",
		},
		{
			name: "txid mismatch",
			setup: func() {
				otherTxSpec := testvectors.GivenTX().WithInput(50).WithP2PKHOutput(40)
				otherRawHex := hex.EncodeToString(otherTxSpec.TX().Bytes())
				given.Bitails().WillReturnRawTxHex(txID, otherRawHex)
			},
			wantErr: "txID mismatch",
		},
		{
			name: "unexpected HTTP error",
			setup: func() {
				given.Bitails().WillReturnRawTxHttpError(txID, http.StatusInternalServerError)
			},
			wantErr: "unexpected HTTP 500",
		},
		{
			name: "network/client failure",
			setup: func() {
				_ = given.Bitails().WillBeUnreachable()
			},
			wantErr: "unexpected HTTP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given:
			tt.setup()

			// when:
			_, err := service.RawTx(t.Context(), txID)

			// then:
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestBitails_RawTx_NotFound(t *testing.T) {
	// given:
	given := testabilities.Given(t)
	service := given.NewBitailsService()

	txSpec := testvectors.GivenTX().WithInput(1).WithP2PKHOutput(1)
	tx := txSpec.TX()
	txID := tx.TxID().String()

	given.Bitails().WillReturnRawTx404(txID)

	// when:
	res, err := service.RawTx(t.Context(), txID)

	// then:
	require.NoError(t, err)
	assert.Nil(t, res)
}
