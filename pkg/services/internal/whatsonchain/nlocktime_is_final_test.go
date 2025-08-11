package whatsonchain_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	tst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/stretchr/testify/require"
)

func TestWhatsOnChain_NLockTimeIsFinal_LockHeightComparisons(t *testing.T) {
	// given:
	const chainHeight = uint32(700_000)

	given := tst.Given(t)
	given.WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, chainHeight)

	svc := given.NewWoCService()

	cases := []struct {
		name     string
		locktime uint32
		want     bool
	}{
		{"nLockTime < height -> final", chainHeight - 1, true},
		{"nLockTime == height -> NOT final", chainHeight, true},
		{"nLockTime > height -> NOT final", chainHeight + 1, false},
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

func TestWhatsOnChain_NLockTimeIsFinal_TimestampComparisons(t *testing.T) {
	// given:
	given := tst.Given(t)
	svc := given.NewWoCService()

	now := uint32(time.Now().Unix())
	const BLOCK_LIMIT = 500000000

	cases := []struct {
		name     string
		locktime uint32
		want     bool
	}{
		{"timestamp: past -> final", now - 60, true},
		{"timestamp: future -> NOT final", now + 60, false},
		{"timestamp: boundary equals limit -> by time", BLOCK_LIMIT, BLOCK_LIMIT < uint32(time.Now().Unix())},
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

func TestWhatsOnChain_NLockTimeIsFinal_Tx_AllInputsFinalShortCircuit(t *testing.T) {
	// given:
	given := tst.Given(t)
	svc := given.NewWoCService()

	tx := testutils.NewTestTransactionWithLocktime(t, 700_000, testutils.MaxSeq, testutils.MaxSeq, testutils.MaxSeq)

	// when:
	got, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.NoError(t, err)
	require.True(t, got)
}

func TestWhatsOnChain_NLockTimeIsFinal_Tx_HeightPath(t *testing.T) {
	// given:
	const chainHeight = uint32(800_000)

	given := tst.Given(t)
	given.WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, chainHeight)

	svc := given.NewWoCService()

	tx := testutils.NewTestTransactionWithLocktime(t, chainHeight-1, 0)

	// when:
	got, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.NoError(t, err)
	require.True(t, got)
}

func TestWhatsOnChain_NLockTimeIsFinal_PropagatesCurrentHeightErrors(t *testing.T) {
	// given:
	given := tst.Given(t)
	given.WhatsOnChain().WillRespondWithChainInfo(http.StatusBadGateway, 0)

	svc := given.NewWoCService()

	tx := testutils.NewTestTransactionWithLocktime(t, 600_000, 0)

	// when:
	_, err := svc.NLockTimeIsFinal(t.Context(), tx)

	// then:
	require.Error(t, err)
}
