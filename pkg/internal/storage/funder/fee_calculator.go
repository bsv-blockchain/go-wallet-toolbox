package funder

import (
	"fmt"
	"math"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
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

func (f *feeCalc) Calculate(txSize uint64) (satoshi.Value, error) {
	size, err := to.Float64FromUnsigned(txSize)
	if err != nil {
		return 0, fmt.Errorf("invalid transaction size: %w", err)
	}

	multiplier := math.Ceil(size / f.bytes)

	fee, err := to.Int64(multiplier * f.value)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate fee value: %w", err)
	}

	sats, err := satoshi.From(fee)
	if err != nil {
		return 0, fmt.Errorf("failed to convert fee to satoshi: %w", err)
	}

	return sats, nil
}
