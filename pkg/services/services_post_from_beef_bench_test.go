package services_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/transaction"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// benchBroadcastServices returns services broadcasting only to Arcade at arcadeURL, or, when
// arcadeURL is empty, only to the given custom implementation (the broadcast-to-all path).
func benchBroadcastServices(b *testing.B, arcadeURL string, custom services.Implementation) *services.WalletServices {
	b.Helper()
	config := defs.DefaultServicesConfig(defs.NetworkTestnet)
	config.ArcConfig.Enabled = false
	config.ArcGorillaPoolConfig.Enabled = false
	config.WhatsOnChain.Enabled = false
	config.Bitails.Enabled = false
	config.BHS.Enabled = false
	config.ChaintracksClient.Enabled = false
	config.Arcade.Enabled = arcadeURL != ""
	config.Arcade.URL = arcadeURL
	config.Arcade.EventsURL = arcadeURL

	var opts []func(*services.Options)
	if arcadeURL == "" {
		opts = append(opts, services.WithCustomImplementation("custom", custom))
	}
	return services.New(slog.New(slog.DiscardHandler), config, opts...)
}

// benchChain returns a BEEF holding a chain of n unmined transactions, each spending the
// previous one, and their txids parents first.
func benchChain(b *testing.B, n int) (*sdk.Beef, []string) {
	b.Helper()
	tx := txtestabilities.GivenTX().
		WithSender(txtestabilities.Alice).WithRecipient(txtestabilities.Alice).
		WithInput(100_000).
		WithP2PKHOutput(99_000).
		TX()
	txIDs := []string{tx.TxID().String()}
	for i := 1; i < n; i++ {
		tx = txtestabilities.GivenTX().
			WithSender(txtestabilities.Alice).WithRecipient(txtestabilities.Alice).
			WithInputFromUTXO(tx, 0).
			WithP2PKHOutput(uint64(99_000 - i*100)).
			TX()
		txIDs = append(txIDs, tx.TxID().String())
	}
	beef, err := sdk.NewBeefFromTransaction(tx)
	require.NoError(b, err)
	return beef, txIDs
}

func acceptEF(_ context.Context, _, txID string) (*wdk.PostedTxID, error) {
	return &wdk.PostedTxID{TxID: txID, Result: wdk.PostedTxIDResultSuccess}, nil
}

func acceptEFBatch(_ context.Context, txs []services.EFTransaction) ([]*wdk.PostedTxID, error) {
	results := make([]*wdk.PostedTxID, len(txs))
	for i, tx := range txs {
		results[i] = &wdk.PostedTxID{TxID: tx.TxID, Result: wdk.PostedTxIDResultSuccess}
	}
	return results, nil
}

// BenchmarkPostFromBEEF measures PostFromBEEF of a chain of 1 or 5 transactions: through Arcade
// (an httptest server that accepts every transaction), and on the broadcast-to-all path through
// a custom implementation with PostEF (EF hex, one call per tx) or PostEFBatch (EF bytes, one
// call per PostFromBEEF).
//
//	go test -run '^$' -bench PostFromBEEF -benchmem ./pkg/services/
func BenchmarkPostFromBEEF(b *testing.B) {
	arcade := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"txid": "accepted", "txStatus": "SEEN_ON_NETWORK"})
	}))
	defer arcade.Close()

	targets := []struct {
		name      string
		arcadeURL string
		custom    services.Implementation
	}{
		{name: "arcade", arcadeURL: arcade.URL},
		{name: "custom_PostEF", custom: services.Implementation{PostEF: acceptEF}},
		{name: "custom_PostEFBatch", custom: services.Implementation{PostEFBatch: acceptEFBatch}},
	}

	for _, target := range targets {
		for _, n := range []int{1, 5} {
			b.Run(target.name+"/txs="+strconv.Itoa(n), func(b *testing.B) {
				svc := benchBroadcastServices(b, target.arcadeURL, target.custom)
				beef, txIDs := benchChain(b, n)

				b.ReportAllocs()
				for b.Loop() {
					if _, err := svc.PostFromBEEF(b.Context(), beef, txIDs); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
