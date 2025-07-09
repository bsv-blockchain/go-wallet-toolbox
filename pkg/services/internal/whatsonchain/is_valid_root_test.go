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

	validRoot, err := chainhash.NewHashFromHex(tst.TestMerkleRootHex)
	require.NoError(t, err, "failed to parse test Merkle root hex")
	invalidRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()

	cases := []struct {
		name  string
		setup func(tst.WoCServiceFixture)
		root  *chainhash.Hash
		want1 want
		want2 want
	}{
		{
			name: "happy path + cache",
			setup: func(f tst.WoCServiceFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK,
						tst.TestBlockHeight, tst.TestMerkleRootHex)
			},
			root:  validRoot,
			want1: want{ok: true},
			want2: want{ok: true},
		},
		{
			name: "mismatching root",
			setup: func(f tst.WoCServiceFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK,
						tst.TestBlockHeight, tst.TestMerkleRootHex)
			},
			root:  invalidRoot,
			want1: want{ok: false},
			want2: want{ok: false},
		},
		{
			name: "height not found (404)",
			setup: func(f tst.WoCServiceFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusNotFound,
						tst.TestBlockHeight, "not found")
			},
			root:  validRoot,
			want1: want{ok: false},
			want2: want{ok: false},
		},
		{
			name: "retry succeeds after one failure",
			setup: func(f tst.WoCServiceFixture) {
				tr := f.WhatsOnChain().Transport()
				pat := `=~.*?/block/` + strconv.Itoa(int(tst.TestBlockHeight)) + `/header$`
				tr.RegisterResponder(http.MethodGet, pat,
					httpmock.NewStringResponder(http.StatusInternalServerError, "boom"))
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK,
						tst.TestBlockHeight, tst.TestMerkleRootHex)
			},
			root:  validRoot,
			want1: want{ok: true},
			want2: want{ok: true},
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

			// when:
			got, err := svc.IsValidRootForHeight(t.Context(), tc.root, tst.TestBlockHeight)

			// then:
			require.True(t, errors.Is(err, tc.want1.err))
			require.Equal(t, tc.want1.ok, got)

			// when:
			tr.Reset()
			got, err = svc.IsValidRootForHeight(t.Context(), tc.root, tst.TestBlockHeight)

			// then:
			require.True(t, errors.Is(err, tc.want2.err))
			require.Equal(t, tc.want2.ok, got)
			require.Equal(t, 0, tr.GetTotalCallCount())
		})
	}
}

func TestIsValidRootForHeight_ContextCancelled(t *testing.T) {
	root, err := chainhash.NewHashFromHex(tst.TestMerkleRootHex)
	require.NoError(t, err, "failed to parse test Merkle root hex")

	// given:
	fix := tst.Given(t)
	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~.*?/block/` + strconv.Itoa(int(tst.TestBlockHeight)) + `/header$`
	fix.WhatsOnChain().Transport().RegisterResponder(http.MethodGet, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})
	svc := fix.NewWoCService()
	tr := fix.WhatsOnChain().Transport()

	// when:
	got, err := svc.IsValidRootForHeight(ctx, root, tst.TestBlockHeight)

	// then:
	require.True(t, errors.Is(err, context.Canceled))
	require.False(t, got)

	// when:
	tr.Reset()
	got, err = svc.IsValidRootForHeight(ctx, root, tst.TestBlockHeight)

	// then:
	require.True(t, errors.Is(err, context.Canceled))
	require.False(t, got)
	require.Equal(t, 0, tr.GetTotalCallCount())
}
