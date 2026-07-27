package defs

import (
	"fmt"
	"math"
)

// UTXOManagementStrategy selects how the wallet manages its spendable UTXO pool.
type UTXOManagementStrategy string

// Supported UTXO management strategies.
const (
	// StrategyPrivacy is the default: randomized change values, best-fit funding
	// from the default change basket.
	StrategyPrivacy UTXOManagementStrategy = "privacy"
	// StrategyThroughput enables the denominated fuel pool: exact-value claims
	// from a dedicated basket, deterministic change, proactive fan-out top-up.
	StrategyThroughput UTXOManagementStrategy = "throughput"
)

// ParseUTXOManagementStrategyStr parses a string into a UTXOManagementStrategy (case-insensitive).
func ParseUTXOManagementStrategyStr(str string) (UTXOManagementStrategy, error) {
	return parseEnumCaseInsensitive(str, StrategyPrivacy, StrategyThroughput)
}

// SpendPolicy controls which UTXO status tiers the throughput funder may claim from.
type SpendPolicy string

// Supported spend policies.
const (
	// SpendPolicyMinedOnly claims only block-included UTXOs.
	SpendPolicyMinedOnly SpendPolicy = "mined_only"
	// SpendPolicyPreferMined claims mined UTXOs first, falling back to unproven.
	SpendPolicyPreferMined SpendPolicy = "prefer_mined"
	// SpendPolicyAny additionally allows UTXOs from transactions still being sent.
	SpendPolicyAny SpendPolicy = "any"
)

// ParseSpendPolicyStr parses a string into a SpendPolicy (case-insensitive).
func ParseSpendPolicyStr(str string) (SpendPolicy, error) {
	return parseEnumCaseInsensitive(str, SpendPolicyMinedOnly, SpendPolicyPreferMined, SpendPolicyAny)
}

const (
	// fuelInputSizeBytes is the serialized size of one P2PKH input spending a
	// fuel UTXO (mirrors txutils.P2PKHEstimatedInputSize, duplicated here to
	// keep defs dependency-free).
	fuelInputSizeBytes = 148
	// commissionOutputSizeBytes is the serialized size of the storage
	// commission output (P2PKH-shaped locking script).
	commissionOutputSizeBytes = 34
	// fanoutRecoveryMargin is the required headroom of per-round mint capacity
	// over rated consumption, so the pool can climb back from low water while
	// absorbing load.
	fanoutRecoveryMargin = 1.2

	// defaultChangeBasketName mirrors wdk.BasketNameForChange (duplicated to
	// avoid a defs→wdk dependency); pool/reserve baskets must not collide with it.
	defaultChangeBasketName = "default"
)

// Throughput configures the denominated fuel pool used by StrategyThroughput.
type Throughput struct {
	// ExpectedTxSizeBytes is the final signed size of the operator's typical
	// action, INCLUDING the fuel input(s) the funder adds.
	ExpectedTxSizeBytes uint64 `mapstructure:"expected_tx_size_bytes"`
	// ExpectedOutputSatoshis is the satoshi value carried by the typical
	// action's own outputs (0 for pure-fee/data transactions).
	ExpectedOutputSatoshis uint64 `mapstructure:"expected_output_satoshis"`
	// DenominationSatoshis overrides the derived denomination when > 0.
	DenominationSatoshis uint64 `mapstructure:"denomination_satoshis"`

	// TargetTPS is the sustained fuel-claim rate the pool must absorb.
	TargetTPS uint64 `mapstructure:"target_tps"`
	// ExpectedConfirmationSeconds is the average block-inclusion wait for
	// minted fuel.
	ExpectedConfirmationSeconds uint64 `mapstructure:"expected_confirmation_seconds"`
	// PoolHeadroomFactor is the safety multiplier over
	// target_tps × expected_confirmation_seconds when deriving the pool target.
	PoolHeadroomFactor float64 `mapstructure:"pool_headroom_factor"`
	// TargetPoolSize overrides the derived pool target when > 0.
	TargetPoolSize uint64 `mapstructure:"target_pool_size"`
	// LowWaterPercent triggers top-up when the pool drops below this fraction
	// of the target; HighWaterPercent is the refill target.
	LowWaterPercent  uint64 `mapstructure:"low_water_percent"`
	HighWaterPercent uint64 `mapstructure:"high_water_percent"`

	// SpendPolicy controls which UTXO status tiers may be claimed.
	SpendPolicy SpendPolicy `mapstructure:"spend_policy"`

	// PoolBasket holds the denominated fuel UTXOs; ReserveBasket holds the
	// operator's deposits that fan-out draws from.
	PoolBasket    string `mapstructure:"pool_basket"`
	ReserveBasket string `mapstructure:"reserve_basket"`

	// FanoutOutputsPerTx is the number of fuel outputs per leaf fan-out tx.
	FanoutOutputsPerTx uint64 `mapstructure:"fanout_outputs_per_tx"`
	// FanoutMaxTxsPerRound caps LEAF fan-out transactions per top-up round.
	FanoutMaxTxsPerRound uint64 `mapstructure:"fanout_max_txs_per_round"`
	// FanoutTreeDepth is 1 (reserve → fuel) or 2 (reserve → chunks → fuel).
	FanoutTreeDepth uint64 `mapstructure:"fanout_tree_depth"`
	// ConsolidationInputsPerTx caps stale fuel inputs per consolidation tx.
	ConsolidationInputsPerTx uint64 `mapstructure:"consolidation_inputs_per_tx"`

	// TopUp schedules the client-side fuel keeper rounds. It is consumed by the
	// keeper (which holds the operator's keys), not by the server monitor.
	TopUp TaskConfig `mapstructure:"top_up"`
}

