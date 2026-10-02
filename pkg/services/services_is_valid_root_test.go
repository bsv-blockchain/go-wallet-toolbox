package services_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/cachestore"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
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
		setup func(testservices.ServicesFixture)
		root  *chainhash.Hash
		want  want
	}{
		{
			name: "happy path",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK,
						height, wocTst.TestMerkleRootHex)
			},
			root: validRoot,
			want: want{ok: true},
		},
		{
			name: "mismatching root",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusOK,
						height, wocTst.TestMerkleRootHex)
			},
			root: invalidRoot,
			want: want{ok: false},
		},
		{
			name: "height not found (404)",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithBlockHeaderByHeight(http.StatusNotFound,
						height, "not found")
			},
			root: validRoot,
			want: want{ok: false},
		},
		{
			name: "provider unreachable",
			setup: func(f testservices.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
			},
			root: validRoot,
			want: want{ok: false, expectErr: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fixture := testservices.GivenServices(t)
			tc.setup(fixture)
			svc := fixture.Services().New()

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
	fixture := testservices.GivenServices(t)
	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~/block/height/` + strconv.Itoa(int(height))
	fixture.WhatsOnChain().Transport().RegisterResponder(http.MethodGet, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})
	svc := fixture.Services().New()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, height)

	// then:
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ok)
}

func TestWalletServices_IsValidRootForHeight_Bitails(t *testing.T) {
	const height = testabilities.TestBlockHeight

	validRoot, _ := chainhash.NewHashFromHex(testabilities.TestMerkleRootHex)
	invalidRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()

	type want struct {
		ok        bool
		expectErr bool
	}

	cases := []struct {
		name  string
		setup func(testservices.ServicesFixture)
		root  *chainhash.Hash
		want  want
	}{
		{
			name: "Bitails happy path (WoC down)",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithInternalFailure()
				header := testabilities.FakeHeaderHexWithMerkleRoot(t, testabilities.TestMerkleRootHex)
				f.Bitails().
					WillRespondWithBlockHeaderByHeight(
						http.StatusOK,
						height,
						header,
					)
			},
			root: validRoot,
			want: want{ok: true},
		},
		{
			name: "mismatching root from Bitails",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithInternalFailure()

				header := testabilities.FakeHeaderHexWithMerkleRoot(t, testabilities.TestMerkleRootHex)
				f.Bitails().
					WillRespondWithBlockHeaderByHeight(
						http.StatusOK,
						height,
						header,
					)
			},
			root: invalidRoot,
			want: want{ok: false},
		},
		{
			name: "height not found (404) on Bitails",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithInternalFailure()
				f.Bitails().
					WillRespondWithBlockHeaderByHeight(
						http.StatusNotFound,
						height,
						"not found",
					)
			},
			root: validRoot,
			want: want{ok: false},
		},
		{
			name: "Bitails unreachable",
			setup: func(f testservices.ServicesFixture) {
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
			fix := testservices.GivenServices(t)
			tc.setup(fix)
			svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

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
	const height = testabilities.TestBlockHeight
	root, _ := chainhash.NewHashFromHex(testabilities.TestMerkleRootHex)

	fix := testservices.GivenServices(t)
	fix.WhatsOnChain().WillRespondWithInternalFailure()

	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~.*?/block/header/height/` + strconv.Itoa(int(height)) + `/raw$`
	fix.Bitails().Transport().RegisterResponder(http.MethodGet, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := fix.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, height)

	// then:
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ok)

	fix.Bitails().Transport().Reset()
	ok, err = svc.IsValidRootForHeight(ctx, root, height)

	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ok)
	require.Equal(t, 0, fix.Bitails().Transport().GetTotalCallCount())
}

const (
	bhsHeight = uint32(54321)
	rootHex   = testabilities.TestMerkleRootHex
)

