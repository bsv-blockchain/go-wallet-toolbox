package whatsonchain_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	tst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
)

func TestWhatsOnChain_GetHeight(t *testing.T) {
	// given:
	const good = uint32(765_432)

	given := tst.Given(t)
	given.WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, good)

	// when:
	got, err := given.NewWoCService().CurrentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, good, got)
}

func TestWhatsOnChain_GetHeight_ErrorCases(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{"non-200", http.StatusInternalServerError},
		{"zero height", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := tst.Given(t)
			given.WhatsOnChain().WillRespondWithChainInfo(tc.status, 0)

			// when:
			_, err := given.NewWoCService().CurrentHeight(t.Context())

			// then:
			require.Error(t, err)
		})
	}
}
