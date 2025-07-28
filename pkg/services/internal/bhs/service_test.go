package bhs_test

import (
	"math"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	bhsTst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestXYZ(t *testing.T) {
	// resty := httpx.NewRestyClientFactory().New()
	// service := bhs.New(resty, slog.Default(), defs.NetworkMainnet, defs.BHS{
	// 	URL:    "http://localhost:8080/api/v1",
	// 	APIKey: "mQZQ6WmxURxWz5ch",
	// })

	// res, err := service.GetChainHeaderByHeight(t.Context(), 1024)
	// require.NoError(t, err)
	// require.NotNil(t, res)

	data := wdk.ChainBaseBlockHeader{
		Version:      1,
		PreviousHash: "00000000e8817bc0ab40272591666f084b92130eee0e5172c04512dd6409fcb7",
		MerkleRoot:   "e4fec85d2803c115a10ed8276514ba76e769c1ca85a8c577b655fcdecc323ef1",
		Time:         1232367378,
		Bits:         0,
		Nonce:        366454553,
	}

	bb, err := data.Bytes()
	require.NoError(t, err)
	require.NotNil(t, bb)
}

func TestBlockHeadersService_GetHeight(t *testing.T) {
	// given:
	given := bhsTst.Given(t)

	const blockHeight = uint(900_000)

	givenBHS := given.BHS()
	givenBHS.OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(blockHeight))
	givenBHS.IsUpAndRunning()

	svc := given.NewBHSService()

	// when:
	got, err := svc.CurrentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, uint32(blockHeight), got)
}

func TestBlockHeadersService_GetHeight_ErrorCases(t *testing.T) {
	const overflow = uint(math.MaxUint32) + 42

	cases := []struct {
		name  string
		setup func(fix testservices.BHSFixture)
	}{
		{
			name: "HTTP 500",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithInternalFailure()
			},
		},
		{
			name: "empty body / zero height",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithEmptyLongestTipBlockHeader()
			},
		},
		{
			name: "service unreachable",
			setup: func(f testservices.BHSFixture) {
				err := f.WillBeUnreachable()
				require.Error(t, err)
			},
		},
		{
			name: "height overflows uint32",
			setup: func(f testservices.BHSFixture) {
				f.OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(overflow))
				f.IsUpAndRunning()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := bhsTst.Given(t)
			tc.setup(given.BHS())

			svc := given.NewBHSService()

			// when:
			_, err := svc.CurrentHeight(t.Context())

			// then:
			require.Error(t, err)
		})
	}
}

func TestBlockHeadersService_FindChainTipHeader(t *testing.T) {
	// given:
	base := testservices.NewBHSFixture(t)
	def := base.DefaultLongestTip()

	makeExpected := func() *wdk.ChainBlockHeader {
		return &wdk.ChainBlockHeader{
			ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
				Version:      def.Version,
				PreviousHash: def.PreviousBlock,
				MerkleRoot:   def.MerkleRoot,
				Time:         def.Timestamp,
				Nonce:        def.Nonce,
			},
			Height: def.Height,
			Hash:   def.Hash,
		}
	}

	given := bhsTst.Given(t)
	given.BHS().IsUpAndRunning()
	svc := given.NewBHSService()

	// when:
	got, err := svc.FindChainTipHeader(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, makeExpected(), got)
}

func TestBlockHeadersService_FindChainTipHeader_ErrorCase(t *testing.T) {
	tests := []struct {
		name  string
		setup func(testservices.BHSFixture)
	}{
		{
			name: "HTTP 500",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithInternalFailure()
			},
		},
		{
			name: "empty body",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithEmptyLongestTipBlockHeader()
			},
		},
		{
			name: "unreachable",
			setup: func(f testservices.BHSFixture) {
				_ = f.WillBeUnreachable()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := bhsTst.Given(t)
			tc.setup(given.BHS())

			svc := given.NewBHSService()

			// when:
			got, err := svc.FindChainTipHeader(t.Context())

			// then:
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
}

func TestBlockHeadersService_IsValidRootForHeight(t *testing.T) {
	const (
		height = uint32(900_123)
		root   = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	)

	validRoot, err := chainhash.NewHashFromHex(root)
	require.NoError(t, err)

	tests := []struct {
		name      string
		setup     func(fix testservices.BHSFixture)
		wantValid bool
	}{
		{
			name: "confirmed",
			setup: func(f testservices.BHSFixture) {
				f.OnMerkleRootVerifyResponse(height, validRoot.String(), "CONFIRMED")
				f.IsUpAndRunning()
			},
			wantValid: true,
		},
		{
			name: "invalid root",
			setup: func(f testservices.BHSFixture) {
				f.OnMerkleRootVerifyResponse(height, validRoot.String(), "INVALID")
				f.IsUpAndRunning()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := bhsTst.Given(t)
			tc.setup(given.BHS())
			svc := given.NewBHSService()

			// when:
			got, err := svc.IsValidRootForHeight(t.Context(), validRoot, height)

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.wantValid, got)
		})
	}
}

func TestBlockHeadersService_IsValidRootForHeight_Unverified(t *testing.T) {
	// given:
	const (
		height  = uint32(900_123)
		rootHex = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	)

	root, err := chainhash.NewHashFromHex(rootHex)
	require.NoError(t, err)

	given := bhsTst.Given(t)

	bhsFx := given.BHS()
	bhsFx.OnMerkleRootVerifyResponse(height, rootHex, "UNABLE_TO_VERIFY")
	bhsFx.IsUpAndRunning()

	svc := given.NewBHSService()

	// when:
	ok, err := svc.IsValidRootForHeight(t.Context(), root, height)

	// then:
	require.Error(t, err)
	require.False(t, ok)
}
