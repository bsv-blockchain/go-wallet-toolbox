package services_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	wocTst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/stretchr/testify/require"
)

func TestWalletServices_IsValidRootForHeight(t *testing.T) {
	const height = wocTst.TestBlockHeight

	validRoot, _ := chainhash.NewHashFromHex(wocTst.TestMerkleRootHex)
	invalidRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()

	type want struct {
		ok        bool
		expectErr bool
	}

	cases := []struct {
		name  string
		setup func(ts.ServicesFixture)
		root  *chainhash.Hash
		want  want
	}{
		{
			name: "happy path",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK,
						height, wocTst.TestMerkleRootHex)
			},
			root: validRoot,
			want: want{ok: true},
		},
		{
			name: "mismatching root",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK,
						height, wocTst.TestMerkleRootHex)
			},
			root: invalidRoot,
			want: want{ok: false},
		},
		{
			name: "height not found (404)",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusNotFound,
						height, "not found")
			},
			root: validRoot,
			want: want{ok: false},
		},
		{
			name: "provider unreachable",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
			},
			root: validRoot,
			want: want{ok: false, expectErr: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fixture := ts.GivenServices(t)
			tc.setup(fixture)
			svc := fixture.Services().WithDefaultConfig()

			// when:
			ok, err := svc.IsValidRootForHeight(t.Context(), tc.root, height)

			// then:
			if tc.want.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want.ok, ok)
		})
	}
}

func TestWalletServices_IsValidRootForHeight_ContextCancelled(t *testing.T) {
	const height = wocTst.TestBlockHeight

	root, _ := chainhash.NewHashFromHex(wocTst.TestMerkleRootHex)

	// given:
	fixture := ts.GivenServices(t)
	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~/block/` + strconv.Itoa(int(height)) + `/header`
	fixture.WhatsOnChain().Transport().RegisterResponder(http.MethodGet, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})
	svc := fixture.Services().WithDefaultConfig()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, height)

	// then:
	require.True(t, errors.Is(err, context.Canceled))
	require.False(t, ok)
}
