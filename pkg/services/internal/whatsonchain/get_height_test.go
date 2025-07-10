package whatsonchain_test

import (
	"net/http"
	"testing"

	tst "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain/testabilities"
	"github.com/stretchr/testify/require"
)

func TestWhatsOnChain_GetHeight(t *testing.T) {
	const good = uint32(765_432)

	cases := []struct {
		name        string
		status      int
		blocks      uint32
		expectErr   bool
		expectValue uint32
	}{
		{"happy path", http.StatusOK, good, false, good},
		{"non-200", http.StatusInternalServerError, 0, true, 0},
		{"zero height", http.StatusOK, 0, true, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fix := tst.Given(t)
			fix.WhatsOnChain().WillRespondWithChainInfo(tc.status, tc.blocks)

			got, err := fix.NewWoCService().GetHeight(t.Context())

			if tc.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectValue, got)
		})
	}
}
