package funder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
)

// TestFuelInputThreshold finds the smallest FuelKeeper fuel output that pays a
// pushdrop transaction's fee on its own (the funder then allocates nothing from
// the change basket), at the production fee rate of 100 sat/kB.
func TestFuelInputThreshold(t *testing.T) {
	const (
		fuelUnlockLen     = 33 // FuelKeeper puzzle: push32 <preimage>
		pushdropUnlockLen = 73 // push of a DER signature with sighash byte
		tokenSats         = 1
	)
	feeCalculator := newFeeCalculator(defs.FeeModel{Type: defs.SatPerKB, Value: 100})

	minFuel := func(inputUnlockLens, outputLockLens []uint64, tokenInSats, tokenOutSats int64) (fuel int64) {
		txSize := txutils.TransactionSizeFromScriptLengths(inputUnlockLens, outputLockLens)
		for fuel = 1; fuel <= 1000; fuel++ {
			c, err := newCollector(satoshi.Value(tokenOutSats-tokenInSats-fuel), txSize, uint64(len(outputLockLens)), 32, 1000, feeCalculator, 8, false)
			require.NoError(t, err)
			if c.IsFunded() {
				return fuel
			}
		}
		t.Fatal("no fuel up to 1000 sats covers the fee")
		return 0
	}

	for _, dataLen := range []uint64{64, 300} {
		lockLen := 1 + 33 + 1 + 3 + dataLen + 1 // <pubkey> OP_CHECKSIG <data> OP_DROP
		mint := minFuel([]uint64{fuelUnlockLen}, []uint64{lockLen}, 0, tokenSats)
		transfer := minFuel([]uint64{fuelUnlockLen, pushdropUnlockLen}, []uint64{lockLen}, tokenSats, tokenSats)
		burn := minFuel([]uint64{fuelUnlockLen, pushdropUnlockLen}, nil, tokenSats, 0)
		t.Logf("pushdrop data %3d bytes: min fuel mint=%d transfer=%d burn=%d sats", dataLen, mint, transfer, burn)

		// A 100-sat fuel output covers every shape. 50 covers mint and transfer,
		// but not burn: with no outputs of its own, a burn needs a change output
		// of at least the dust floor, on top of the fee.
		require.LessOrEqual(t, max(mint, transfer, burn), int64(100))
		require.LessOrEqual(t, max(mint, transfer), int64(55))
		require.Greater(t, burn, int64(50))
	}
}
