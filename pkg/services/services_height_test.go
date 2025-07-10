package services_test

import (
	"net/http"
	"testing"

	ts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/stretchr/testify/require"
)

func TestWalletServices_Height(t *testing.T) {
	const (
		wocTip = uint32(901475)
		bitTip = uint32(54321)
		bhsTip = uint32(777777)
	)

	tests := []struct {
		name        string
		setup       func(ts.ServicesFixture)
		expectValue int64
	}{
		{
			name: "WhatsOnChain succeeds (primary)",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().
					WillRespondWithChainInfo(http.StatusOK, wocTip)
			},
			expectValue: int64(wocTip),
		},
		{
			name: "WoC unreachable → Bitails succeeds (first fallback)",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				f.Bitails().
					WillReturnNetworkInfo(http.StatusOK, bitTip)
			},
			expectValue: int64(bitTip),
		},
		{
			name: "WoC & Bitails fail → BHS succeeds (second fallback)",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				f.Bitails().WillReturnNetworkInfo(http.StatusBadGateway, 0)

				// mock BHS tip height
				f.BHS().
					OnLongestTipBlockHeaderResponseWith(
						ts.WithLongestChainTipHeight(uint(bhsTip)))
				f.BHS().IsUpAndRunning()
			},
			expectValue: int64(bhsTip),
		},
		{
			name: "all providers fail → height is 0",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				f.Bitails().WillReturnNetworkInfo(http.StatusBadGateway, 0)
				_ = f.BHS().WillBeUnreachable()
			},
			expectValue: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := ts.GivenServices(t)
			tc.setup(fix)

			svc := fix.Services().WithDefaultConfig()

			// when:
			got := svc.Height()

			// then:
			require.Equal(t, tc.expectValue, got)
		})
	}
}
