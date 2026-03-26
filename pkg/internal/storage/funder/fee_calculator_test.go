package funder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// TestFeeCalculatorPerBytePrecision tests that the fee calculator uses per-byte precision
// instead of rounding up to the nearest kilobyte before fee calculation.
func TestFeeCalculatorPerBytePrecision(t *testing.T) {
	tests := []struct {
		name          string
		txSize        uint64
		satoshisPerKB int64
		expectedFee   int64
		description   string
	}{
		{
			name:          "240 bytes at 100 sats/KB",
			txSize:        240,
			satoshisPerKB: 100,
			expectedFee:   24,
			description:   "240/1000 * 100 = 24 - verifies per-byte precision, not per-KB rounding",
		},
		{
			name:          "240 bytes at 1 sat/KB",
			txSize:        240,
			satoshisPerKB: 1,
			expectedFee:   1,
			description:   "240/1000 * 1 = 0.24, ceil = 1",
		},
		{
			name:          "240 bytes at 10 sats/KB",
			txSize:        240,
			satoshisPerKB: 10,
			expectedFee:   3,
			description:   "240/1000 * 10 = 2.4, ceil = 3",
		},
		{
			name:          "250 bytes at 500 sats/KB",
			txSize:        250,
			satoshisPerKB: 500,
			expectedFee:   125,
			description:   "250/1000 * 500 = 125",
		},
		{
			name:          "1000 bytes at 100 sats/KB",
			txSize:        1000,
			satoshisPerKB: 100,
			expectedFee:   100,
			description:   "1000/1000 * 100 = 100",
		},
		{
			name:          "1500 bytes at 100 sats/KB",
			txSize:        1500,
			satoshisPerKB: 100,
			expectedFee:   150,
			description:   "1500/1000 * 100 = 150",
		},
		{
			name:          "1500 bytes at 500 sats/KB",
			txSize:        1500,
			satoshisPerKB: 500,
			expectedFee:   750,
			description:   "1500/1000 * 500 = 750",
		},
		{
			name:          "small transaction - 44 bytes at 1 sat/KB",
			txSize:        44,
			satoshisPerKB: 1,
			expectedFee:   1,
			description:   "44/1000 * 1 = 0.044, ceil = 1",
		},
		{
			name:          "small transaction - 100 bytes at 1 sat/KB",
			txSize:        100,
			satoshisPerKB: 1,
			expectedFee:   1,
			description:   "100/1000 * 1 = 0.1, ceil = 1",
		},
		{
			name:          "typical transaction - 192 bytes at 1 sat/KB",
			txSize:        192,
			satoshisPerKB: 1,
			expectedFee:   1,
			description:   "192/1000 * 1 = 0.192, ceil = 1",
		},
		{
			name:          "typical transaction - 192 bytes at 50 sats/KB",
			txSize:        192,
			satoshisPerKB: 50,
			expectedFee:   10,
			description:   "192/1000 * 50 = 9.6, ceil = 10",
		},
		{
			name:          "large transaction - 100000 bytes at 1 sat/KB",
			txSize:        100000,
			satoshisPerKB: 1,
			expectedFee:   100,
			description:   "100000/1000 * 1 = 100",
		},
		{
			name:          "edge case - 1 byte at 1 sat/KB",
			txSize:        1,
			satoshisPerKB: 1,
			expectedFee:   1,
			description:   "1/1000 * 1 = 0.001, ceil = 1",
		},
		{
			name:          "edge case - 1 byte at 1000 sats/KB",
			txSize:        1,
			satoshisPerKB: 1000,
			expectedFee:   1,
			description:   "1/1000 * 1000 = 1",
		},
		{
			name:          "999 bytes at 1 sat/KB",
			txSize:        999,
			satoshisPerKB: 1,
			expectedFee:   1,
			description:   "999/1000 * 1 = 0.999, ceil = 1",
		},
		{
			name:          "1001 bytes at 1 sat/KB",
			txSize:        1001,
			satoshisPerKB: 1,
			expectedFee:   2,
			description:   "1001/1000 * 1 = 1.001, ceil = 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feeModel := defs.FeeModel{
				Type:  defs.SatPerKB,
				Value: tt.satoshisPerKB,
			}
			calc := newFeeCalculator(feeModel)

			fee, err := calc.Calculate(tt.txSize)
			require.NoError(t, err, tt.description)
			require.Equal(t, tt.expectedFee, fee.Int64(), tt.description)
		})
	}
}

// TestFeeCalculatorRegressionSmallIncrements tests that fee calculation
// is precise for small increments of transaction size.
func TestFeeCalculatorRegressionSmallIncrements(t *testing.T) {
	feeModel := defs.FeeModel{
		Type:  defs.SatPerKB,
		Value: 1,
	}
	calc := newFeeCalculator(feeModel)

	// Test that incrementing transaction size by 1 byte doesn't cause unexpected jumps
	// For 1 sat/KB, fees should only increment when crossing a 1000-byte boundary
	testCases := []struct {
		txSize      uint64
		expectedFee int64
	}{
		{1, 1},
		{100, 1},
		{500, 1},
		{999, 1},
		{1000, 1},
		{1001, 2},
		{1100, 2},
		{1500, 2},
		{1999, 2},
		{2000, 2},
		{2001, 3},
	}

	for _, tc := range testCases {
		fee, err := calc.Calculate(tc.txSize)
		require.NoError(t, err)
		require.Equal(t, tc.expectedFee, fee.Int64(),
			"Fee for %d bytes should be %d sats", tc.txSize, tc.expectedFee)
	}
}

