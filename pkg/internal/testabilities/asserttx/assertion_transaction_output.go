package asserttx

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
)

type outputAssertion struct {
	testing.TB

	index  int
	output *transaction.TransactionOutput
}

func (a *outputAssertion) HasLockingScript(expected []byte) BSVOutputAssertion {
	if a.output == nil {
		return a
	}
	expectedLockingScript := script.NewFromBytes(expected)

	lockingScript := to.Value(a.output.LockingScript)

	assert.Equalf(a, expectedLockingScript.String(), lockingScript.String(), "expect output %d to have the same locking script", a.index)
	return a
}

func (a *outputAssertion) HasSatoshis(expectedSats uint64) BSVOutputAssertion {
	if a.output == nil {
		return a
	}
	assert.Equalf(a, expectedSats, a.output.Satoshis, "unexpected satoshis value of output %d", a.index)
	return a
}

func (a *outputAssertion) IsChange() {
	if a.output == nil {
		return
	}
	assert.True(a, a.output.Change, "expect output %d to be change", a.index)
}

func (a *outputAssertion) IsNotChange() {
	if a.output == nil {
		return
	}
	assert.False(a, a.output.Change, "expect output %d to not be change", a.index)
}

func (a *outputAssertion) HasChangeFlag(expectedIsChange bool) {
	if a.output == nil {
		return
	}
	assert.Equalf(a, expectedIsChange, a.output.Change, "unexpected change flag on output %d", a.index)
}
