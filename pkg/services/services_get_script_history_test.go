package services_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
)

func TestWalletServices_GetScriptHashHistory(t *testing.T) {
	const testScriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	type want struct {
		expectErr    bool
		historyCount int
		name         string
	}

	cases := []struct {
		name  string
		setup func(testservices.ServicesFixture)
		hash  string
		want  want
	}{
		{
			name: "happy path with confirmed and unconfirmed transactions",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(2, 800000).
					WithUnconfirmedTransactions(1).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{historyCount: 3, name: whatsonchain.ServiceName},
		},
		{
			name: "empty history",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithEmptyHistory().
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{historyCount: 0, name: whatsonchain.ServiceName},
		},
		{
			name: "only confirmed transactions",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(3, 750000).
					WithUnconfirmedTransactions(0).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{historyCount: 3, name: whatsonchain.ServiceName},
		},
		{
			name: "only unconfirmed transactions",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(0, 0).
					WithUnconfirmedTransactions(2).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{historyCount: 2, name: whatsonchain.ServiceName},
		},
		{
			name: "service API error",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Script not found").
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{expectErr: true, name: whatsonchain.ServiceName},
		},
		{
			name: "service HTTP error",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Not found").
					WithConfirmedStatusCode(http.StatusNotFound).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{expectErr: true},
		},
		{
			name: "invalid script hash",
			setup: func(f testservices.ServicesFixture) {
				// No setup needed - validation happens before service calls
			},
			hash: "invalid-hash",
			want: want{expectErr: true},
		},
		{
			name: "empty script hash",
			setup: func(f testservices.ServicesFixture) {
				// No setup needed - validation happens before service calls
			},
			hash: "",
			want: want{expectErr: true},
		},
		{
			name: "provider unreachable",
			setup: func(f testservices.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
			},
			hash: testScriptHash,
			want: want{expectErr: true},
		},
		{
			name: "large history set",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(100, 800000).
					WithUnconfirmedTransactions(10).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{historyCount: 110, name: whatsonchain.ServiceName},
		},
		{
			name: "bitails happy path with confirmed and unconfirmed",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(2, 800000).
					WithUnconfirmedTransactions(1).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{historyCount: 3, name: bitails.ServiceName},
		},
		{
			name: "bitails empty history",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithEmptyHistory().
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{historyCount: 0, name: bitails.ServiceName},
		},
		{
			name: "bitails confirmed error",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("some error").
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{expectErr: true},
		},
		{
			name: "bitails HTTP 404 error",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("not found").
					WithConfirmedStatusCode(http.StatusNotFound).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{expectErr: true},
		},
		{
			name: "bitails unreachable",
			setup: func(f testservices.ServicesFixture) {
				_ = f.Bitails().WillBeUnreachable()
			},
			hash: testScriptHash,
			want: want{expectErr: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fixture := testservices.GivenServices(t)
			tc.setup(fixture)
			svc := fixture.Services().Config(testservices.WithEnabledBitails(true)).New()

			// when:
			result, err := svc.GetScriptHashHistory(t.Context(), tc.hash)

			// then:
			if tc.want.expectErr {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.hash, result.ScriptHash)
				require.Equal(t, tc.want.name, result.Name)
				require.Len(t, result.History, tc.want.historyCount)
			}
		})
	}
}

func TestWalletServices_GetScriptHashHistory_ContextCancelled(t *testing.T) {
	const testScriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	t.Run("WhatsOnChain cancels context", func(t *testing.T) {
		// given:
		fixture := testservices.GivenServices(t)
		ctx, cancel := context.WithCancelCause(t.Context())

		// Setup responder that cancels context when called
		confirmedPattern := `=~/script/` + testScriptHash + `/confirmed/history`
		fixture.WhatsOnChain().Transport().RegisterResponder(http.MethodGet, confirmedPattern,
			func(_ *http.Request) (*http.Response, error) {
				cancel(context.Canceled)
				return nil, context.Canceled
			})

		svc := fixture.Services().New()

		// when:
		result, err := svc.GetScriptHashHistory(ctx, testScriptHash)

		// then:
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, result)
	})

	t.Run("Bitails cancels context", func(t *testing.T) {
		// given:
		fixture := testservices.GivenServices(t)
		ctx, cancel := context.WithCancelCause(t.Context())

		bitailsPattern := `=~/scripthash/` + testScriptHash + `/history`
		fixture.Bitails().Transport().RegisterResponder(http.MethodGet, bitailsPattern,
			func(_ *http.Request) (*http.Response, error) {
				cancel(context.Canceled)
				return nil, context.Canceled
			})

		svc := fixture.Services().Config(testservices.WithEnabledBitails(true)).New()

		// when:
		result, err := svc.GetScriptHashHistory(ctx, testScriptHash)

		// then:
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, result)
	})
}