// UTXOManagement is the top-level UTXO management configuration.
type UTXOManagement struct {
	Strategy   UTXOManagementStrategy `mapstructure:"strategy"`
	Throughput Throughput             `mapstructure:"throughput"`
}

// DefaultUTXOManagement returns the default configuration: the privacy
// strategy, with throughput defaults shaped for the repo-market profile
// documented in plans/high-throughput-utxo-management.md §6.1.
func DefaultUTXOManagement() UTXOManagement {
	return UTXOManagement{
		Strategy: StrategyPrivacy,
		Throughput: Throughput{
			ExpectedTxSizeBytes:         668,
			ExpectedOutputSatoshis:      170,
			DenominationSatoshis:        0,
			TargetTPS:                   10_000,
			ExpectedConfirmationSeconds: 300,
			PoolHeadroomFactor:          1.5,
			TargetPoolSize:              0,
			LowWaterPercent:             60,
			HighWaterPercent:            100,
			SpendPolicy:                 SpendPolicyPreferMined,
			PoolBasket:                  "fuel",
			ReserveBasket:               "reserve",
			FanoutOutputsPerTx:          100,
			FanoutMaxTxsPerRound:        12_000,
			FanoutTreeDepth:             2,
			ConsolidationInputsPerTx:    1000,
			TopUp: TaskConfig{
				Enabled:          true,
				IntervalSeconds:  10,
				StartImmediately: true,
			},
		},
	}
}

// Enabled reports whether the throughput strategy is active.
func (u *UTXOManagement) Enabled() bool {
	return u.Strategy == StrategyThroughput
}

// Denomination resolves the fuel denomination in satoshis: the explicit
// override when set, otherwise derived from the expected action shape as
// ceil(size/1000 × fee rate) + expected output satoshis
// (+ commission satoshis and the commission output's bytes when enabled).
func (t *Throughput) Denomination(feeModel FeeModel, commission Commission) (uint64, error) {
	if t.DenominationSatoshis > 0 {
		return t.DenominationSatoshis, nil
	}
	size := t.ExpectedTxSizeBytes
	extra := t.ExpectedOutputSatoshis
	if commission.Enabled() {
		size += commissionOutputSizeBytes
		extra += commission.Satoshis
	}
	denomination := feeForSize(size, feeModel) + extra
	if denomination == 0 {
		return 0, fmt.Errorf("derived denomination is zero: set expected_tx_size_bytes/expected_output_satoshis or denomination_satoshis")
	}
	return denomination, nil
}

// MarginalFuelInputFee is the fee cost of including one fuel input at the
// configured fee rate — the throughput-mode denomination floor. The generic
// funder dust floor (2× a minimal future spend) deliberately does not apply:
// a fuel UTXO's spend fee is priced into the denomination by construction.
func MarginalFuelInputFee(feeModel FeeModel) uint64 {
	return feeForSize(fuelInputSizeBytes, feeModel)
}

// TargetPool resolves the steady-state pool size: the explicit override when
// set, otherwise target_tps × expected_confirmation_seconds × headroom.
func (t *Throughput) TargetPool() uint64 {
	if t.TargetPoolSize > 0 {
		return t.TargetPoolSize
	}
	derived := float64(t.TargetTPS) * float64(t.ExpectedConfirmationSeconds) * t.PoolHeadroomFactor
	return uint64(math.Ceil(derived))
}

func feeForSize(sizeBytes uint64, feeModel FeeModel) uint64 {
	if feeModel.Value <= 0 {
		return 0
	}
	return uint64(math.Ceil(float64(sizeBytes) / 1000.0 * float64(feeModel.Value)))
}

