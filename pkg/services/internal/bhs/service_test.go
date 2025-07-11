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
	const good = uint(900_000)
	type setupFn func(fix testservices.BHSFixture)
	cases := []struct {
		name      string
		setup     setupFn
		wantValue uint32
	}{
		{
			name: "happy path",
			setup: func(f testservices.BHSFixture) {
				f.OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(good))
				f.IsUpAndRunning()
			},
			wantValue: uint32(good),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := bhsTst.Given(t)
			tc.setup(given.BHS())

			svc := given.NewBHSService()

			// when:
			got, err := svc.GetHeight(t.Context())

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.wantValue, got)
		})
	}
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
			_, err := svc.GetHeight(t.Context())

			// then:
			require.Error(t, err)
		})
	}
}

func TestBlockHeadersService_FindChainTipHeader(t *testing.T) {
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

	tests := []struct {
		name    string
		setup   func(testservices.BHSFixture)
		wantErr bool
	}{
		{
			name: "happy path",
			setup: func(f testservices.BHSFixture) {
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
			got, err := svc.FindChainTipHeader(t.Context())

			// then:
			require.NoError(t, err)
			require.Equal(t, makeExpected(), got)
		})
	}
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
