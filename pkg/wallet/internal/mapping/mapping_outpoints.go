package mapping

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapIndexesToOutpoints(txID *chainhash.Hash, indexes []int) ([]transaction.Outpoint, error) {
	outpoints, err := slices.MapOrError(indexes, indexToOutpointMapper(txID))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare outpoints from indexes: %w", err)
	}
	return outpoints, nil
}

func indexToOutpointMapper(txID *chainhash.Hash) func(it int) (transaction.Outpoint, error) {
	return func(it int) (transaction.Outpoint, error) {
		index, err := to.UInt32(it)
		if err != nil {
			return transaction.Outpoint{}, fmt.Errorf("invalid transaction output index: %w", err)
		}

		return transaction.Outpoint{
			Txid:  *txID,
			Index: index,
		}, nil
	}
}
