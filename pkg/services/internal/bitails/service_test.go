package bitails_test

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	bt "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/jarcoal/httpmock"
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
	// given:
	fixture := testabilities.Given(t)
	service := fixture.NewBitailsService()

	headerHex := testabilities.TestFakeHeaderBinary
	rawHeader, err := hex.DecodeString(headerHex)
	require.NoError(t, err)

	blockHash := chainhash.DoubleHashH(rawHeader).String()
	height := testabilities.TestBlockHeight

	client := fixture.Bitails().HttpClient()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name  string
		setup func()
		want  *wdk.ChainBlockHeader
	}{
		{
			name: "happy path",
			setup: func() {
				httpmock.Reset()
				fixture.Bitails().WillReturnLatestBlock(blockHash, uint32(height))
				fixture.Bitails().WillReturnBlockHeader(blockHash, headerHex)
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
			tc.setup()

			// when:
			got, err := service.FindChainTipHeader(t.Context())

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBitails_FindChainTipHeader_ErrorCases(t *testing.T) {
	// given:
	fixture := testabilities.Given(t)
	service := fixture.NewBitailsService()

	client := fixture.Bitails().HttpClient()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		name  string
		setup func()
	}{
		{
			name: "HTTP 500 (internal error)",
			setup: func() {
				httpmock.Reset()
				fixture.Bitails().WillRespondWithInternalFailure()
			},
		},

		{
			name: "empty body from /block/latest",
			setup: func() {
				httpmock.Reset()
				fixture.Bitails().WillReturnLatestBlock("", 0)
			},
		},
		{
			name: "service unreachable",
			setup: func() {
				httpmock.Reset()
				_ = fixture.Bitails().WillBeUnreachable()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			tc.setup()

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
	assert.WithinDuration(t, time.Now(), *result.Notes[0].When, 2*time.Second)
}

func TestBitails_PostBEEF(t *testing.T) {
	txSpec := testvectors.GivenTX().
		WithInput(100).
		WithP2PKHOutput(90)
	tx := txSpec.TX()
	calculatedTxID := tx.TxID().String()

	beef, err := transaction.NewBeefFromTransaction(tx)
	require.NoError(t, err)

	fixture := testabilities.Given(t)
	bitailsService := fixture.NewBitailsService()

	client := fixture.Bitails().HttpClient()
	httpmock.ActivateNonDefault(client.GetClient())
	defer httpmock.DeactivateAndReset()

	testCases := []struct {
		name               string
		setup              func()
		expectTxID         string
		expectErrorResult  bool
		expectDoubleSpend  bool
		expectAlreadyKnown bool
		expectNotes        []string
		expectBlockHeight  int64
		expectBlockHash    string
	}{
		{
			name: "success - matching txid",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnSuccess(calculatedTxID)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
		},
		{
			name: "already in mempool",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnAlreadyInMempool(calculatedTxID, bitails.ErrAlreadyKnown)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:         calculatedTxID,
			expectErrorResult:  false,
			expectAlreadyKnown: true,
			expectNotes:        []string{"already in mempool"},
		},
		{
			name: "double spend - missing inputs",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnDoubleSpend(calculatedTxID, bitails.ErrMissingInputs)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: false,
			expectDoubleSpend: true,
			expectNotes:       []string{"missing inputs"},
		},
		{
			name: "mismatched txid",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnSuccess("othertxid987")
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: true,
			expectNotes:       []string{"returned txid (othertxid987) does not match expected txid (f036a074ace427c5ebc3d0de89a63d4a5e7aabeff9fbc77435eb58f8dbfd59a9)"},
		},
		{
			name: "internal error",
			setup: func() {
				fixture.Bitails().OnBroadcast().WillReturnHttpError(http.StatusInternalServerError)
				fixture.Bitails().WillReturnTxInfo(calculatedTxID, "mocked-block-hash", 99999)
			},
			expectTxID:        calculatedTxID,
			expectErrorResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			tc.setup()

			// when:
			result, err := bitailsService.PostBEEF(t.Context(), beef, []string{calculatedTxID})

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.TxIDResults, 1)

			assertTxIDResult(t, result.TxIDResults[0], tc)
		})
	}
}

func assertTxIDResult(t *testing.T, got wdk.PostedTxID, tc struct {
	name               string
	setup              func()
	expectTxID         string
	expectErrorResult  bool
	expectDoubleSpend  bool
	expectAlreadyKnown bool
	expectNotes        []string
	expectBlockHeight  int64
	expectBlockHash    string
}) {
	require.Equal(t, tc.expectTxID, got.TxID)

	if tc.expectErrorResult {
		require.Equal(t, wdk.PostedTxIDResultError, got.Result)
	} else {
		require.NotEqual(t, wdk.PostedTxIDResultError, got.Result)
		require.Nil(t, got.Error)
	}

	require.Equal(t, tc.expectDoubleSpend, got.DoubleSpend)
	require.Equal(t, tc.expectAlreadyKnown, got.AlreadyKnown)
	require.Equal(t, tc.expectBlockHeight, got.BlockHeight)
	require.Equal(t, tc.expectBlockHash, got.BlockHash)

	strNotes := slices.Map(got.Notes, func(note wdk.ReqHistoryNote) string {
		return note.What
	})

	for _, expectedNote := range tc.expectNotes {
		assert.Condition(t, func() bool {
			for _, gotNote := range strNotes {
				if strings.Contains(gotNote, expectedNote) {
					return true
				}
			}
			return false
		}, fmt.Sprintf("Expected note to contain: %q, but got: %v", expectedNote, strNotes))
	}
}
