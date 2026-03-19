package txutils

import (
	"fmt"
	"iter"
	"testing"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/seq2"
	"github.com/go-softwarelab/common/pkg/seqerr"
	"github.com/stretchr/testify/require"
)

func TestInputSize(t *testing.T) {
	// given:
	unlockingScriptSize := uint64(100)

	// when:
	size := TransactionInputSize(unlockingScriptSize)

	// then:
	require.Equal(t, size, 40+unlockingScriptSize+1)
}

func TestOutputSize(t *testing.T) {
	// given:
	lockingScriptSize := uint64(100)

	// when:
	size := TransactionOutputSize(lockingScriptSize)

	// then:
	require.Equal(t, size, 8+lockingScriptSize+1)
}

func TestTransactionSize(t *testing.T) {
	tests := map[string]struct {
		inputSizes  iter.Seq[uint64]
		outputSizes iter.Seq[uint64]
		expected    uint64
	}{
		"two inputs, two outputs": {
			inputSizes:  seq.Of[uint64](100, 200),
			outputSizes: seq.Of[uint64](300, 400),
			expected: 8 + // tx envelope size
				1 + // varint size of inputs count
				141 + // 40+100+1 // input [0] size
				241 + // 40+200+1 // input [1] size
				1 + // varint size of outputs count
				311 + // 8+300+3// output [0] size
				411, // 8+400+3// output [1] size
		},
		"zero inputs, two outputs": {
			inputSizes:  seq.Of[uint64](),
			outputSizes: seq.Of[uint64](300, 400),
			expected: 8 + // tx envelope size
				1 + // varint size of inputs count
				1 + // varint size of outputs count
				311 + // 8+300+3// output [0] size
				411, // 8+400+3// output [1] size
		},
		"two inputs, zero outputs": {
			inputSizes:  seq.Of[uint64](100, 200),
			outputSizes: seq.Of[uint64](),
			expected: 8 + // tx envelope size
				1 + // varint size of inputs count
				141 + // 40+100+1 // input [0] size
				241 + // 40+200+1 // input [1] size
				1, // varint size of outputs count
		},
		"300 inputs, 400 outputs": {
			inputSizes:  seq.Repeat[uint64](100, 300),
			outputSizes: seq.Repeat[uint64](200, 400),
			expected: 8 + // tx envelope size
				3 + // varint size of inputs count
				300*141 + // 40+100+1 // inputs size
				3 + // varint size of outputs count
				400*209, // 8+300+1// outputs size
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// when:
			size, err := TransactionSize(seqerr.FromSeq(test.inputSizes), seqerr.FromSeq(test.outputSizes))

			// then:
			require.NoError(t, err)
			require.Equal(t, test.expected, size)
		})
	}
}

func TestTransactionSizeWithErrorOnInputs(t *testing.T) {
	// given:
	inputSizes := seqerr.Of[uint64](100, 200)
	inputSizes = seq2.Append(inputSizes, 0, fmt.Errorf("error"))
	outputsSizes := seq.Of[uint64](300, 400)

	// when:
	_, err := TransactionSize(inputSizes, seqerr.FromSeq(outputsSizes))

	// then:
	require.Error(t, err)
}

func TestTransactionSizeWithErrorOnOutputs(t *testing.T) {
	// given:
	inputSizes := seq.Of[uint64](100, 200)
	outputsSizes := seqerr.Of[uint64](300, 400)
	outputsSizes = seq2.Append(outputsSizes, 0, fmt.Errorf("error"))

	// when:
	_, err := TransactionSize(seqerr.FromSeq(inputSizes), outputsSizes)

	// then:
	require.Error(t, err)
}
