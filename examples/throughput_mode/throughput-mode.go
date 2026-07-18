//go:build throughput_example

// Throughput-mode wallet server — repo-market profile.
//
// STATUS: This example tracks the design proposal in
// plans/high-throughput-utxo-management.md (PR #936). It is excluded from
// normal builds via the `throughput_example` build tag until the
// `utxo_management` configuration and the denominated funder land (Phases 1-3
// of the proposal). The build tag will be removed when the feature ships.
//
// It demonstrates the intended end-to-end operator flow:
//
//  1. Run an infra server with `strategy: throughput` — a denominated "fuel"
//     pool sized to the typical action shape, refilled from a "reserve"
//     basket by the pool_top_up monitor task.
//  2. Deposit operating funds into the reserve basket (ordinary
//     internalizeAction with basket insertion).
//  3. Issue packed many-output createActions — the application-layer batching
//     that carries peak load (~160k outputs/s as ~160 actions × 1,000
//     outputs). Each typical 15-output action funds with ONE 240-sat fuel
//     claim; no in-flight signing bottleneck, no COUNT(*), no change
//     randomization.
//  4. Observe everything via OpenTelemetry: pool/reserve runway gauges and
//     funder/top-up counters export over the same OTLP endpoint as traces, so
//     external tooling (Prometheus/Grafana/Alertmanager) owns low-funds
//     paging. There is no in-process alerting. See throughput-mode.md for the
//     collector setup and the alert-rule set.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

// infraConfigYAML is the operator-facing configuration this example runs
// with. Everything under utxo_management is specified by
// plans/high-throughput-utxo-management.md §4; the rest is the standard infra
// config surface.
const infraConfigYAML = `
name: throughput-wallet
bsv_network: main
db:
  engine: postgres   # throughput mode targets Postgres; validation warns on SQLite

fee_model:
  type: sat/kb
  value: 100

utxo_management:
  strategy: throughput          # privacy (default) | throughput
  throughput:
    # Denomination sized to the TYPICAL action: 15 outputs ≈ 668 bytes signed
    # (incl. the single fuel input) → 67-sat fee + ~170 sat of output value.
    # Explicit value wins over derivation; 0 would derive from the fields below.
    denomination_satoshis: 240
    expected_tx_size_bytes: 668
    expected_output_satoshis: 170

    # Pool sized for the diurnal peaks (repo market: open/close bursts of
    # ~160 actions/s × 1,000 outputs ⇒ ~10k fuel claims/s for ~15 min).
    target_tps: 10000               # peak fuel-claim rate, not action rate
    expected_confirmation_seconds: 300
    pool_headroom_factor: 1.5
    target_pool_size: 18000000      # explicit: covers both daily peaks

    low_water_percent: 60
    high_water_percent: 100
    spend_policy: prefer_mined      # mined_only | prefer_mined | any

    pool_basket: fuel
    reserve_basket: reserve

    fanout_outputs_per_tx: 100
    fanout_max_txs_per_round: 12000 # leaf txs; ≥ target_tps × interval × 1.2
    fanout_tree_depth: 2
    consolidation_inputs_per_tx: 1000

    top_up:
      enabled: true
      interval_seconds: 10
      start_immediately: true

# Metrics ride the same OTLP endpoint as traces; no in-process alerting.
observability:
  metrics:
    enabled: true
    export_interval_seconds: 15

tracing:
  enabled: true
  dialAddr: http://localhost:4317   # your OTel collector
  sample: 100
`

