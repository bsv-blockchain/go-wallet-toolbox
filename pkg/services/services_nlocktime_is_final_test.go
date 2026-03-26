package services_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
)

func TestWalletServices_NLockTimeIsFinal_Primary_WoC(t *testing.T) {
	const wocTip = uint32(750_000)

	cases := []struct {
		name        string
		lockTime    uint32
		expectFinal bool
	}{
		{
			name:        "locktime < height -> final",
			lockTime:    wocTip - 1,
			expectFinal: true,
		},
		{
			name:        "locktime == height -> not final",
			lockTime:    wocTip,
			expectFinal: true,
		},
		{
			name:        "locktime > height -> not final",
			lockTime:    wocTip + 1,
			expectFinal: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := testservices.GivenServices(t)
			given.WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, wocTip)

			svc := given.Services().New()

			// when:
			got, err := svc.NLockTimeIsFinal(t.Context(), tc.lockTime)

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.expectFinal, got)
		})
	}
}

func TestWalletServices_NLockTimeIsFinal_Fallbacks(t *testing.T) {
	const (
		bitTip = uint32(543_210)
		bhsTip = uint32(777_777)
	)

	t.Run("WoC unreachable → Bitails succeeds", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		err := given.WhatsOnChain().WillBeUnreachable()
		require.Error(t, err)
		given.Bitails().WillReturnNetworkInfo(http.StatusOK, bitTip)

		svc := given.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		got, err := svc.NLockTimeIsFinal(t.Context(), bitTip-1)

		// then:
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("WoC & Bitails fail → BHS succeeds", func(t *testing.T) {
		// given:
		given := testservices.GivenServices(t)
		err := given.WhatsOnChain().WillBeUnreachable()
		require.Error(t, err)
		given.Bitails().WillReturnNetworkInfo(http.StatusBadGateway, 0)

		given.BHS().OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(uint(bhsTip)))
		given.BHS().IsUpAndRunning()

		svc := given.Services().Config(testservices.WithEnabledBitails(true), testservices.WithEnabledBHS(true)).New()

		// when:
		got, err := svc.NLockTimeIsFinal(t.Context(), bhsTip-1)

		// then:
		require.NoError(t, err)
		require.True(t, got)
	})
}

func TestWalletServices_NLockTimeIsFinal_AllProvidersFail(t *testing.T) {
	// given:
	given := testservices.GivenServices(t)

	err := given.WhatsOnChain().WillBeUnreachable()
	require.Error(t, err)
	given.Bitails().WillReturnNetworkInfo(http.StatusBadGateway, 0)
	err = given.BHS().WillBeUnreachable()
	require.Error(t, err)
	_ = given.Chaintracks().WillFail()

	svc := given.Services().Config(
		testservices.WithEnabledBitails(true),
		testservices.WithEnabledBHS(true),
		testservices.WithEnabledChaintracks(true),
	).New()

	// when:
	_, err = svc.NLockTimeIsFinal(t.Context(), uint32(400_000_000))

	// then:
	require.Error(t, err)
}

func TestWalletServices_NLockTimeIsFinal_TimestampPath(t *testing.T) {
	const blockLimit = uint32(500_000_000)

	type tc struct {
		name     string
		lockTime func(now uint32) uint32
		expect   func(now uint32) bool
	}

	cases := []tc{
		{
			name:     "time-based final when locktime < now",
			lockTime: func(now uint32) uint32 { return now - 60 },
			expect:   func(now uint32) bool { return true },
		},
		{
			name:     "time-based not final when locktime > now",
			lockTime: func(now uint32) uint32 { return now + 60 },
			expect:   func(now uint32) bool { return false },
		},
		{
			name:     "locktime == BLOCK_LIMIT uses timestamp path",
			lockTime: func(now uint32) uint32 { return blockLimit },
			expect:   func(now uint32) bool { return blockLimit < now },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// given:
			now := uint32(time.Now().Unix()) //nolint:gosec // unix timestamp fits in uint32 until year 2106

			given := testservices.GivenServices(t)
			svc := given.Services().Config(testservices.WithEnabledBitails(true)).New()

			lt := c.lockTime(now)
			want := c.expect(now)

			// when:
			got, err := svc.NLockTimeIsFinal(t.Context(), lt)

			// then:
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

func TestWalletServices_NLockTimeIsFinal_Tx_AllInputsMaxSequence_ShortCircuit(t *testing.T) {
	// given:
	svc := testservices.GivenServices(t).Services().New()

	tx := testutils.NewTestTransactionWithLocktime(t, 700_000, testutils.MaxSeq, testutils.MaxSeq, testutils.MaxSeq)

	// when:
	got, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.NoError(t, err)
	require.True(t, got)
}

func TestWalletServices_NLockTimeIsFinal_Tx_HeightPath(t *testing.T) {
	// given:
	const wocTip = uint32(800_000)

	given := testservices.GivenServices(t)
	given.WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, wocTip)

	svc := given.Services().New()
	tx := testutils.NewTestTransactionWithLocktime(t, wocTip-1, 0)

	// when:
	got, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.NoError(t, err)
	require.True(t, got)
}
