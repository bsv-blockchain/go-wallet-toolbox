package services_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-sdk/chainhash"

	ts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	btTst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	wocTst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

func TestWalletServices_IsValidRootForHeight_WoC(t *testing.T) {
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

func TestWalletServices_IsValidRootForHeight_WoC_ContextCancelled(t *testing.T) {
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

func TestWalletServices_IsValidRootForHeight_Bitails(t *testing.T) {
	const height = btTst.TestBlockHeight

	validRoot, _ := chainhash.NewHashFromHex(btTst.TestMerkleRootHex)
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
			name: "Bitails happy path (WoC down)",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithInternalFailure()
				header := btTst.FakeHeaderHexWithMerkleRoot(t, btTst.TestMerkleRootHex)
				f.Bitails().
					WillRespondWithBlockHeaderByHeight(
						http.StatusOK,
						height,
						header)
			},
			root: validRoot,
			want: want{ok: true},
		},
		{
			name: "mismatching root from Bitails",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithInternalFailure()

				header := btTst.FakeHeaderHexWithMerkleRoot(t, btTst.TestMerkleRootHex)
				f.Bitails().
					WillRespondWithBlockHeaderByHeight(
						http.StatusOK,
						height,
						header)
			},
			root: invalidRoot,
			want: want{ok: false},
		},
		{
			name: "height not found (404) on Bitails",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithInternalFailure()
				f.Bitails().
					WillRespondWithBlockHeaderByHeight(
						http.StatusNotFound,
						height,
						"not found")
			},
			root: validRoot,
			want: want{ok: false},
		},
		{
			name: "Bitails unreachable",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithInternalFailure()
				_ = f.Bitails().WillBeUnreachable()
			},
			root: validRoot,
			want: want{ok: false, expectErr: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := ts.GivenServices(t)
			tc.setup(fix)
			svc := fix.Services().WithDefaultConfig()

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

func TestWalletServices_IsValidRootForHeight_Bitails_ContextCancelled(t *testing.T) {
	// given:
	const height = btTst.TestBlockHeight
	root, _ := chainhash.NewHashFromHex(btTst.TestMerkleRootHex)

	fix := ts.GivenServices(t)
	fix.WhatsOnChain().WillRespondWithInternalFailure()

	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~.*?/block/header/height/` + strconv.Itoa(int(height)) + `/raw$`
	fix.Bitails().Transport().RegisterResponder(http.MethodGet, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := fix.Services().WithDefaultConfig()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, height)

	// then:
	require.True(t, errors.Is(err, context.Canceled))
	require.False(t, ok)

	fix.Bitails().Transport().Reset()
	ok, err = svc.IsValidRootForHeight(ctx, root, height)

	require.True(t, errors.Is(err, context.Canceled))
	require.False(t, ok)
	require.Equal(t, 0, fix.Bitails().Transport().GetTotalCallCount())
}

const (
	bhsHeight = uint32(54321)
	rootHex   = btTst.TestMerkleRootHex
)

func TestWalletServices_IsValidRootForHeight_BHS(t *testing.T) {
	validRoot, _ := chainhash.NewHashFromHex(rootHex)
	invalidRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()

	type want struct {
		ok bool
	}

	tests := []struct {
		name  string
		setup func(ts.ServicesFixture)
		root  *chainhash.Hash
		want  want
	}{
		{
			name: "happy path via BHS",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				_ = f.Bitails().WillBeUnreachable()

				f.BHS().OnMerkleRootVerifyResponse(bhsHeight, rootHex, "CONFIRMED")
				f.BHS().IsUpAndRunning()
			},
			root: validRoot,
			want: want{ok: true},
		},
		{
			name: "mismatching root - BHS rejects",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				hdr := btTst.FakeHeaderHexWithMerkleRoot(t, rootHex)
				f.Bitails().WillRespondWithBlockHeaderByHeight(http.StatusOK, bhsHeight, hdr)

				f.BHS().OnMerkleRootVerifyResponse(bhsHeight, rootHex, "REJECTED")
				f.BHS().IsUpAndRunning()
			},
			root: invalidRoot,
			want: want{ok: false},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := ts.GivenServices(t)
			tc.setup(given)
			svc := given.Services().WithDefaultConfig()

			// when:
			ok, err := svc.IsValidRootForHeight(t.Context(), tc.root, bhsHeight)

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.want.ok, ok)
		})
	}
}

func TestWalletServices_IsValidRootForHeight_BHS_Unreachable(t *testing.T) {
	// given:
	given := ts.GivenServices(t)
	_ = given.WhatsOnChain().WillBeUnreachable()
	_ = given.Bitails().WillBeUnreachable()
	target := given.BHS().WillBeUnreachable()

	svc := given.Services().WithDefaultConfig()
	root, err := chainhash.NewHashFromHex(rootHex)
	require.NoError(t, err)

	// when:
	ok, err := svc.IsValidRootForHeight(t.Context(), root, bhsHeight)

	// then:
	require.ErrorIs(t, err, target)
	require.False(t, ok)
}

func TestWalletServices_IsValidRootForHeight_BHS_ContextCancelled_DuringCall(t *testing.T) {
	root := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
	var bhsHeight uint32 = testabilities.TestBlockHeight

	// given:
	given := ts.GivenServices(t)
	_ = given.WhatsOnChain().WillBeUnreachable()
	_ = given.Bitails().WillBeUnreachable()

	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~.*/api/v1/chain/merkleroot/verify$`

	given.BHS().Transport().RegisterResponder(http.MethodPost, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := given.Services().WithDefaultConfig()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, bhsHeight)

	// then:
	require.True(t, errors.Is(err, context.Canceled))
	require.False(t, ok)
}

func TestWalletServices_IsValidRootForHeight_BHS_ContextAlreadyCancelled(t *testing.T) {
	root := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
	var bhsHeight uint32 = testabilities.TestBlockHeight

	// given:
	given := ts.GivenServices(t)
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(context.Canceled)

	svc := given.Services().WithDefaultConfig()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, bhsHeight)

	// then:
	require.True(t, errors.Is(err, context.Canceled))
	require.False(t, ok)
	require.Equal(t, 0, given.BHS().Transport().GetTotalCallCount())
}
