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
	const overflow = uint(math.MaxUint32) + 42

	type setupFn func(fix testservices.BHSFixture)

	cases := []struct {
		name      string
		setup     setupFn
		wantErr   bool
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
		{
			name: "HTTP 500",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithInternalFailure()
			},
			wantErr: true,
		},
		{
			name: "empty body / zero height",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithEmptyLongestTipBlockHeader()
			},
			wantErr: true,
		},
		{
			name: "service unreachable",
			setup: func(f testservices.BHSFixture) {
				_ = f.WillBeUnreachable()
			},
			wantErr: true,
		},
		{
			name: "height overflows uint32",
			setup: func(f testservices.BHSFixture) {
				f.OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(overflow))
				f.IsUpAndRunning()
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := bhsTst.Given(t)
			tc.setup(fix.BHS())

			svc := fix.NewBHSService()

			// when:
			got, err := svc.GetHeight(t.Context())

			// then:
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantValue, got)
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
		{
			name: "HTTP 500",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithInternalFailure()
			},
			wantErr: true,
		},
		{
			name: "empty body",
			setup: func(f testservices.BHSFixture) {
				f.WillRespondWithEmptyLongestTipBlockHeader()
			},
			wantErr: true,
		},
		{
			name: "unreachable",
			setup: func(f testservices.BHSFixture) {
				_ = f.WillBeUnreachable()
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := bhsTst.Given(t)
			tc.setup(fix.BHS())

			svc := fix.NewBHSService()

			// when:
			got, err := svc.FindChainTipHeader(t.Context())

			// then:
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.Equal(t, makeExpected(), got)
		})
	}
}
