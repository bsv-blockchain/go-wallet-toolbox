package bitails_test

import (
	"net/http"
	"testing"

	bt "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/stretchr/testify/require"
)

func TestBitails_GetHeight(t *testing.T) {
	const good = uint32(123_456)

	cases := []struct {
		name        string
		status      int
		blocks      uint32
		expectErr   bool
		expectValue uint32
	}{
		{"happy path", http.StatusOK, good, false, good},
		{"non-200", http.StatusBadGateway, 0, true, 0},
		{"zero height", http.StatusOK, 0, true, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fix := bt.Given(t)
			fix.Bitails().WillReturnNetworkInfo(tc.status, tc.blocks)

			got, err := fix.NewBitailsService().GetHeight(t.Context())

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectValue, got)
			}
		})
	}
}
