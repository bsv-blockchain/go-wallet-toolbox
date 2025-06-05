package asserttx

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type OutputMatcher interface {
	MatchesOutput(index int, output *transaction.TransactionOutput) bool
}

type InputMatcher interface {
	MatchesInput(index int, input *transaction.TransactionInput) bool
}

type Index int

func (i Index) MatchesOutput(index int, _ *transaction.TransactionOutput) bool {
	return index == int(i)
}

func (i Index) MatchesInput(index int, _ *transaction.TransactionInput) bool {
	return index == int(i)
}

type BSVTransactionAssertion interface {
	HasInputsThatFundsOutputs() BSVTransactionAssertion
	HasMinimalFee() BSVTransactionAssertion
	Inputs() BSVInputsAssertion
	Outputs() BSVOutputsAssertion
	Output(index int) BSVOutputAssertion
}

type BSVInputsAssertion interface {
	AllHaveUnlockingScript() BSVInputsAssertion
	HasTotalInputValue(sats int) BSVInputsAssertion
}

type BSVOutputsAssertion interface {
	AllHaveLockingScript()
}

type BSVOutputAssertion interface {
	HasLockingScript(script []byte) BSVOutputAssertion
	HasSatoshis(sats uint64) BSVOutputAssertion
	IsChange()
	IsNotChange()
	HasChangeFlag(bool)
}

func RestoredFromBEEFBytes(t testing.TB, bytes []byte) BSVTransactionAssertion {
	tx, err := transaction.NewTransactionFromBEEF(bytes)
	require.NoError(t, err, "BEEF bytes must be parseable to transaction")

	return &txAssertion{
		TB: t,
		tx: tx,
	}
}

type txAssertion struct {
	testing.TB
	tx *transaction.Transaction
}

func (t *txAssertion) AllHaveLockingScript() {
	for i, output := range t.tx.Outputs {
		lockingScript := to.Value(output.LockingScript)
		assert.NotEmptyf(t, lockingScript, "output %d must contain locking script", i)
	}
}

func (t *txAssertion) HasInputsThatFundsOutputs() BSVTransactionAssertion {
	totalInputs, ok := t.totalInputSatoshis()
	if !ok {
		return t
	}
	totalOutputs := t.tx.TotalOutputSatoshis()
	assert.GreaterOrEqualf(t, totalInputs, totalOutputs, "expect transaction outputs to be funded with inputs")
	return t
}

func (t *txAssertion) HasMinimalFee() BSVTransactionAssertion {
	totalInputs, ok := t.totalInputSatoshis()
	if !ok {
		return t
	}

	size := t.tx.Size()
	var expectedFee = must.ConvertToUInt64(to.IfThen(size%1000 == 0, size/1000).ElseThen(size/1000 + 1))

	totalOutputs := t.tx.TotalOutputSatoshis()

	fee := totalInputs - totalOutputs

	assert.Equal(t, expectedFee, fee, "expect transaction to have minimal valid value of fee")
	return t
}

func (t *txAssertion) Inputs() BSVInputsAssertion {
	return t
}

func (t *txAssertion) AllHaveUnlockingScript() BSVInputsAssertion {
	for i, input := range t.tx.Inputs {
		if assert.NotNil(t, input.SourceTransaction) {
			for j, inputParent := range input.SourceTransaction.Inputs {
				assert.NotEmptyf(t, inputParent.UnlockingScript, "expect tx input %d to contain unlocking script for its input %d", i, j)
			}
		}
	}
	return t
}

func (t *txAssertion) HasTotalInputValue(sats int) BSVInputsAssertion {
	totalInputs, ok := t.totalInputSatoshis()
	if !ok {
		return t
	}

	assert.EqualValues(t, sats, int(totalInputs), "expect transaction to have total input value")
	return t
}

func (t *txAssertion) Outputs() BSVOutputsAssertion {
	return t
}

func (t *txAssertion) Output(index int) BSVOutputAssertion {
	output := t.tx.OutputIdx(index)

	assert.NotNil(t, output, "expect transaction to have output at index %d", index)
	return &outputAssertion{
		TB:     t.TB,
		index:  index,
		output: output,
	}
}

func (t *txAssertion) totalInputSatoshis() (totalInputs uint64, ok bool) {
	totalInputs, err := t.tx.TotalInputSatoshis()
	return totalInputs, assert.NoError(t, err, "expect transaction to have all inputs data needed to calculate total inputs value")
}
