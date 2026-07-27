// Package config loads throughput-dashboard settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Config holds dashboard runtime settings.
type Config struct {
	ServerURL     string
	Network       string
	PrivateKeyHex string
	Originator    string
	HTTPAddr      string
	TPS           int
	Workers       int
	SampleSeconds int
	// WalletMaxInFlight bounds concurrent storage RPCs on the shared wallet
	// (0 → syncwallet.DefaultMaxInFlight). Tune per stack: too high and the
	// local infra HTTP path starts dropping connections.
	WalletMaxInFlight int
}

// ConfigFromEnv reads configuration from environment variables.
// PRIVATE_KEY is required. Defaults are mainnet-first and demo-safe.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		ServerURL:     envOrDefault("SERVER_URL", "http://127.0.0.1:8101"),
		Network:       envOrDefault("BSV_NETWORK", string(defs.NetworkMainnet)),
		PrivateKeyHex: os.Getenv("PRIVATE_KEY"),
		Originator:    envOrDefault("ORIGINATOR", "throughput-dashboard.local"),
		HTTPAddr:      envOrDefault("HTTP_ADDR", "127.0.0.1:8200"),
		TPS:           10,
		Workers:       0, // 0 → derive from TPS in stream.WorkersForTPS
		SampleSeconds: 1,
	}

	if cfg.PrivateKeyHex == "" {
		return Config{}, fmt.Errorf("PRIVATE_KEY is required")
	}

	var err error
	if cfg.TPS, err = envIntOrDefault("TPS", 10); err != nil {
		return Config{}, err
	}
	// WORKERS=0 or unset → auto from TPS. Positive value is an explicit override.
	if cfg.Workers, err = envIntOrDefault("WORKERS", 0); err != nil {
		return Config{}, err
	}
	if cfg.SampleSeconds, err = envIntOrDefault("SAMPLE_SECONDS", 1); err != nil {
		return Config{}, err
	}
	if cfg.WalletMaxInFlight, err = envIntOrDefault("WALLET_MAX_IN_FLIGHT", 0); err != nil {
		return Config{}, err
	}
	if cfg.WalletMaxInFlight < 0 {
		return Config{}, fmt.Errorf("WALLET_MAX_IN_FLIGHT must be >= 0 (0 = default), got %d", cfg.WalletMaxInFlight)
	}

	if cfg.TPS <= 0 {
		return Config{}, fmt.Errorf("TPS must be > 0, got %d", cfg.TPS)
	}
	// Workers may be 0 (auto-derived from TPS). Negative is invalid.
	if cfg.Workers < 0 {
		return Config{}, fmt.Errorf("WORKERS must be >= 0 (0 = auto from TPS), got %d", cfg.Workers)
	}
	if cfg.SampleSeconds <= 0 {
		return Config{}, fmt.Errorf("SAMPLE_SECONDS must be > 0, got %d", cfg.SampleSeconds)
	}
	if _, err = defs.ParseBSVNetworkStr(cfg.Network); err != nil {
		return Config{}, fmt.Errorf("BSV_NETWORK: %w", err)
	}

	return cfg, nil
}

