package funder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
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
