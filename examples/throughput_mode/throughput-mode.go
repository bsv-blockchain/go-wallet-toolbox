// Throughput-mode wallet server — repo-market profile.
//
// This example shows the end-to-end operator flow of the throughput
// UTXO-management strategy (design: plans/high-throughput-utxo-management.md,
// spec: docs/superpowers/specs/2026-07-18-throughput-fuel-funding-design.md):
//
//  1. Run an infra server with `strategy: throughput` — a denominated "fuel"
//     pool sized to the typical action shape, claimed by the funder in one
//     exact-match micro-query per action.
//  2. Deposit operating funds via ordinary internalizeAction (default
//     basket). The FuelKeeper — a CLIENT-side loop, because fan-out
//     transactions are signed with the operator's keys — chunks those funds
//     into the reserve basket and fans chunks out into exact-denomination
//     fuel, keeping the pool between its water marks.
//  3. Issue packed many-output createActions — the application-layer batching
//     that carries peak load (~160k outputs/s as ~160 actions × 1,000
//     outputs). Each typical 15-output action funds with ONE 240-sat fuel
//     claim; no COUNT(*), no change randomization.
//  4. Observe everything via OpenTelemetry: pool/reserve runway gauges and
//     funder counters export over the same OTLP endpoint as traces, so
//     external tooling (Prometheus/Grafana/Alertmanager) owns low-funds
//     paging. There is no in-process alerting. See throughput-mode.md for the
//     collector setup and the alert-rule set.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/example_setup"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/fuelkeeper"
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
		if serveErr := server.ListenAndServe(ctx); serveErr != nil {
			panic(serveErr)
		}
	}()
	defer server.Cleanup()

	// On startup with strategy: throughput, new users are seeded with the
	// `fuel` and `reserve` baskets alongside `default`.

	// --- 2. Fund the operator and start the FuelKeeper ----------------------
	// Deposit operating funds with an ordinary internalizeAction (they land
	// in the default basket — see throughput-mode.md). The keeper runs in
	// THIS process because fan-out transactions are signed with the
	// operator's keys: each round it measures the pool and, when below low
	// water, chunks default-basket funds into `reserve` and fans chunks out
	// into exact-denomination fuel (~5 min to claimable maturity under
	// prefer_mined; unproven fuel is claimable immediately).
	operator := example_setup.CreateAlice()
	operatorWallet, cleanup := operator.CreateWallet(ctx)
	defer cleanup()

	depositOperatingFunds(ctx, operatorWallet) // see throughput-mode.md for the full flow

	logger := slog.Default()
	keeper, err := fuelkeeper.New(operatorWallet, fuelkeeper.FromThroughput(defs.Throughput{
		DenominationSatoshis:        240,
		TargetTPS:                   10_000,
		ExpectedConfirmationSeconds: 300,
		PoolHeadroomFactor:          1.5,
		TargetPoolSize:              18_000_000,
		LowWaterPercent:             60,
		HighWaterPercent:            100,
		PoolBasket:                  "fuel",
		ReserveBasket:               "reserve",
		FanoutOutputsPerTx:          100,
		FanoutMaxTxsPerRound:        12_000,
		TopUp:                       defs.TaskConfig{Enabled: true, IntervalSeconds: 10},
	}, 240), logger)
	if err != nil {
		panic(fmt.Errorf("failed to create fuel keeper: %w", err))
	}
	go keeper.Run(ctx)

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

// depositOperatingFunds internalizes operating funds with the standard
// wallet-payment protocol — they land in the default basket as
// funder-spendable change. See
// examples/wallet_examples/internalize_tx_from_faucet for the full mechanics.
// The FuelKeeper takes it from there: default → reserve chunks → fuel.
func depositOperatingFunds(ctx context.Context, operatorWallet *wallet.Wallet) {
	// sketch:
	//   operatorWallet.InternalizeAction(ctx, sdk.InternalizeActionArgs{
	//       Tx: fundingBEEF,
	//       Outputs: []sdk.InternalizeOutput{{
	//           OutputIndex:        0,
	//           Protocol:           sdk.InternalizeProtocolWalletPayment,
	//           PaymentRemittance:  &sdk.Payment{ /* derivation from the sender */ },
	//       }},
	//       Description: "operating funds deposit",
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
