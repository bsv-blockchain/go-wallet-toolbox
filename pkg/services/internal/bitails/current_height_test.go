package bitails_test

import (
	"net/http"
	"testing"

	bt "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails/testabilities"
	"github.com/stretchr/testify/require"
)

func TestBitails_GetHeight(t *testing.T) {
	// given:
	const good = uint32(123_456)

	given := bt.Given(t)
	given.Bitails().WillReturnNetworkInfo(http.StatusOK, good)

	// when:
	got, err := given.NewBitailsService().CurrentHeight(t.Context())

	// then:
	require.NoError(t, err)
	require.Equal(t, good, got)
}

func TestBitails_GetHeight_ErrorCases(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		blocks      uint32
		expectValue uint32
	}{
		{"non-200", http.StatusBadGateway, 0, 0},
		{"zero height", http.StatusOK, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given := bt.Given(t)
			given.Bitails().WillReturnNetworkInfo(tc.status, tc.blocks)

			// when:
			_, err := given.NewBitailsService().CurrentHeight(t.Context())

			// then:
			require.Error(t, err)
		})
	}
}
