package funder

import (
	"context"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Funder interface {
	// Fund
	// @param targetSat - the target amount of satoshis to fund (total inputs - total outputs)
	// @param currentTxSize - the current size of the transaction in bytes (size of tx + current inputs + current outputs)
	// @param numberOfDesiredUTXOs - the number of UTXOs in basket #TakeFromBasket
	// @param minimumDesiredUTXOValue - the minimum value of UTXO in basket #TakeFromBasket
	// @param userID - the user ID
	// @param forbiddenOutputIDs - the list of output IDs that should not be used in the funding process
	Fund(ctx context.Context, targetSat satoshi.Value, currentTxSize uint64, basket *wdk.TableOutputBasket, userID int, forbiddenOutputIDs []uint) (*Result, error)
}
