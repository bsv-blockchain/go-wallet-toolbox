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

func TestProbeServerConcurrency(t *testing.T) {
	serverURL := os.Getenv("PROBE_SERVER_URL")
	if serverURL == "" {
		t.Skip("PROBE_SERVER_URL not set; live probe skipped")
	}
	network, err := defs.ParseBSVNetworkStr(os.Getenv("PROBE_NETWORK"))
	require.NoError(t, err)

	const measure = 10 * time.Second

	for _, clients := range []int{1, 4, 8, 16, 32} {
		t.Run(nameForClients(clients), func(t *testing.T) {
			wallets := make([]*wallet.Wallet, clients)
			for i := range wallets {
				priv, err := ec.NewPrivateKey()
				require.NoError(t, err)
				w, err := connect.Wallet(context.Background(), network, priv, serverURL, nil)
				require.NoError(t, err, "client %d connect", i)
				t.Cleanup(w.Close)
				wallets[i] = w
			}

			var ops atomic.Uint64
			ctx, cancel := context.WithTimeout(context.Background(), measure)
			defer cancel()
			var wg sync.WaitGroup
			start := time.Now()
			for _, w := range wallets {
				wg.Add(1)
				go func(w *wallet.Wallet) {
					defer wg.Done()
					for ctx.Err() == nil {
						// Serial per client (single AuthFetch is not
						// concurrency-safe); parallelism comes from N clients.
						if _, err := w.ListOutputs(ctx, sdk.ListOutputsArgs{
							Basket: "fuel",
							Limit:  to.Ptr(uint32(1)),
						}, "probe"); err != nil {
							if ctx.Err() != nil {
								return
							}
							t.Logf("listOutputs error: %v", err)
							return
						}
						ops.Add(1)
					}
				}(w)
			}
			wg.Wait()
			elapsed := time.Since(start)
			rate := float64(ops.Load()) / elapsed.Seconds()
			t.Logf("PROBE clients=%d ops=%d elapsed=%s rate=%.1f rpc/s (%.1f rpc/s per client)",
				clients, ops.Load(), elapsed.Round(time.Millisecond), rate, rate/float64(clients))
		})
	}
}

func nameForClients(n int) string {
	return fmt.Sprintf("clients_%02d", n)
}

// TestProbeSingleClientConcurrency drives ONE wallet client (one identity, one
// AuthFetch session) from K goroutines concurrently. Upstream go-sdk v1.3.1
// hangs here (response listener bug — see third_party/go-sdk local patch);
// with the patch each request resolves independently, so this measures the
// safe in-flight window for the dashboard's shared wallet.
func TestProbeSingleClientConcurrency(t *testing.T) {
	serverURL := os.Getenv("PROBE_SERVER_URL")
	if serverURL == "" {
		t.Skip("PROBE_SERVER_URL not set; live probe skipped")
	}
	network, err := defs.ParseBSVNetworkStr(os.Getenv("PROBE_NETWORK"))
	require.NoError(t, err)

	const measure = 10 * time.Second

	priv, err := ec.NewPrivateKey()
	require.NoError(t, err)
	w, err := connect.Wallet(context.Background(), network, priv, serverURL, nil)
	require.NoError(t, err)
	t.Cleanup(w.Close)

	for _, workers := range []int{1, 8, 16, 32} {
		t.Run(fmt.Sprintf("goroutines_%02d", workers), func(t *testing.T) {
			var ops, failures atomic.Uint64
			ctx, cancel := context.WithTimeout(context.Background(), measure)
			defer cancel()
			var wg sync.WaitGroup
			start := time.Now()
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for ctx.Err() == nil {
						if _, err := w.ListOutputs(ctx, sdk.ListOutputsArgs{
							Basket: "fuel",
							Limit:  to.Ptr(uint32(1)),
						}, "probe"); err != nil {
							if ctx.Err() != nil {
								return
							}
							if failures.Add(1) <= 3 {
								t.Logf("sample failure: %v", err)
							}
							continue
						}
						ops.Add(1)
					}
				}()
			}
			wg.Wait()
			elapsed := time.Since(start)
			rate := float64(ops.Load()) / elapsed.Seconds()
			t.Logf("PROBE single-client goroutines=%d ops=%d failures=%d rate=%.1f rpc/s",
				workers, ops.Load(), failures.Load(), rate)
		})
	}
}
