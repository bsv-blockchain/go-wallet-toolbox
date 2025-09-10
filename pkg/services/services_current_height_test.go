package services_test

import (
	"net/http"
	"testing"

	ts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/stretchr/testify/require"
)

func TestWalletServices_CurrentHeight(t *testing.T) {
	const (
		wocTip = uint32(901475)
		bitTip = uint32(54321)
		bhsTip = uint32(777777)
	)

	tests := []struct {
		name        string
		setup       func(ts.ServicesFixture)
		expectValue uint32
	}{
		{
			name: "WhatsOnChain succeeds (primary)",
			setup: func(f ts.ServicesFixture) {
				f.WhatsOnChain().WillRespondWithChainInfo(http.StatusOK, wocTip)
			},
			expectValue: wocTip,
		},
		{
			name: "WoC unreachable → Bitails succeeds (first fallback)",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				f.Bitails().WillReturnNetworkInfo(http.StatusOK, bitTip)
			},
			expectValue: bitTip,
		},
		{
			name: "WoC & Bitails fail → BHS succeeds (second fallback)",
			setup: func(f ts.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
				f.Bitails().WillReturnNetworkInfo(http.StatusBadGateway, 0)

				f.BHS().OnLongestTipBlockHeaderResponseWith(ts.WithLongestChainTipHeight(uint(bhsTip)))
				f.BHS().IsUpAndRunning()
			},
			expectValue: bhsTip,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fix := ts.GivenServices(t)
			tc.setup(fix)

			svc := fix.Services().New()

			// when:
			got, err := svc.CurrentHeight(t.Context())

			// then:
			require.NoError(t, err)
			require.Equal(t, tc.expectValue, got)
		})
	}
}

func TestWalletServices_CurrentHeight_ErrorCases(t *testing.T) {

	tests := []struct {
		name        string
		setup       func(ts.ServicesFixture)
		expectValue uint32
	}{
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

			svc := fix.Services().New()

			// when:
			got, err := svc.CurrentHeight(t.Context())

			// then:
			require.Error(t, err)
			require.Equal(t, tc.expectValue, got)
		})
	}
}
