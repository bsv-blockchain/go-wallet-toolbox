package funder

import (
	"fmt"
	"math"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/go-softwarelab/common/pkg/to"
)

type feeCalc struct {
	bytes float64
	value float64
}

func newFeeCalculator(model defs.FeeModel) *feeCalc {
	if model.Type != defs.SatPerKB {
		panic("unsupported fee model")
	}

	if model.Value < 0 {
		panic("fee model value cannot be negative")
	}

	feeValue, err := to.Float64(model.Value)
	if err != nil {
		panic("invalid fee model value: " + err.Error())
	}

	return &feeCalc{
		value: feeValue,
		bytes: 1000,
	}
}

// Calculate computes the transaction fee based on the transaction size in bytes.
// The fee is calculated using per-byte precision to align with SV Node's fee calculation method.
// 
// Formula: fee = ceil(txSize / 1000 * satoshisPerKB)
//
// This ensures accurate fee calculation for transactions of any size, particularly important
// for smaller transactions where rounding to the nearest kilobyte would result in overpayment.
// For example, a 240-byte transaction at 100 sats/KB:
//   - Correct (per-byte): ceil(240/1000 * 100) = ceil(24) = 24 sats
//   - Incorrect (per-KB):  ceil(240/1000) * 100 = 1 * 100 = 100 sats
func (f *feeCalc) Calculate(txSize uint64) (satoshi.Value, error) {
	size, err := to.Float64FromUnsigned(txSize)
	if err != nil {
		return 0, fmt.Errorf("invalid transaction size: %w", err)
	}

	// Calculate fee with per-byte precision: ceil(size / bytes * feeRate)
	// This maintains precision before rounding, ensuring fees are proportional to actual size
	feeAmount := math.Ceil(size / f.bytes * f.value)

	fee, err := to.Int64(feeAmount)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate fee value: %w", err)
	}

	sats, err := satoshi.From(fee)
	if err != nil {
		return 0, fmt.Errorf("failed to convert fee to satoshi: %w", err)
	}

	return sats, nil
}
