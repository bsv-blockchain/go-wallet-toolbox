package whatsonchain_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	tst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestIsValidRootForHeight(t *testing.T) {
	type want struct {
		ok  bool
		err error
	}

	validRoot, _ := chainhash.NewHashFromHex(tst.TestMerkleRootHex)
	invalidRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()

	var ctxDuringRequest context.Context

	cases := []struct {
		name  string
		setup func(tst.WoCServiceFixture)
		root  *chainhash.Hash
		ctx   context.Context
		want1 want
		want2 want
	}{
		{
			name: "happy path + cache",
			setup: func(f tst.WoCServiceFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK, tst.TestBlockHeight, tst.TestMerkleRootHex)
			},
			root:  validRoot,
			ctx:   t.Context(),
			want1: want{ok: true},
			want2: want{ok: true},
		},
		{
			name: "mismatching root",
			setup: func(f tst.WoCServiceFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK, tst.TestBlockHeight, tst.TestMerkleRootHex)
			},
			root:  invalidRoot,
			ctx:   t.Context(),
			want1: want{ok: false},
			want2: want{ok: false},
		},
		{
			name: "height not found (404)",
			setup: func(f tst.WoCServiceFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusNotFound, tst.TestBlockHeight, "not found")
			},
			root:  validRoot,
			ctx:   t.Context(),
			want1: want{ok: false},
			want2: want{ok: false},
		},
		{
			name: "retry succeeds after one failure",
			setup: func(f tst.WoCServiceFixture) {
				tr := f.WhatsOnChain().Transport()
				pat := `=~.*?/block/` + strconv.Itoa(int(tst.TestBlockHeight)) + `/header$`
				tr.RegisterResponder(http.MethodGet, pat,
					httpmock.NewStringResponder(http.StatusInternalServerError, "boom"),
				)
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK, tst.TestBlockHeight, tst.TestMerkleRootHex)
			},
			root:  validRoot,
			ctx:   t.Context(),
			want1: want{ok: true},
			want2: want{ok: true},
		},
		{
			name: "context canceled during request",
			setup: func(f tst.WoCServiceFixture) {
				local, cancel := context.WithCancelCause(t.Context())
				ctxDuringRequest = local
				pat := `=~.*?/block/` + strconv.Itoa(int(tst.TestBlockHeight)) + `/header$`
				f.WhatsOnChain().Transport().RegisterResponder(http.MethodGet, pat,
					func(_ *http.Request) (*http.Response, error) {
						cancel(context.Canceled)
						return nil, context.Canceled
					})
			},
			root:  validRoot,
			ctx:   nil, // replaced below
			want1: want{ok: false, err: context.Canceled},
			want2: want{ok: false, err: context.Canceled},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := tst.Given(t)
			if tc.setup != nil {
				tc.setup(fix)
			}
			svc := fix.NewWoCService()
			tr := fix.WhatsOnChain().Transport()

			ctx := tc.ctx
			if ctx == nil && ctxDuringRequest != nil {
				ctx = ctxDuringRequest
			}

			// when:
			got, err := svc.IsValidRootForHeight(ctx, tc.root, tst.TestBlockHeight)

			// then:
			require.True(t, errors.Is(err, tc.want1.err))
			require.Equal(t, tc.want1.ok, got)

			// when:
			tr.Reset()
			got, err = svc.IsValidRootForHeight(ctx, tc.root, tst.TestBlockHeight)

			// then:
			require.True(t, errors.Is(err, tc.want2.err))
			require.Equal(t, tc.want2.ok, got)
			require.Equal(t, 0, tr.GetTotalCallCount())
		})
	}
}
