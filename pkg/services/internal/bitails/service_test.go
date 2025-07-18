package bitails_test

import (
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	bt "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitails_GetHeight(t *testing.T) {
	// given:
	const good = uint32(123_456)

	given := bt.Given(t)
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
			given := bt.Given(t)
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
				given.Bitails().WillReturnLatestBlock(blockHash, uint32(height))
				given.Bitails().WillReturnBlockHeader(blockHash, headerHex)
			},
			want: func() *wdk.ChainBlockHeader {
				want, err := bitails.ConvertHeader(rawHeader, uint32(height))
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

func TestBitails_PostBEEF(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
	require.NoError(t, err)

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
			result, err := bitailsService.PostBEEF(t.Context(), beef, []string{givenTxID})

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.TxIDResults, 1)

			singleResult := result.TxIDResults[0]
			assert.Equal(t, test.resultStatus, singleResult.Result)
			assert.Equal(t, givenTxID, singleResult.TxID)
			assert.Nil(t, singleResult.Error)
			assert.False(t, singleResult.DoubleSpend)
			assert.Equal(t, test.alreadyKnown, singleResult.AlreadyKnown)
			assert.Len(t, singleResult.CompetingTxs, 0)
			assert.Len(t, singleResult.Notes, 1)
		})
	}
}

func TestBitails_PostBEEF_ErrorCases(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	givenTxID := txSpec.TX().TxID().String()

	beef, err := transaction.NewBeefFromTransaction(txSpec.TX())
	require.NoError(t, err)

	tests := map[string]struct {
		setup         func(testabilities.BitailsServiceFixture)
		resultStatus  wdk.PostedTxIDResultStatus
		doubleSpend   bool
		additionalErr bool
	}{
		"double spend - missing inputs": {
			setup: func(given testabilities.BitailsServiceFixture) {
				given.Bitails().OnBroadcast().WillReturnDoubleSpend(givenTxID, bitails.ErrMissingInputs)
				given.Bitails().WillReturnTxInfo(givenTxID, "mocked-block-hash", 99999)
			},
			resultStatus: wdk.PostedTxIDResultDoubleSpend,
			doubleSpend:  true,
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
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given := testabilities.Given(t)
			bitailsService := given.NewBitailsService()

			// and:
			test.setup(given)

			// when:
			result, err := bitailsService.PostBEEF(t.Context(), beef, []string{givenTxID})

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.TxIDResults, 1)

			singleResult := result.TxIDResults[0]
			assert.Equal(t, test.resultStatus, singleResult.Result)
			assert.Equal(t, givenTxID, singleResult.TxID)
			assert.Equal(t, test.doubleSpend, singleResult.DoubleSpend)
			assert.False(t, singleResult.AlreadyKnown)
			assert.Len(t, singleResult.Notes, 1)

			if test.additionalErr {
				assert.Error(t, singleResult.Error)
			}
		})
	}
}
