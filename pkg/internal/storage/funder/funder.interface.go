package funder

import (
	"context"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
)

type Funder interface {
	// Fund
	// @param targetSat - the target amount of satoshis to fund (total inputs - total outputs)
	// @param currentTxSize - the current size of the transaction in bytes (size of tx + current inputs + current outputs)
	// @param numberOfDesiredUTXOs - the number of UTXOs in basket #TakeFromBasket
	// @param minimumDesiredUTXOValue - the minimum value of UTXO in basket #TakeFromBasket
	// @param userID - the user ID.
	// @param forbiddenOutputIDs - defines the output IDs that should not be used as sources to cover the target satoshis value.
	// @param priorityOutputs - defines the outputs that should be used as source to cover the target satoshi value before fetching the required number of outputs from database.
	// @param includeSending - defines whether to include currently sending outputs in the basket.
	Fund(
		ctx context.Context,
		targetSat satoshi.Value,
		currentTxSize uint64,
		basket *entity.OutputBasket,
		userID int,
		forbiddenOutputIDs []uint,
		priorityOutputs []*entity.Output,
		includeSending bool,
	) (*Result, error)
}
