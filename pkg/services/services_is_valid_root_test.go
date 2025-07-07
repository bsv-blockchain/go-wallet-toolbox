package services_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	wocTst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/stretchr/testify/require"
)

func TestWalletServices_IsValidRootForHeight(t *testing.T) {
	const height = wocTst.TestBlockHeight

	validRoot, _ := chainhash.NewHashFromHex(wocTst.TestMerkleRootHex)
	invalidRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()

	var ctxDuringRequest context.Context

	type want struct {
		ok        bool
		expectErr bool
		canceled  bool
	}

	cases := []struct {
		name  string
		setup func(testservices.ServicesFixture)
		root  *chainhash.Hash
		ctx   context.Context
		want  want
	}{
		{
			name: "happy path",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK, height, wocTst.TestMerkleRootHex)
			},
			root: validRoot,
			ctx:  t.Context(),
			want: want{ok: true},
		},
		{
			name: "mismatching root",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK, height, wocTst.TestMerkleRootHex)
			},
			root: invalidRoot,
			ctx:  t.Context(),
			want: want{ok: false},
		},
		{
			name: "height not found (404)",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusNotFound, height, "not found")
			},
			root: validRoot,
			ctx:  t.Context(),
			want: want{ok: false},
		},
		{
			name: "provider unreachable",
			setup: func(f testservices.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
			},
			root: validRoot,
			ctx:  t.Context(),
			want: want{ok: false, expectErr: true},
		},
		{
			name: "context canceled during request",
			setup: func(f testservices.ServicesFixture) {
				localCtx, cancel := context.WithCancelCause(t.Context())
				ctxDuringRequest = localCtx

				pat := `=~/block/` + strconv.Itoa(int(height)) + `/header`
				f.WhatsOnChain().Transport().RegisterResponder(
					http.MethodGet, pat,
					func(_ *http.Request) (*http.Response, error) {
						cancel(context.Canceled)
						return nil, context.Canceled
					})
			},
			root: validRoot,
			ctx:  nil, // replaced below
			want: want{ok: false, expectErr: true, canceled: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := testservices.GivenServices(t)
			if tc.setup != nil {
				tc.setup(given)
			}
			svc := given.Services().WithDefaultConfig()

			ctx := tc.ctx
			if ctx == nil {
				ctx = ctxDuringRequest
			}

			// when:
			ok, err := svc.IsValidRootForHeight(ctx, tc.root, height)

			// then:
			if tc.want.expectErr {
				require.Error(t, err)
				if tc.want.canceled {
					require.True(t, errors.Is(err, context.Canceled))
				}
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want.ok, ok)
		})
	}
}