const (
	// OutputsPerAction is the typical repo-market action shape. Peak-load
	// actions pack up to 1,000 outputs; the funder multi-claims fuel for
	// those automatically.
	OutputsPerAction = 15

	// Originator identifies the calling application.
	Originator = "repo-market.example.com"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- 1. Run the wallet server in throughput mode -----------------------
	configPath := writeConfig()
	server, err := infra.NewServer(ctx, infra.WithConfigFile(configPath))
	if err != nil {
		panic(fmt.Errorf("failed to create infra server: %w", err))
	}
	go func() {
		if err := server.ListenAndServe(ctx); err != nil {
			panic(err)
		}
	}()
	defer server.Cleanup()

	// On startup with strategy: throughput the provider find-or-creates the
	// `fuel` and `reserve` baskets for the operator user, and the pool_top_up
	// task begins watching the (empty) pool gauge.

	// --- 2. Fund the reserve ------------------------------------------------
	// Deposit large UTXOs into the `reserve` basket with an ordinary
	// internalizeAction (basket insertion). Only the top-up task ever spends
	// reserve; neither funding path can touch it. From here the top-up task
	// fans out: reserve → chunks → 240-sat fuel outputs, ~5 min to maturity.
	operator := example_setup.CreateAlice()
	operatorWallet, cleanup := operator.CreateWallet(ctx)
	defer cleanup()

	depositIntoReserve(ctx, operatorWallet) // see throughput-mode.md for the full flow

	// --- 3. Issue packed createActions --------------------------------------
	// The application batches by packing outputs into actions (there is no
	// batch endpoint — actions are atomic per txid). A typical 15-output
	// action funds with a single exact-match 240-sat fuel claim.
	outputs := make([]sdk.CreateActionOutput, 0, OutputsPerAction)
	for i := 0; i < OutputsPerAction; i++ {
		outputs = append(outputs, sdk.CreateActionOutput{
			LockingScript:     settlementLockingScript(i),
			Satoshis:          11,
			OutputDescription: "repo settlement leg",
		})
	}

	result, err := operatorWallet.CreateAction(ctx, sdk.CreateActionArgs{
		Description: "repo market settlement batch",
		Outputs:     outputs,
	}, Originator)
	if err != nil {
		panic(fmt.Errorf("createAction failed: %w", err))
	}
	fmt.Printf("settled batch txid=%s (%d outputs, 1 fuel claim)\n", result.Txid, OutputsPerAction)

	// --- 4. Watch the metrics ----------------------------------------------
	// wallet.utxo.pool.runway_seconds, wallet.utxo.reserve.runway_seconds,
	// wallet.funder.claims{result}, wallet.topup.* — all flowing to the OTLP
	// collector configured above. Alert thresholds live in your monitoring
	// stack, not in the wallet: see throughput-mode.md for the Prometheus
	// rule set (low pool runway, low reserve runway, top-up failing,
	// funding errors).

	<-ctx.Done()
}

// writeConfig materializes the embedded YAML so infra.WithConfigFile can load
// it. In a real deployment the file is provisioned by your infrastructure and
// secrets (db password, server key) arrive via the loader's env overrides.
func writeConfig() string {
	path := "throughput-config.yaml"
	if err := os.WriteFile(path, []byte(infraConfigYAML), 0o600); err != nil {
		panic(fmt.Errorf("failed to write config: %w", err))
	}
	return path
}

// depositIntoReserve internalizes operating funds into the `reserve` basket.
// The flow is the standard basket-insertion internalizeAction — see
// examples/wallet_examples/internalize_tx_from_faucet for the mechanics; the
// only throughput-mode specific detail is basket: "reserve".
func depositIntoReserve(ctx context.Context, operatorWallet *wallet.Wallet) {
	// sketch:
	//   operatorWallet.InternalizeAction(ctx, sdk.InternalizeActionArgs{
	//       Tx: fundingBEEF,
	//       Outputs: []sdk.InternalizeOutput{{
	//           OutputIndex: 0,
	//           Protocol:    sdk.InternalizeProtocolBasketInsertion,
	//           InsertionRemittance: &sdk.BasketInsertion{Basket: "reserve"},
	//       }},
	//       Description: "operating reserve deposit",
	//   }, Originator)
	_ = ctx
	_ = operatorWallet
}

// settlementLockingScript stands in for the application's real per-leg
// locking script (P2PKH to a counterparty, a token transfer, etc.).
func settlementLockingScript(leg int) []byte {
	_ = leg
	return []byte{0x51} // OP_TRUE placeholder — replace with real scripts
}
