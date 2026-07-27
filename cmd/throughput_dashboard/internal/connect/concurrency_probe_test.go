package connect_test

// Live probe for how far N independent storage clients (each with its OWN
// identity key, hence its own AuthFetch session) scale RPC throughput against
// a running infra server. Guides the multi-client pool sizing for high-TPS
// streaming; not run in CI.
//
// Usage:
//
//	PROBE_SERVER_URL=http://127.0.0.1:8101 PROBE_NETWORK=tstn \
//	  go test ./cmd/throughput_dashboard/internal/connect -run TestProbeServerConcurrency -v

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/connect"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

// probeMeasureWindow is how long each probe configuration is hammered.
const probeMeasureWindow = 10 * time.Second

func TestProbeServerConcurrency(t *testing.T) {
	serverURL, network := probeEnv(t)

	for _, clients := range []int{1, 4, 8, 16, 32} {
		t.Run(nameForClients(clients), func(t *testing.T) {
			wallets := dialWallets(t, clients, network, serverURL)

			res := hammer(t, clients, probeMeasureWindow, func(i int) *wallet.Wallet {
				// Serial per client (a single client is used one request at a
				// time); parallelism comes from N distinct clients.
				return wallets[i]
			})

			rate := float64(res.ops) / res.elapsed.Seconds()
			t.Logf("PROBE clients=%d ops=%d elapsed=%s rate=%.1f rpc/s (%.1f rpc/s per client)",
				clients, res.ops, res.elapsed.Round(time.Millisecond), rate, rate/float64(clients))
		})
	}
}

func nameForClients(n int) string {
	return fmt.Sprintf("clients_%02d", n)
}

// TestProbeSingleClientConcurrency drives ONE wallet client (one identity, one
// AuthFetch session) from K goroutines concurrently. Before the BRC-104 auth
// fixes released in go-sdk v1.3.2 this hung; it now measures the safe in-flight
// window for the dashboard's shared wallet.
func TestProbeSingleClientConcurrency(t *testing.T) {
	serverURL, network := probeEnv(t)

	priv, err := ec.NewPrivateKey()
	require.NoError(t, err)
	w, err := connect.Wallet(context.Background(), network, priv, serverURL, nil)
	require.NoError(t, err)
	t.Cleanup(w.Close)

	for _, workers := range []int{1, 8, 16, 32} {
		t.Run(fmt.Sprintf("goroutines_%02d", workers), func(t *testing.T) {
			res := hammer(t, workers, probeMeasureWindow, func(int) *wallet.Wallet { return w })

			t.Logf("PROBE single-client goroutines=%d ops=%d failures=%d rate=%.1f rpc/s",
				workers, res.ops, res.failures, float64(res.ops)/res.elapsed.Seconds())
		})
	}
}

// probeResult is the outcome of hammering a set of wallets concurrently.
type probeResult struct {
	ops      uint64
	failures uint64
	elapsed  time.Duration
}

// hammer runs `workers` goroutines for `d`, each repeatedly calling fn on the
// wallet its index selects, and reports the aggregate throughput.
func hammer(t *testing.T, workers int, d time.Duration, pick func(i int) *wallet.Wallet) probeResult {
	t.Helper()

	var ops, failures atomic.Uint64
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	var wg sync.WaitGroup
	start := time.Now()
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := pick(i)
			for ctx.Err() == nil {
				_, err := w.ListOutputs(ctx, sdk.ListOutputsArgs{
					Basket: "fuel",
					Limit:  to.Ptr(uint32(1)),
				}, "probe")
				switch {
				case err == nil:
					ops.Add(1)
				case ctx.Err() != nil:
					return
				default:
					if failures.Add(1) <= 3 {
						t.Logf("sample failure: %v", err)
					}
				}
			}
		}(i)
	}
	wg.Wait()

	return probeResult{ops: ops.Load(), failures: failures.Load(), elapsed: time.Since(start)}
}

// dialWallets opens n independent clients, each with its own identity key.
func dialWallets(t *testing.T, n int, network defs.BSVNetwork, serverURL string) []*wallet.Wallet {
	t.Helper()

	wallets := make([]*wallet.Wallet, n)
	for i := range wallets {
		priv, err := ec.NewPrivateKey()
		require.NoError(t, err)
		w, err := connect.Wallet(context.Background(), network, priv, serverURL, nil)
		require.NoError(t, err, "client %d connect", i)
		t.Cleanup(w.Close)
		wallets[i] = w
	}
	return wallets
}

// probeEnv reads the live-probe configuration, skipping when it is absent.
func probeEnv(t *testing.T) (serverURL string, network defs.BSVNetwork) {
	t.Helper()

	serverURL = os.Getenv("PROBE_SERVER_URL")
	if serverURL == "" {
		t.Skip("PROBE_SERVER_URL not set; live probe skipped")
	}
	network, err := defs.ParseBSVNetworkStr(os.Getenv("PROBE_NETWORK"))
	require.NoError(t, err)
	return serverURL, network
}