func TestWalletServices_GetScriptHashHistory_ServiceOrchestration(t *testing.T) {
	const testScriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	type want struct {
		expectErr     bool
		expectSuccess bool
		serviceName   string
	}

	cases := []struct {
		name  string
		setup func(testservices.ServicesFixture)
		want  want
	}{
		{
			name: "primary service succeeds",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(1, 800000).
					WithUnconfirmedTransactions(1).
					WillBeReturned()
			},
			want: want{expectSuccess: true, serviceName: whatsonchain.ServiceName},
		},
		{
			name: "service returns confirmed transactions error",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Service unavailable").
					WithConfirmedStatusCode(http.StatusOK).
					WillBeReturned()
			},
			want: want{expectErr: true},
		},
		{
			name: "service returns unconfirmed transactions error",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(1, 800000).
					WithUnconfirmedTransactionsError("Service unavailable").
					WithUnconfirmedStatusCode(http.StatusOK).
					WillBeReturned()
			},
			want: want{expectErr: true},
		},
		{
			name: "bitails primary service succeeds",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(1, 800000).
					WithUnconfirmedTransactions(1).
					WillBeReturned()
			},
			want: want{expectSuccess: true, serviceName: bitails.ServiceName},
		},
		{
			name: "bitails service returns confirmed transactions error",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Service unavailable").
					WithConfirmedStatusCode(http.StatusOK).
					WillBeReturned()
			},
			want: want{expectErr: true},
		},
		{
			name: "bitails service returns unconfirmed transactions error",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(1, 800000).
					WithUnconfirmedTransactionsError("Service unavailable").
					WithUnconfirmedStatusCode(http.StatusOK).
					WillBeReturned()
			},
			want: want{expectErr: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fixture := testservices.GivenServices(t)
			tc.setup(fixture)
			svc := fixture.Services().Config(testservices.WithEnabledBitails(true)).New()

			// when:
			result, err := svc.GetScriptHashHistory(t.Context(), testScriptHash)

			// then:
			if tc.want.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failed to get script history")
				require.Nil(t, result)
			} else if tc.want.expectSuccess {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.want.serviceName, result.Name)
				require.Equal(t, testScriptHash, result.ScriptHash)
			}
		})
	}
}

func TestWalletServices_GetScriptHashHistory_ErrorHandling(t *testing.T) {
	const testScriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	type want struct {
		errorContains string
	}

	cases := []struct {
		name  string
		setup func(testservices.ServicesFixture)
		hash  string
		want  want
	}{
		{
			name: "validation error wrapped properly",
			setup: func(f testservices.ServicesFixture) {
			},
			hash: "invalid",
			want: want{errorContains: "failed to get script history"},
		},
		{
			name: "API error wrapped properly",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Rate limit exceeded").
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{errorContains: "failed to get script history"},
		},
		{
			name: "HTTP error wrapped properly",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Internal server error").
					WithConfirmedStatusCode(http.StatusInternalServerError).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{errorContains: "failed to get script history"},
		},
		{
			name: "service unreachable error wrapped properly",
			setup: func(f testservices.ServicesFixture) {
				_ = f.WhatsOnChain().WillBeUnreachable()
			},
			hash: testScriptHash,
			want: want{errorContains: "failed to get script history"},
		},
		{
			name: "bitails API error wrapped properly",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Rate limit").
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{errorContains: "failed to get script history"},
		},
		{
			name: "bitails HTTP error wrapped properly",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactionsError("Internal server error").
					WithConfirmedStatusCode(http.StatusInternalServerError).
					WillBeReturned()
			},
			hash: testScriptHash,
			want: want{errorContains: "failed to get script history"},
		},
		{
			name: "bitails service unreachable error wrapped properly",
			setup: func(f testservices.ServicesFixture) {
				_ = f.Bitails().WillBeUnreachable()
			},
			hash: testScriptHash,
			want: want{errorContains: "failed to get script history"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fixture := testservices.GivenServices(t)
			tc.setup(fixture)
			svc := fixture.Services().Config(testservices.WithEnabledBitails(true)).New()

			// when:
			result, err := svc.GetScriptHashHistory(t.Context(), tc.hash)

			// then:
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want.errorContains)
			require.Nil(t, result)
		})
	}
}