// DemoThroughput returns the fuel profile for the local demo dashboard.
// Shape is 200 B OP_RETURN / 0 output sats; fee rate (and thus denomination)
// depends on the network — see DemoFeeModel.
//
// TargetPoolSize stays at a modest static default for callers that only need
// the shape; the live dashboard sizes inventory via DemoTargetPoolForTPS from
// the UI/env TPS so FuelKeeper tracks the stream rate.
//
// Mint throughput (for ~1000 TPS catch-up):
//
//	FanoutOutputsPerTx × FanoutMaxTxsPerRound = 100 × 300 = 30_000 fuel UTXOs/round
//
// While below low water the keeper runs rounds back-to-back (not waiting the
// full TopUp interval), so fill rate is gated mainly by fan-out RPC latency.
func DemoThroughput() defs.Throughput {
	base := defs.DefaultUTXOManagement().Throughput
	base.ExpectedTxSizeBytes = 200
	base.ExpectedOutputSatoshis = 0
	// Explicit 30 sats instead of the derived 20: the real 1-input OP_RETURN
	// tx is ~202 B → 21-sat fee at 100 sat/kb, so a 20-sat fuel UTXO just
	// misses and the funder claims TWO per action ("multi_claim") — doubling
	// burn rate and UTXO contention. 30 covers the fee with one claim.
	// (At 1 sat/kb this would be 2 — see DemoFeeModel.)
	// MUST match the server profile (infra-config *throughput*.yaml
	// denomination_satoshis) or every leaf fan-out fails validation.
	base.DenominationSatoshis = 30
	base.TargetTPS = 1000
	base.TargetPoolSize = 500 // static fallback; live path uses DemoTargetPoolForTPS
	base.FanoutOutputsPerTx = 100
	base.FanoutMaxTxsPerRound = 300 // was 50 — too slow to fill 1000 TPS runway
	base.TopUp.IntervalSeconds = 2  // idle poll when healthy; catch-up ignores this
	base.TopUp.Enabled = true
	base.TopUp.StartImmediately = true
	return base
}

// DemoFeeModel is the fee rate the demo dashboard assumes for fuel denomination.
// 100 sat/kb — matches the private Arcade host's GoBDK min-fee policy
// (DefaultMinFeePerKB=100). The going network rate is 1 sat/kb and the stack
// works at it (verified 2026-07-24: a 236 B tx paying exactly 1 sat was
// correctly built), but this Arcade host rejects anything under its floor
// with ARC 465 insufficient-fee. To go back to 1 sat/kb: lower the Arcade
// policy, set Value to 1 here, and set DenominationSatoshis to 2 (both sides —
// keep in sync with infra-config *throughput*.yaml).
func DemoFeeModel(network defs.BSVNetwork) defs.FeeModel {
	_ = network
	return defs.DefaultFeeModel()
}

// DemoDenomination resolves fuel UTXO size for the demo profile on the given network.
// At 100 sat/kb with the 200 B OP_RETURN shape this is 20 sats (derived).
func DemoDenomination(network defs.BSVNetwork) (uint64, error) {
	tp := DemoThroughput()
	return tp.Denomination(DemoFeeModel(network), defs.DefaultCommission())
}

// DemoRefillHorizonSeconds is how much burn the fuel pool must cover: the time
// from minting a fuel UTXO to it becoming spendable. The server marks outputs
// spendable on NETWORK ACCEPTANCE (not on mining), and the demo's spend policy
// accepts unproven/sending fuel, so that is a few seconds of broadcast latency
// — not the 300s mined-confirmation horizon that defs.Throughput assumes.
//
// Sizing on 300s made the target 20× larger than needed (1000 TPS → 450_000
// UTXOs → 13.5M satoshis standing at 30 sats each, more than the demo wallet
// holds). The keeper could then never reach low water, so it stayed in
// permanent catch-up, competing with the stream for the whole run instead of
// filling once and idling.
const DemoRefillHorizonSeconds = 15

// DemoTargetPoolForTPS sizes the FuelKeeper inventory target from a stream TPS
// setting: target_tps × DemoRefillHorizonSeconds × pool_headroom_factor.
//
// Examples (15s horizon, 1.5 headroom):
//
//	10 TPS    → 225 fuel UTXOs
//	100 TPS   → 2_250 fuel UTXOs
//	1000 TPS  → 22_500 fuel UTXOs (675k satoshis at 30 sats — affordable)
func DemoTargetPoolForTPS(tps int) uint64 {
	if tps <= 0 {
		tps = 10
	}
	tp := DemoThroughput()
	tp.TargetTPS = uint64(tps)
	tp.TargetPoolSize = 0 // force derivation from TPS × horizon × headroom
	tp.ExpectedConfirmationSeconds = DemoRefillHorizonSeconds
	return tp.TargetPool()
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid integer %q: %w", key, v, err)
	}
	return n, nil
}
