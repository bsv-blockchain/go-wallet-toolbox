package wallet_test

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestGetHeightOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.GetHeightResult, error) {
			return w.GetHeight(ctx, nil, originator)
		},
	)
}

func TestWallet_GetHeight(t *testing.T) {
	// given:
	given, _, cleanup := testabilities.New(t)

	defer cleanup()

	w := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

	validOriginator := "example.com"

	// when:
	result, err := w.GetHeight(t.Context(), struct{}{}, validOriginator)

	// then:
	if err != nil && strings.Contains(err.Error(), "failed to get current height") {
		t.Skipf("Skipping due to external service error: %v", err)
	}
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Positive(t, result.Height)
	t.Logf("Successfully got height: %d", result.Height)
}

func TestWallet_GetHeight_ValidOriginators(t *testing.T) {
	tests := map[string]string{
		"empty":         "",
		"simple domain": "example.com",
		"subdomain":     "api.example.com",
		"short":         "a.b",
	}

	for name, originator := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			w := given.Wallet().WithSQLiteStorage().WithServices().ForUser(testusers.Alice)

			// when:
			result, err := w.GetHeight(t.Context(), struct{}{}, originator)

			// then:
			if err != nil && strings.Contains(err.Error(), "failed to get current height") {
				t.Skipf("Skipping due to external service error: %v", err)
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Positive(t, result.Height)
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
	result, err := wallet.GetHeight(t.Context(), nil, "test-originator")

	// Then
	require.Error(t, err)
	require.Contains(t, err.Error(), "services are not configured for this wallet")
	require.Nil(t, result)
}