func TestWalletServices_GetScriptHashHistory_ResultFormatting(t *testing.T) {
	const testScriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"

	type want struct {
		serviceName      string
		confirmedCount   int
		unconfirmedCount int
	}

	cases := []struct {
		name  string
		setup func(testservices.ServicesFixture)
		want  want
	}{
		{
			name: "result contains proper service name and counts",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(3, 800000).
					WithUnconfirmedTransactions(2).
					WillBeReturned()
			},
			want: want{
				serviceName:      whatsonchain.ServiceName,
				confirmedCount:   3,
				unconfirmedCount: 2,
			},
		},
		{
			name: "result with only confirmed transactions",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(5, 750000).
					WithUnconfirmedTransactions(0).
					WillBeReturned()
			},
			want: want{
				serviceName:      whatsonchain.ServiceName,
				confirmedCount:   5,
				unconfirmedCount: 0,
			},
		},
		{
			name: "result with only unconfirmed transactions",
			setup: func(f testservices.ServicesFixture) {
				f.WhatsOnChain().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(0, 0).
					WithUnconfirmedTransactions(3).
					WillBeReturned()
			},
			want: want{
				serviceName:      whatsonchain.ServiceName,
				confirmedCount:   0,
				unconfirmedCount: 3,
			},
		},
		{
			name: "bitails result with only unconfirmed transactions",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(0, 0).
					WithUnconfirmedTransactions(3).
					WillBeReturned()
			},
			want: want{
				serviceName:      bitails.ServiceName,
				confirmedCount:   0,
				unconfirmedCount: 3,
			},
		},
		{
			name: "bitails result with confirmed and unconfirmed",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(3, 800000).
					WithUnconfirmedTransactions(2).
					WillBeReturned()
			},
			want: want{
				serviceName:      bitails.ServiceName,
				confirmedCount:   3,
				unconfirmedCount: 2,
			},
		},
		{
			name: "bitails result contains proper service name and counts",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(3, 800000).
					WithUnconfirmedTransactions(2).
					WillBeReturned()
			},
			want: want{
				serviceName:      bitails.ServiceName,
				confirmedCount:   3,
				unconfirmedCount: 2,
			},
		},
		{
			name: "bitails result with only confirmed transactions",
			setup: func(f testservices.ServicesFixture) {
				f.Bitails().
					ScriptHistoryData().
					WithScriptHash(testScriptHash).
					WithConfirmedTransactions(5, 750000).
					WithUnconfirmedTransactions(0).
					WillBeReturned()
			},
			want: want{
				serviceName:      bitails.ServiceName,
				confirmedCount:   5,
				unconfirmedCount: 0,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given:
			fixture := testservices.GivenServices(t)
			tc.setup(fixture)
			svc := fixture.Services().Config(testservices.WithEnabledBitails(true)).New()

			// when:
			result, err := svc.GetScriptHashHistory(t.Context(), testScriptHash)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.want.serviceName, result.Name)
			require.Equal(t, testScriptHash, result.ScriptHash)

			confirmedCount := 0
			unconfirmedCount := 0
			for _, item := range result.History {
				if item.Height != nil {
					confirmedCount++
				} else {
					unconfirmedCount++
				}
			}

			require.Equal(t, tc.want.confirmedCount, confirmedCount)
			require.Equal(t, tc.want.unconfirmedCount, unconfirmedCount)
		})
	}
}

func TestWalletServices_GetScriptHashHistory_ConcurrentAccess(t *testing.T) {
	const testScriptHash = "0374d9ee2df8e5d7c5fd8359f33456996f2a1a9c76d9c783d2f8d5ee05ba5832"
	const numConcurrent = 10

	t.Run("WhatsOnChain concurrent access", func(t *testing.T) {
		fixture := testservices.GivenServices(t)
		fixture.WhatsOnChain().
			ScriptHistoryData().
			WithScriptHash(testScriptHash).
			WithConfirmedTransactions(1, 800000).
			WithUnconfirmedTransactions(1).
			WillBeReturned()

		svc := fixture.Services().New()

		results := make(chan interface{}, numConcurrent)
		errors := make(chan error, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			go func() {
				result, err := svc.GetScriptHashHistory(t.Context(), testScriptHash)
				results <- result
				errors <- err
			}()
		}

		for i := 0; i < numConcurrent; i++ {
			result := <-results
			err := <-errors

			require.NoError(t, err)
			require.NotNil(t, result)
		}
	})

	t.Run("Bitails concurrent access", func(t *testing.T) {
		fixture := testservices.GivenServices(t)
		fixture.Bitails().
			ScriptHistoryData().
			WithScriptHash(testScriptHash).
			WithConfirmedTransactions(1, 800000).
			WithUnconfirmedTransactions(1).
			WillBeReturned()

		svc := fixture.Services().Config(testservices.WithEnabledBitails(true)).New()

		results := make(chan interface{}, numConcurrent)
		errors := make(chan error, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			go func() {
				result, err := svc.GetScriptHashHistory(t.Context(), testScriptHash)
				results <- result
				errors <- err
			}()
		}

		for i := 0; i < numConcurrent; i++ {
			result := <-results
			err := <-errors

			require.NoError(t, err)
			require.NotNil(t, result)
		}
	})
}
