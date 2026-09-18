package funder

import (
	"fmt"
	"testing"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
)

// TestNewCollector_ZeroMinimumDesiredUTXOValue_DoesNotPanic drives newCollector with
// minimumDesiredUTXOValue=0, which used to panic with "integer divide by zero" in
// calculateChangeCount (change/c.minimumDesiredUTXOValue). A negative txSats (a valid,
// documented input — see create.go's "target satoshis can be negative" note) makes
// change() positive immediately during construction, without needing to allocate any UTXO,
// so newCollector alone is enough to reach the division.
func TestNewCollector_ZeroMinimumDesiredUTXOValue_DoesNotPanic(t *testing.T) {
	feeCalculator := newFeeCalculator(defs.FeeModel{Type: defs.SatPerKB, Value: 1})

	var (
		collector *utxoCollector
		err       error
	)

	require.NotPanics(t, func() {
		collector, err = newCollector(
			-1000, // txSats: negative so change() > 0 with no UTXOs allocated yet
			44,    // txSize
			1,     // outputCount
			1,     // numberOfDesiredUTXOs
			0,     // minimumDesiredUTXOValue - previously caused a divide-by-zero panic
			feeCalculator,
			100, // maxChangeOutputsPerTx
			false,
		)
	})

	require.NoError(t, err)
	require.NotNil(t, collector)
	require.GreaterOrEqual(t, collector.minimumDesiredUTXOValue, uint64(1), "minimumDesiredUTXOValue must be clamped to at least 1")
	require.GreaterOrEqual(t, collector.changeOutputsCount, uint64(1))
}

// requireDistributableChange asserts the invariants the change distribution relies on:
// the result's change can be split into ChangeOutputsCount outputs and the fee matches
// the transaction size that includes the change outputs exactly once.
func requireDistributableChange(t *testing.T, collector *utxoCollector, result *Result, baseSize uint64, feeCalculator *feeCalc) {
	t.Helper()

	dist := txutils.NewChangeDistribution(result.ChangeInitialValue, func(uint64) uint64 { return 0 })
	require.True(t, dist.CanDistribute(result.ChangeOutputsCount, result.ChangeAmount),
		"cannot distribute change %d among %d outputs", result.ChangeAmount, result.ChangeOutputsCount)

	values, err := dist.Distribute(result.ChangeOutputsCount, result.ChangeAmount)
	require.NoError(t, err)
	if result.ChangeOutputsCount > 0 {
		outputs := seq.Collect(values)
		require.Len(t, outputs, int(result.ChangeOutputsCount))
		require.Equal(t, result.ChangeAmount, satoshi.MustSum(seq.FromSlice(outputs)))
	}

	require.Equal(t, baseSize+collector.changeOutputsSize, collector.txSize,
		"change outputs size must be counted exactly once")
	if result.ChangeOutputsCount > 0 {
		require.Equal(t, result.ChangeOutputsCount*changeOutputSize, collector.changeOutputsSize)
	}
	expectedFee, err := feeCalculator.Calculate(collector.txSize)
	require.NoError(t, err)
	require.Equal(t, expectedFee, result.Fee)
}

// TestCollector_SmallMinimumDesiredUTXOValue_ChangeIsDistributable reproduces the panic
// "Cannot distribute change outputs among given outputs (count: 8) for given amount (343)":
// with minimumDesiredUTXOValue=50 and maxChangeOutputsPerTx=8, the change outputs count was
// chosen from the change before the change outputs' fee, which left too little change after it.
func TestCollector_SmallMinimumDesiredUTXOValue_ChangeIsDistributable(t *testing.T) {
	const (
		txSats         = 1000
		txSize         = 44
		minimumDesired = 50
		desiredUTXOs   = 512
	)

	for _, maxChangeOutputs := range []uint64{1, 3, 8, 512} {
		for _, feeRate := range []int64{1, 100, 1000} {
			feeCalculator := newFeeCalculator(defs.FeeModel{Type: defs.SatPerKB, Value: feeRate})
			for extra := uint64(0); extra <= 2000; extra++ {
				t.Run(fmt.Sprintf("max=%d/fee=%v/extra=%d", maxChangeOutputs, feeRate, extra), func(t *testing.T) {
					// given:
					collector, err := newCollector(txSats, txSize, 1, desiredUTXOs, minimumDesired, feeCalculator, maxChangeOutputs, false)
					require.NoError(t, err)

					inputFee, err := feeCalculator.Calculate(txSize + txutils.P2PKHEstimatedInputSize)
					require.NoError(t, err)

					// when:
					err = collector.allocateUTXO(&models.UserUTXO{
						OutputID:           1,
						Satoshis:           txSats + inputFee.MustUInt64() + extra,
						EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
					})
					require.NoError(t, err)

					if !collector.IsFunded() {
						return
					}
					result, err := collector.GetResult()

					// then:
					require.NoError(t, err)
					requireDistributableChange(t, collector, result, txSize+txutils.P2PKHEstimatedInputSize, feeCalculator)
				})
			}
		}
	}
}

