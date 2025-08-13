package wallet_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func TestWallet_GetHeight(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	w := given.Wallet().WithSQLiteStorage().WithServices().ForUser(testusers.Alice)
	validOriginator := "example.com"

	// when:
	result, err := w.GetHeight(context.Background(), struct{}{}, validOriginator)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Greater(t, result.Height, uint32(0))
	t.Logf("Successfully got height: %d", result.Height)
}

func TestWallet_GetHeight_InvalidOriginator(t *testing.T) {
	tests := []struct {
		name       string
		originator string
	}{
		{"too long", strings.Repeat("a", 251)},
		{"empty part", "invalid..originator"},
		{"part too long", "part1." + strings.Repeat("a", 64) + ".part3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			w := given.Wallet().WithSQLiteStorage().WithServices().ForUser(testusers.Alice)

			// when:
			result, err := w.GetHeight(context.Background(), struct{}{}, tc.originator)

			// then:
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid originator")
			require.Nil(t, result)
		})
	}
}

func TestWallet_GetHeight_ValidOriginators(t *testing.T) {
	tests := []struct {
		name       string
		originator string
	}{
		{"empty", ""},
		{"simple domain", "example.com"},
		{"subdomain", "api.example.com"},
		{"short", "a.b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			w := given.Wallet().WithSQLiteStorage().WithServices().ForUser(testusers.Alice)

			// when:
			result, err := w.GetHeight(context.Background(), struct{}{}, tc.originator)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Greater(t, result.Height, uint32(0))
		})
	}
}

func TestWallet_GetHeight_WalletWithoutServices(t *testing.T) {
	// Given
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	wallet := given.Wallet().
		WithSQLiteStorage().
		// NOT calling WithServices()
		ForUser(testusers.Alice)

	// When
	result, err := wallet.GetHeight(context.Background(), nil, "test-originator")

	// Then
	require.Error(t, err)
	require.Contains(t, err.Error(), "services are not configured for this wallet")
	require.Nil(t, result)
}
