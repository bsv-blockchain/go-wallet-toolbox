package bhs_test

import (
	"math"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	bhsTst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

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
	type setupFn func(fix testservices.BHSFixture)

	cases := []struct {
		name  string
		setup setupFn
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
				_ = f.WillBeUnreachable()
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

func TestBlockHeadersService_FindChainTipHeader1(t *testing.T) {
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

func TestBlockHeadersService_ErrorCase(t *testing.T) {
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