// TestCollector_MultipleAllocations_CountChangeOutputsSizeOnce verifies that recalculating
// change outputs after each allocated UTXO replaces their size instead of adding it again.
func TestCollector_MultipleAllocations_CountChangeOutputsSizeOnce(t *testing.T) {
	const txSize = 44
	inputSize := txutils.P2PKHEstimatedInputSize
	feeCalculator := newFeeCalculator(defs.FeeModel{Type: defs.SatPerKB, Value: 100})

	tests := map[string]struct {
		txSats  satoshi.Value
		isSweep bool
		utxos   []uint64
	}{
		"sweep of many UTXOs": {
			txSats:  0,
			isSweep: true,
			utxos:   []uint64{1000, 2000, 3000, 4000, 5000, 6000},
		},
		"first UTXO leaves change but is not enough after change outputs fee": {
			txSats: 1000,
			utxos:  []uint64{1021, 5000},
		},
		"small UTXOs": {
			txSats:  0,
			isSweep: true,
			utxos:   []uint64{60, 70, 80, 90, 100, 110, 120, 130},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			collector, err := newCollector(test.txSats, txSize, 1, 512, 50, feeCalculator, 8, test.isSweep)
			require.NoError(t, err)

			// when:
			for i, sats := range test.utxos {
				err = collector.allocateUTXO(&models.UserUTXO{OutputID: uint(i + 1), Satoshis: sats, EstimatedInputSize: inputSize})
				require.NoError(t, err)
			}
			result, err := collector.GetResult()

			// then:
			require.NoError(t, err)
			requireDistributableChange(t, collector, result, txSize+uint64(len(test.utxos))*inputSize, feeCalculator)
		})
	}
}

// TestCollector_ReportedCase_Count8Amount343 pins down the reported production case:
// minimumDesiredUTXOValue=50, maxChangeOutputsPerTx=8, 100 sat/KB and a pre-fee change of 370 sats.
// Previously the count (8) was chosen from 370, then 8 change outputs (272 bytes, +27 sats fee)
// dropped the change to 343 < 7*50, and Distribute(8, 343) panicked.
func TestCollector_ReportedCase_Count8Amount343(t *testing.T) {
	// given:
	const (
		txSats = 1000
		txSize = 44
	)
	feeCalculator := newFeeCalculator(defs.FeeModel{Type: defs.SatPerKB, Value: 100})
	collector, err := newCollector(txSats, txSize, 1, 512, 50, feeCalculator, 8, false)
	require.NoError(t, err)

	inputFee, err := feeCalculator.Calculate(txSize + txutils.P2PKHEstimatedInputSize)
	require.NoError(t, err)

	// when:
	err = collector.allocateUTXO(&models.UserUTXO{
		OutputID:           1,
		Satoshis:           txSats + inputFee.MustUInt64() + 370,
		EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
	})
	require.NoError(t, err)
	result, err := collector.GetResult()

	// then:
	require.NoError(t, err)
	require.Equal(t, uint64(7), result.ChangeOutputsCount)
	require.Equal(t, satoshi.Value(347), result.ChangeAmount)

	values, err := txutils.NewChangeDistribution(result.ChangeInitialValue, func(uint64) uint64 { return 0 }).
		Distribute(result.ChangeOutputsCount, result.ChangeAmount)
	require.NoError(t, err)
	require.Equal(t, []satoshi.Value{47, 50, 50, 50, 50, 50, 50}, seq.Collect(values))
}
