package storage_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	clients "github.com/bsv-blockchain/go-sdk/auth/clients/authhttp"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestAdminStats(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	faucet := given.Faucet(activeStorage, testusers.Alice)
	_, _ = faucet.TopUp(satoshi.MustFrom(1000), func(o *testabilities.TopUpOptions) { o.Mined = true })
	_, _ = faucet.TopUp(satoshi.MustFrom(500))

	// when:
	stats, err := activeStorage.AdminStats(t.Context(), "admin")

	// then:
	require.NoError(t, err)
	assert.Equal(t, "admin", stats.RequestedBy)
	assert.Positive(t, stats.Users.Day)
	assert.Equal(t, stats.Users.Day, stats.Users.Total, "everything was created just now")
	assert.Equal(t, int64(1), stats.TxByStatus[wdk.TxStatusCompleted].Day)
	assert.Equal(t, int64(1), stats.TxByStatus[wdk.TxStatusUnproven].Day)
	assert.Equal(t, int64(2), stats.Transactions.Total)
	assert.Equal(t, wdk.AdminStatsCounts{Day: 1000, Week: 1000, Month: 1000, Total: 1000}, stats.SatoshisDefault, "only completed transactions count")
	assert.Positive(t, stats.Baskets.Total)
}

func TestAdminStatsEndpoint(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	cleanupSrv := given.StartedRPCServerFor(activeStorage, func(opts *storage.ServerOptions) {
		opts.AdminIdentityKeys = []string{testusers.Alice.IdentityKey(t)}
	})
	defer cleanupSrv()

	t.Run("admin gets stats", func(t *testing.T) {
		// when:
		status, body := getAuthenticated(t, given.ServerURL()+"/admin/stats", testusers.Alice)

		// then:
		require.Equal(t, http.StatusOK, status, "body: %s", body)
		var stats wdk.AdminStats
		require.NoError(t, json.Unmarshal(body, &stats))
		assert.Equal(t, testusers.Alice.IdentityKey(t), stats.RequestedBy)
	})

	t.Run("non-admin is forbidden", func(t *testing.T) {
		// when:
		status, _ := getAuthenticated(t, given.ServerURL()+"/admin/stats", testusers.Bob)

		// then:
		assert.Equal(t, http.StatusForbidden, status)
	})
}

func getAuthenticated(t *testing.T, url string, user testusers.User) (int, []byte) {
	t.Helper()

	protoWallet, err := wallet.NewCompletedProtoWallet(user.PrivateKey(t))
	require.NoError(t, err)

	resp, err := clients.New(protoWallet).Fetch(t.Context(), url, &clients.SimplifiedFetchRequestOptions{Method: http.MethodGet})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}