func TestWalletServices_IsValidRootForHeight_BHS(t *testing.T) {
	validRoot, _ := chainhash.NewHashFromHex(rootHex)
	invalidRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()

	type want struct {
		ok bool
	}

	tests := []struct {
		name  string
		setup func(testservices.ServicesFixture)
		root  *chainhash.Hash
		want  want
	}{
		{
			name: "happy path via BHS",
			setup: func(f testservices.ServicesFixture) {
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
			setup: func(f testservices.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				hdr := testabilities.FakeHeaderHexWithMerkleRoot(t, rootHex)
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
			given := testservices.GivenServices(t)
			tc.setup(given)
			svc := given.Services().Config(testservices.WithEnabledBitails(true), testservices.WithEnabledBHS(true)).New()

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
	given := testservices.GivenServices(t)
	_ = given.WhatsOnChain().WillBeUnreachable()
	_ = given.Bitails().WillBeUnreachable()
	target := given.BHS().WillBeUnreachable()
	_ = given.Chaintracks().WillFail()

	svc := given.Services().Config(
		testservices.WithEnabledBitails(true),
		testservices.WithEnabledBHS(true),
		testservices.WithEnabledChaintracks(true),
	).New()
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
	given := testservices.GivenServices(t)
	_ = given.WhatsOnChain().WillBeUnreachable()
	_ = given.Bitails().WillBeUnreachable()

	ctx, cancel := context.WithCancelCause(t.Context())
	pat := `=~.*/api/v1/chain/merkleroot/verify$`

	given.BHS().Transport().RegisterResponder(http.MethodPost, pat,
		func(_ *http.Request) (*http.Response, error) {
			cancel(context.Canceled)
			return nil, context.Canceled
		})

	svc := given.Services().Config(testservices.WithEnabledBitails(true), testservices.WithEnabledBHS(true)).New()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, bhsHeight)

	// then:
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ok)
}

func TestWalletServices_IsValidRootForHeight_BHS_ContextAlreadyCancelled(t *testing.T) {
	root := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
	var bhsHeight uint32 = testabilities.TestBlockHeight

	// given:
	given := testservices.GivenServices(t)
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(context.Canceled)

	svc := given.Services().Config(testservices.WithEnabledBitails(true)).New()

	// when:
	ok, err := svc.IsValidRootForHeight(ctx, root, bhsHeight)

	// then:
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ok)
	require.Equal(t, 0, given.BHS().Transport().GetTotalCallCount())
}

func TestWalletServices_IsValidRootForHeight_CachesOnlyPositiveAnswers(t *testing.T) {
	validRoot := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
	otherRoot := func() *chainhash.Hash { h := *validRoot; h[0] ^= 0xff; return &h }()
	const height uint32 = testabilities.TestBlockHeight

	redisStore := func(t *testing.T) cachestore.Provider {
		return givenCacheStore(t, defs.Cache{Type: defs.CacheRedis, Redis: defs.CacheRedisConfig{URL: "redis://" + miniredis.RunT(t).Addr()}})
	}

	cases := map[string]struct {
		store     func(t *testing.T) cachestore.Provider // nil = WalletServices default (memory)
		root      *chainhash.Hash
		wantOK    bool
		wantCalls int
	}{
		"default memory: valid root is asked once": {
			root:      validRoot,
			wantOK:    true,
			wantCalls: 1,
		},
		"default memory: invalid root is asked every time": {
			root:      otherRoot,
			wantOK:    false,
			wantCalls: 2,
		},
		"none: asks every time": {
			store:     func(t *testing.T) cachestore.Provider { return givenCacheStore(t, defs.Cache{Type: defs.CacheNone}) },
			root:      validRoot,
			wantOK:    true,
			wantCalls: 2,
		},
		"redis: valid root is asked once": {
			store:     redisStore,
			root:      validRoot,
			wantOK:    true,
			wantCalls: 1,
		},
		"redis: invalid root is asked every time": {
			store:     redisStore,
			root:      otherRoot,
			wantOK:    false,
			wantCalls: 2,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given:
			calls := 0
			given := givenCountingRootProvider(t, validRoot, &calls)
			if tc.store != nil {
				given.Opts(services.WithCacheStore(tc.store(t)))
			}
			svc := given.New()

			// when:
			for range 2 {
				ok, err := svc.IsValidRootForHeight(t.Context(), tc.root, height)

				// then:
				require.NoError(t, err)
				require.Equal(t, tc.wantOK, ok)
			}
			require.Equal(t, tc.wantCalls, calls)
		})
	}
}

func TestWalletServices_IsValidRootForHeight_RedisCacheIsSharedBetweenInstances(t *testing.T) {
	validRoot := testabilities.HashFromHex(t, testabilities.TestMerkleRootHex)
	const height uint32 = testabilities.TestBlockHeight

	// given:
	cfg := defs.Cache{Type: defs.CacheRedis, Redis: defs.CacheRedisConfig{URL: "redis://" + miniredis.RunT(t).Addr()}}
	newInstance := func(calls *int) *services.WalletServices {
		// a separate provider per instance, as two processes would have
		return givenCountingRootProvider(t, validRoot, calls).
			Opts(services.WithCacheStore(givenCacheStore(t, cfg))).
			New()
	}
	var firstCalls, secondCalls int
	first, second := newInstance(&firstCalls), newInstance(&secondCalls)

	// when:
	_, err := first.IsValidRootForHeight(t.Context(), validRoot, height)
	require.NoError(t, err)
	ok, err := second.IsValidRootForHeight(t.Context(), validRoot, height)

	// then:
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, firstCalls)
	require.Equal(t, 0, secondCalls)
}

func givenCacheStore(t *testing.T, cfg defs.Cache) cachestore.Provider {
	store, err := cachestore.New(logging.NewTestLogger(t), cfg)
	require.NoError(t, err)
	return store
}

// givenCountingRootProvider replaces every IsValidRootForHeight provider with one that
// counts its calls and accepts only validRoot.
func givenCountingRootProvider(t *testing.T, validRoot *chainhash.Hash, calls *int) testservices.WalletServicesFixture {
	provider := func(_ context.Context, root *chainhash.Hash, _ uint32) (bool, error) {
		*calls++
		return root.IsEqual(validRoot), nil
	}

	return testservices.GivenServices(t).Services().
		Opts(services.WithIsValidRootForHeightMethodsModifier(
			func([]services.Named[services.IsValidRootForHeightFunc]) []services.Named[services.IsValidRootForHeightFunc] {
				return []services.Named[services.IsValidRootForHeightFunc]{{Name: "counting", Item: provider}}
			}))
}
