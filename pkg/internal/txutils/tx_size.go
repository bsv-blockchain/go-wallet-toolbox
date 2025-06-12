package txutils

import (
	"fmt"
	"iter"

	sdk "github.com/bsv-blockchain/go-sdk/util"
)

const (
	txEnvelopeSize = 4 + 4 // version + locktime
	satoshisSize   = 8
	inputConstSize = 32 + 4 + 4 // txID + vout + sequence
)

// TransactionInputSize calculates the size in bytes of a transaction input
// with the given script size
func TransactionInputSize(scriptSize uint64) uint64 {
	return inputConstSize +
		varIntSize(scriptSize) +
		scriptSize
}

// TransactionOutputSize calculates the serialized byte length of a transaction output
// with the given script size in bytes
func TransactionOutputSize(scriptSize uint64) uint64 {
	return varIntSize(scriptSize) +
		scriptSize +
		satoshisSize
}

// TransactionSize calculates the total size of a transaction in bytes
// inputs is a sequence of input script sizes (and possibly error)
// outputs is a sequence of output script sizes (and possibly error)
func TransactionSize(inputSizes iter.Seq2[uint64, error], outputSizes iter.Seq2[uint64, error]) (uint64, error) {
	var inputsCount uint64
	var inputsSize uint64
	for scriptSize, err := range inputSizes {
		if err != nil {
			return 0, fmt.Errorf("failed to calculate unlocking script size: %w", err)
		}
		inputsCount++
		inputsSize += TransactionInputSize(scriptSize)
	}

	var outputsCount uint64
	var outputsSize uint64
	for scriptSize, err := range outputSizes {
		if err != nil {
			return 0, fmt.Errorf("failed to calculate locking script size: %w", err)
		}
		outputsCount++
		outputsSize += TransactionOutputSize(scriptSize)
	}

	return txEnvelopeSize +
			varIntSize(inputsCount) + // Sizeof value of number of inputs
			inputsSize + // All inputs accumulated size
			varIntSize(outputsCount) + // Sizeof value of number of outputs
			outputsSize, // All outputs accumulated size
		nil
}

func varIntSize(val uint64) uint64 {
	length := sdk.VarInt(val).Length()
	return toU64(length)
}

//nolint:gosec // No need to check for overflows from int to uint64 here
func toU64(val int) uint64 {
	return uint64(val)
}
