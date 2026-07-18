package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func feeModel(satPerKB int64) defs.FeeModel {
	return defs.FeeModel{Type: defs.SatPerKB, Value: satPerKB}
}

func throughputConfig() defs.UTXOManagement {
	cfg := defs.DefaultUTXOManagement()
	cfg.Strategy = defs.StrategyThroughput
	return cfg
}

func TestDenominationDerivation(t *testing.T) {
	tests := map[string]struct {
		throughput   defs.Throughput
		commission   defs.Commission
		feeRate      int64
		expected     uint64
		expectsError bool
	}{
		"explicit override wins": {
			throughput: defs.Throughput{DenominationSatoshis: 240, ExpectedTxSizeBytes: 9999},
			feeRate:    100,
			expected:   240,
		},
		"derived from 240-byte tx at 100 sat/kb": {
			throughput: defs.Throughput{ExpectedTxSizeBytes: 240},
			feeRate:    100,
			expected:   24,
		},
		"derived with output satoshis": {
			throughput: defs.Throughput{ExpectedTxSizeBytes: 668, ExpectedOutputSatoshis: 170},
			feeRate:    100,
			expected:   237, // ceil(66.8) = 67 + 170
		},
		"commission folds in satoshis and output bytes": {
			throughput: defs.Throughput{ExpectedTxSizeBytes: 240},
			commission: defs.Commission{Satoshis: 10, PubKeyHex: "02f5f0a5b6f3f9e1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f70819aa"},
			feeRate:    100,
			expected:   38, // ceil((240+34)/1000*100) = 28 + 10
		},
		"zero everything errors": {
			throughput:   defs.Throughput{},
			feeRate:      100,
			expectsError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			denomination, err := test.throughput.Denomination(feeModel(test.feeRate), test.commission)
			if test.expectsError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expected, denomination)
		})
	}
}

func TestMarginalFuelInputFee(t *testing.T) {
	require.Equal(t, uint64(15), defs.MarginalFuelInputFee(feeModel(100))) // ceil(148/1000*100)
	require.Equal(t, uint64(23), defs.MarginalFuelInputFee(feeModel(150)))
}

func TestTargetPool(t *testing.T) {
	throughput := defs.Throughput{TargetTPS: 10_000, ExpectedConfirmationSeconds: 300, PoolHeadroomFactor: 1.5}
	require.Equal(t, uint64(4_500_000), throughput.TargetPool())

	throughput.TargetPoolSize = 18_000_000
	require.Equal(t, uint64(18_000_000), throughput.TargetPool())
}

func TestUTXOManagementValidate(t *testing.T) {
	t.Run("privacy strategy skips throughput validation", func(t *testing.T) {
		cfg := defs.UTXOManagement{Strategy: defs.StrategyPrivacy}
		require.NoError(t, cfg.Validate(feeModel(100), defs.Commission{}))
	})

	t.Run("default throughput config is valid", func(t *testing.T) {
		cfg := throughputConfig()
		require.NoError(t, cfg.Validate(feeModel(100), defs.Commission{}))
	})

	t.Run("strategy is case-insensitive and normalized", func(t *testing.T) {
		cfg := defs.UTXOManagement{Strategy: "Privacy"}
		require.NoError(t, cfg.Validate(feeModel(100), defs.Commission{}))
		require.Equal(t, defs.StrategyPrivacy, cfg.Strategy)
	})

	t.Run("unknown strategy rejected", func(t *testing.T) {
		cfg := defs.UTXOManagement{Strategy: "turbo"}
		require.Error(t, cfg.Validate(feeModel(100), defs.Commission{}))
	})

	rejections := map[string]func(*defs.Throughput){
		"denomination at the marginal input fee": func(tp *defs.Throughput) {
			tp.DenominationSatoshis = 15 // == marginal fee at 100 sat/kb
		},
		"unknown spend policy": func(tp *defs.Throughput) {
			tp.SpendPolicy = "yolo"
		},
		"pool basket collides with default": func(tp *defs.Throughput) {
			tp.PoolBasket = "default"
		},
		"pool and reserve baskets collide": func(tp *defs.Throughput) {
			tp.ReserveBasket = tp.PoolBasket
		},
		"zero target tps": func(tp *defs.Throughput) {
			tp.TargetTPS = 0
		},
		"water marks inverted": func(tp *defs.Throughput) {
			tp.LowWaterPercent = 90
			tp.HighWaterPercent = 50
		},
		"tree depth out of range": func(tp *defs.Throughput) {
			tp.FanoutTreeDepth = 3
		},
		"sustained-throughput identity violated": func(tp *defs.Throughput) {
			tp.FanoutMaxTxsPerRound = 10 // 100×10 = 1000 < 10000×10×1.2
		},
		"zero consolidation inputs": func(tp *defs.Throughput) {
			tp.ConsolidationInputsPerTx = 0
		},
		"top up interval zero": func(tp *defs.Throughput) {
			tp.TopUp.IntervalSeconds = 0
		},
	}

	for name, mutate := range rejections {
		t.Run("rejects "+name, func(t *testing.T) {
			cfg := throughputConfig()
			mutate(&cfg.Throughput)
			require.Error(t, cfg.Validate(feeModel(100), defs.Commission{}))
		})
	}

	t.Run("identity not enforced when top up disabled", func(t *testing.T) {
		cfg := throughputConfig()
		cfg.Throughput.TopUp.Enabled = false
		cfg.Throughput.FanoutMaxTxsPerRound = 10
		require.NoError(t, cfg.Validate(feeModel(100), defs.Commission{}))
	})
}
