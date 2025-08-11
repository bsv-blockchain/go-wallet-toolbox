// pkg/services/internal/bhs/nlocktime_is_final_test.go
package bhs_test

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	bhsTst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs/testabilities"
	"github.com/stretchr/testify/require"
)

func TestNLockTimeIsFinal_LockHeightComparisons(t *testing.T) {
	// given:
	const chainHeight = uint(700_000)

	given := bhsTst.Given(t)
	bhsFx := given.BHS()
	bhsFx.OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(chainHeight))
	bhsFx.IsUpAndRunning()

	svc := given.NewBHSService()

	cases := []struct {
		name     string
		locktime uint32
		want     bool
	}{
		{"nLockTime < height -> final", uint32(chainHeight - 1), true},
		{"nLockTime == height -> NOT final", uint32(chainHeight), true},
		{"nLockTime > height -> NOT final", uint32(chainHeight + 1), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when:
			got, err := svc.NLockTimeIsFinal(t.Context(), tc.locktime)

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestNLockTimeIsFinal_TimestampComparisons(t *testing.T) {
	// given:
	given := bhsTst.Given(t)
	svc := given.NewBHSService()

	now := uint32(time.Now().Unix())
	const BLOCK_LIMIT = 500000000

	cases := []struct {
		name     string
		locktime uint32
		want     bool
	}{
		{"timestamp: past -> final", now - 60, true},
		{"timestamp: future -> NOT final", now + 60, false},
		{"timestamp boundary -> treat as time", uint32(BLOCK_LIMIT), now > BLOCK_LIMIT},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when:
			got, err := svc.NLockTimeIsFinal(t.Context(), tc.locktime)

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestNLockTimeIsFinal_Tx_AllInputsFinalShortCircuit(t *testing.T) {
	// given:
	given := bhsTst.Given(t)
	svc := given.NewBHSService()

	tx := testutils.NewTestTransactionWithLocktime(t, 700_000, testutils.MaxSeq, testutils.MaxSeq, testutils.MaxSeq)

	// when:
	got, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.NoError(t, err)
	require.True(t, got)
}

func TestNLockTimeIsFinal_Tx_HeightPath(t *testing.T) {
	// given:
	const chainHeight = uint(800_000)

	given := bhsTst.Given(t)
	bhsFx := given.BHS()
	bhsFx.OnLongestTipBlockHeaderResponseWith(testservices.WithLongestChainTipHeight(chainHeight))
	bhsFx.IsUpAndRunning()

	svc := given.NewBHSService()

	tx := testutils.NewTestTransactionWithLocktime(t, uint32(chainHeight-1), 0)

	// when:
	got, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.NoError(t, err)
	require.True(t, got)
}

func TestNLockTimeIsFinal_PropagatesCurrentHeightErrors(t *testing.T) {
	// given:
	given := bhsTst.Given(t)
	given.BHS().WillRespondWithInternalFailure()

	svc := given.NewBHSService()

	tx := testutils.NewTestTransactionWithLocktime(t, 600_000, 0)

	// when:
	_, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.Error(t, err)
}