// Validate checks the UTXO management configuration. Throughput invariants are
// only enforced when the throughput strategy is selected.
func (u *UTXOManagement) Validate(feeModel FeeModel, commission Commission) error {
	var err error
	if u.Strategy, err = ParseUTXOManagementStrategyStr(string(u.Strategy)); err != nil {
		return fmt.Errorf("invalid utxo management strategy: %w", err)
	}
	if u.Strategy != StrategyThroughput {
		return nil
	}
	return u.Throughput.validate(feeModel, commission)
}

func (t *Throughput) validate(feeModel FeeModel, commission Commission) error {
	var err error
	if t.SpendPolicy, err = ParseSpendPolicyStr(string(t.SpendPolicy)); err != nil {
		return fmt.Errorf("invalid spend policy: %w", err)
	}
	if err = t.validateBaskets(); err != nil {
		return err
	}
	if err = t.validateDenomination(feeModel, commission); err != nil {
		return err
	}
	if err = t.validatePoolSizing(); err != nil {
		return err
	}
	if err = t.validateFanoutShape(); err != nil {
		return err
	}
	return t.validateTopUpCapacity()
}

func (t *Throughput) validateBaskets() error {
	if t.PoolBasket == "" || t.ReserveBasket == "" {
		return fmt.Errorf("pool_basket and reserve_basket must be non-empty")
	}
	if t.PoolBasket == t.ReserveBasket {
		return fmt.Errorf("pool_basket and reserve_basket must differ, both are %q", t.PoolBasket)
	}
	if t.PoolBasket == defaultChangeBasketName || t.ReserveBasket == defaultChangeBasketName {
		return fmt.Errorf("pool_basket and reserve_basket must not be the default change basket %q", defaultChangeBasketName)
	}
	return nil
}

func (t *Throughput) validateDenomination(feeModel FeeModel, commission Commission) error {
	denomination, err := t.Denomination(feeModel, commission)
	if err != nil {
		return err
	}
	if floor := MarginalFuelInputFee(feeModel); denomination <= floor {
		return fmt.Errorf("denomination %d must exceed the marginal fuel input fee %d at the configured fee rate", denomination, floor)
	}
	return nil
}

func (t *Throughput) validatePoolSizing() error {
	if t.TargetTPS == 0 {
		return fmt.Errorf("target_tps must be greater than 0")
	}
	if t.TargetPoolSize == 0 && (math.IsNaN(t.PoolHeadroomFactor) || math.IsInf(t.PoolHeadroomFactor, 0) || t.PoolHeadroomFactor < 1) {
		return fmt.Errorf("pool_headroom_factor must be a finite number >= 1 when target_pool_size is derived, got %v", t.PoolHeadroomFactor)
	}
	if t.TargetPool() == 0 {
		return fmt.Errorf("target pool size resolves to 0: set target_pool_size or target_tps × expected_confirmation_seconds × pool_headroom_factor")
	}
	if t.LowWaterPercent == 0 || t.LowWaterPercent > t.HighWaterPercent || t.HighWaterPercent > 100 {
		return fmt.Errorf("water marks must satisfy 0 < low_water_percent (%d) <= high_water_percent (%d) <= 100", t.LowWaterPercent, t.HighWaterPercent)
	}
	return nil
}

func (t *Throughput) validateFanoutShape() error {
	if t.FanoutTreeDepth != 1 && t.FanoutTreeDepth != 2 {
		return fmt.Errorf("fanout_tree_depth must be 1 or 2, got %d", t.FanoutTreeDepth)
	}
	if t.FanoutOutputsPerTx == 0 || t.FanoutMaxTxsPerRound == 0 {
		return fmt.Errorf("fanout_outputs_per_tx and fanout_max_txs_per_round must be greater than 0")
	}
	if t.ConsolidationInputsPerTx == 0 {
		return fmt.Errorf("consolidation_inputs_per_tx must be greater than 0")
	}
	return nil
}

func (t *Throughput) validateTopUpCapacity() error {
	if !t.TopUp.Enabled {
		return nil
	}
	if t.TopUp.IntervalSeconds == 0 {
		return fmt.Errorf("top_up.interval_seconds must be greater than 0")
	}
	mintCapacity := float64(t.FanoutOutputsPerTx) * float64(t.FanoutMaxTxsPerRound)
	required := float64(t.TargetTPS) * float64(t.TopUp.IntervalSeconds) * fanoutRecoveryMargin
	if mintCapacity < required {
		return fmt.Errorf(
			"sustained-throughput identity violated: fanout_outputs_per_tx × fanout_max_txs_per_round = %.0f must be >= target_tps × top_up.interval_seconds × %.1f = %.0f",
			mintCapacity, fanoutRecoveryMargin, required,
		)
	}
	return nil
}
