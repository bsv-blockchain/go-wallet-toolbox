package wallet_test

import (
	"context"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

type testCase struct {
	ctx        context.Context
	args       sdk.GetHeaderArgs
	originator string
}

func TestGetHeaderForHeightOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.GetHeaderResult, error) {
			args := sdk.GetHeaderArgs{
				Height: 100,
			}
			return w.GetHeaderForHeight(ctx, args, originator)
		},
	)
}

func TestWallet_GetHeaderForHeight_Validation(t *testing.T) {
	// Given
	given, _, cleanup := testabilities.New(t)

	defer cleanup()

	wallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

	tests := map[string]testCase{
		"valid request": {
			ctx: context.Background(),
			args: sdk.GetHeaderArgs{
				Height: 100,
			},
			originator: fixtures.DefaultOriginator,
		},
		"zero height (should be handled by validation)": {
			ctx: context.Background(),
			args: sdk.GetHeaderArgs{
				Height: 0,
			},
			originator: fixtures.DefaultOriginator,
		},
		"height 1 (genesis + 1)": {
			ctx: context.Background(),
			args: sdk.GetHeaderArgs{
				Height: 1,
			},
			originator: fixtures.DefaultOriginator,
		},
		"height 100": {
			ctx: context.Background(),
			args: sdk.GetHeaderArgs{
				Height: 100,
			},
			originator: fixtures.DefaultOriginator,
		},
		"height 100000": {
			ctx: context.Background(),
			args: sdk.GetHeaderArgs{
				Height: 100000,
			},
			originator: fixtures.DefaultOriginator,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// When
			result, err := wallet.GetHeaderForHeight(tc.ctx, tc.args, tc.originator)

			// Then
			require.NoError(t, err, "Unexpected error for height %d: %v", tc.args.Height, err)
			require.NotNil(t, result)
		})
	}
}

func TestWallet_GetHeaderForHeight_WalletWithoutServices(t *testing.T) {
	// Given
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	wallet := given.Wallet().
		WithSQLiteStorage().
		// NOT calling WithServices()
		ForUser(testusers.Alice)

	// When
	result, err := wallet.GetHeaderForHeight(
		context.Background(),
		sdk.GetHeaderArgs{Height: 100},
		fixtures.DefaultOriginator,
	)

	// Then
	require.Error(t, err)
	require.Contains(t, err.Error(), "wallet services not configured")
	require.Nil(t, result)
}

type serviceErrorTestCase struct {
	height        uint32
	description   string
	expectedError string
}

func TestWallet_GetHeaderForHeight_ServiceErrors(t *testing.T) {
	// Given
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	wallet := given.Wallet().
		WithSQLiteStorage().
		WithServices().
		ForUser(testusers.Alice)

	serviceErrorTests := map[string]serviceErrorTestCase{
		"service unavailable height": {
			height:        999999999,
			description:   "Very high height that services likely don't have",
			expectedError: "service unavailable",
		},
		"edge case height max uint32": {
			height:        4294967295,
			description:   "Maximum possible height",
			expectedError: "service unavailable",
		},
	}

	for name, tc := range serviceErrorTests {
		t.Run(name, func(t *testing.T) {
			// When
			result, err := wallet.GetHeaderForHeight(
				context.Background(),
				sdk.GetHeaderArgs{Height: tc.height},
				fixtures.DefaultOriginator,
			)

			// Then
			require.Errorf(t, err, "Expected error for height %d: %s", tc.height, tc.expectedError)
			require.Nil(t, result, "Expected nil result for height %d", tc.height)
		})
	}
}