// TestFeeCalculatorRegressionHighFeeRate tests fee calculation with higher fee rates
// to ensure per-byte precision is maintained across different rates.
func TestFeeCalculatorRegressionHighFeeRate(t *testing.T) {
	feeModel := defs.FeeModel{
		Type:  defs.SatPerKB,
		Value: 50,
	}
	calc := newFeeCalculator(feeModel)

	testCases := []struct {
		txSize      uint64
		expectedFee int64
	}{
		{10, 1},    // 10/1000 * 50 = 0.5, ceil = 1
		{20, 1},    // 20/1000 * 50 = 1.0
		{21, 2},    // 21/1000 * 50 = 1.05, ceil = 2
		{100, 5},   // 100/1000 * 50 = 5
		{192, 10},  // 192/1000 * 50 = 9.6, ceil = 10
		{240, 12},  // 240/1000 * 50 = 12
		{1000, 50}, // 1000/1000 * 50 = 50
		{1001, 51}, // 1001/1000 * 50 = 50.05, ceil = 51
	}

	for _, tc := range testCases {
		fee, err := calc.Calculate(tc.txSize)
		require.NoError(t, err)
		require.Equal(t, tc.expectedFee, fee.Int64(),
			"Fee for %d bytes at 50 sats/KB should be %d sats", tc.txSize, tc.expectedFee)
	}
}

// TestFeeCalculatorSVNodeAlignment tests that the fee calculator aligns with SV Node logic
// by verifying that fees are directly proportional to transaction size without additional scaling.
func TestFeeCalculatorSVNodeAlignment(t *testing.T) {
	// Test with the minimum typical fee rate
	feeModel := defs.FeeModel{
		Type:  defs.SatPerKB,
		Value: 1,
	}
	calc := newFeeCalculator(feeModel)

	// Verify that doubling the transaction size approximately doubles the fee
	// (accounting for ceiling rounding)
	size1 := uint64(1000)
	size2 := uint64(2000)

	fee1, err := calc.Calculate(size1)
	require.NoError(t, err)
	fee2, err := calc.Calculate(size2)
	require.NoError(t, err)

	// For 1 sat/KB: 1000 bytes = 1 sat, 2000 bytes = 2 sats
	require.Equal(t, int64(1), fee1.Int64())
	require.Equal(t, int64(2), fee2.Int64())
	require.Equal(t, fee1.Int64()*2, fee2.Int64(), "Fee should scale linearly with size")
}

// TestFeeCalculatorErrorHandling tests error cases for the fee calculator.
func TestFeeCalculatorErrorHandling(t *testing.T) {
	t.Run("negative fee rate panics", func(t *testing.T) {
		require.Panics(t, func() {
			feeModel := defs.FeeModel{
				Type:  defs.SatPerKB,
				Value: -1,
			}
			newFeeCalculator(feeModel)
		})
	})

	t.Run("unsupported fee model type panics", func(t *testing.T) {
		require.Panics(t, func() {
			feeModel := defs.FeeModel{
				Type:  "unsupported",
				Value: 1,
			}
			newFeeCalculator(feeModel)
		})
	})

	t.Run("zero transaction size", func(t *testing.T) {
		feeModel := defs.FeeModel{
			Type:  defs.SatPerKB,
			Value: 1,
		}
		calc := newFeeCalculator(feeModel)

		fee, err := calc.Calculate(0)
		require.NoError(t, err)
		require.Equal(t, int64(0), fee.Int64())
	})
}

// TestFeeCalculatorConsistencyWithExistingTests verifies that the new implementation
// doesn't break existing test expectations in sql_test.go.
func TestFeeCalculatorConsistencyWithExistingTests(t *testing.T) {
	feeModel := defs.FeeModel{
		Type:  defs.SatPerKB,
		Value: 1,
	}
	calc := newFeeCalculator(feeModel)

	// Test cases from sql_test.go
	testCases := []struct {
		name        string
		txSize      uint64
		expectedFee int64
	}{
		{
			name:        "smallTransactionSize (44 bytes)",
			txSize:      44,
			expectedFee: 1, // 44/1000 * 1 = 0.044, ceil = 1
		},
		{
			name:        "transactionSizeForHigherFee (1001 bytes)",
			txSize:      1001,
			expectedFee: 2, // 1001/1000 * 1 = 1.001, ceil = 2
		},
		{
			name:        "990 bytes (from test 'adding change increases the fee')",
			txSize:      990,
			expectedFee: 1, // 990/1000 * 1 = 0.99, ceil = 1
		},
		{
			name:        "999 bytes (from test 'adding change will increase the fee')",
			txSize:      999,
			expectedFee: 1, // 999/1000 * 1 = 0.999, ceil = 1
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fee, err := calc.Calculate(tc.txSize)
			require.NoError(t, err)
			require.Equal(t, tc.expectedFee, fee.Int64())
		})
	}
}
