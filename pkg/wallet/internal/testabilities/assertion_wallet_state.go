package testabilities

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
)

type WalletReader interface {
	ListActions(ctx context.Context, args sdk.ListActionsArgs, originator string) (*sdk.ListActionsResult, error)
	ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error)
}

type WalletStateAssertion interface {
	HasActionsCount(expected int, labels ...string) WalletStateAssertion
	ActionAtIndex(index int, labels ...string) WalletActionAssertion
	HasActionsWithStatusCount(expected int, status sdk.ActionStatus) WalletStateAssertion
	WaitForActionsWithStatusCount(expectedCount int, status sdk.ActionStatus, timeout time.Duration)
}

type WalletActionAssertion interface {
	WithStatus(expected sdk.ActionStatus) WalletActionAssertion
	WithDescription(expected string) WalletActionAssertion
	WithLabels(expected ...string) WalletActionAssertion
	WithTxID(expected string) WalletActionAssertion
	WithNotEmptyTxID() WalletActionAssertion
	WithoutTxID() WalletActionAssertion
	WithSatoshis(expected int64) WalletActionAssertion
	OutputAtIndex(index int) WalletActionOutputAssertion
}

type WalletActionOutputAssertion interface {
	WithSatoshis(expected uint64) WalletActionOutputAssertion
	WithLockingScript(expected []byte) WalletActionOutputAssertion
	WithOutputIndex(expected uint32) WalletActionOutputAssertion
	WithTags(expected ...string) WalletActionOutputAssertion
	WithCustomInstructions(expected string) WalletActionOutputAssertion
	WithSpendable(expected bool) WalletActionOutputAssertion
	WithBasket(expected string) WalletActionOutputAssertion
}

func ThenWalletState(t testing.TB, wallet WalletReader) WalletStateAssertion {
	return &walletStateAssertion{
		TB:     t,
		wallet: wallet,
	}
}

type walletStateAssertion struct {
	testing.TB

	wallet WalletReader
}

func (a *walletStateAssertion) HasActionsCount(expected int, labels ...string) WalletStateAssertion {
	a.Helper()
	result := a.listActions(labels...)
	assert.Len(a, result.Actions, expected, "Expected number of transactions does not match")
	assert.Equal(a, expected, int(result.TotalActions), "Total count of transactions does not match")
	return a
}

func (a *walletStateAssertion) getActionsWithStatusCount(status sdk.ActionStatus) int {
	a.Helper()
	result := a.listActions()
	counter := 0
	for _, action := range result.Actions {
		if action.Status == status {
			counter++
		}
	}

	return counter
}

func (a *walletStateAssertion) HasActionsWithStatusCount(expected int, status sdk.ActionStatus) WalletStateAssertion {
	counter := a.getActionsWithStatusCount(status)
	assert.Equal(a, expected, counter, "Expected number of transactions with status %s does not match", status)
	return a
}

func (a *walletStateAssertion) WaitForActionsWithStatusCount(expectedCount int, status sdk.ActionStatus, timeout time.Duration) {
	a.Helper()

	condition := func() bool {
		current := a.getActionsWithStatusCount(status)
		return current == expectedCount
	}

	assert.Eventually(a, condition, timeout, 500*time.Millisecond, "Expected %d actions with status %s", expectedCount, status)
}

func (a *walletStateAssertion) ActionAtIndex(index int, labels ...string) WalletActionAssertion {
	a.Helper()
	result := a.listActions(labels...)
	require.Greater(a, len(result.Actions), index, "Index out of range")

	return &walletActionAssertion{
		TB:     a.TB,
		wallet: a.wallet,
		action: &result.Actions[index],
	}
}

func (a *walletStateAssertion) listActions(labels ...string) *sdk.ListActionsResult {
	a.Helper()
	args := fixtures.DefaultWalletListActionsArgsWithIncludes()
	args.Limit = to.Ptr[uint32](validate.MaxPaginationLimit)
	args.Labels = labels
	result, err := a.wallet.ListActions(a.Context(), args, fixtures.DefaultOriginator)
	require.NoError(a, err, "Failed to list actions")
	return result
}

type walletActionAssertion struct {
	testing.TB

	wallet WalletReader
	action *sdk.Action
}

func (a *walletActionAssertion) WithNotEmptyTxID() WalletActionAssertion {
	a.Helper()
	assert.NotEmpty(a, a.action.Txid, "Action tx should not be empty")
	return a
}

func (a *walletActionAssertion) WithStatus(expected sdk.ActionStatus) WalletActionAssertion {
	a.Helper()
	assert.Equal(a, expected, a.action.Status, "Action status does not match")
	return a
}

func (a *walletActionAssertion) WithDescription(expected string) WalletActionAssertion {
	a.Helper()
	assert.Equal(a, expected, a.action.Description, "Action description does not match")
	return a
}

func (a *walletActionAssertion) WithLabels(expected ...string) WalletActionAssertion {
	a.Helper()
	assert.GreaterOrEqual(a, len(a.action.Labels), len(expected), "Label count does not match")
	for i, label := range expected {
		assert.Contains(a, a.action.Labels[i], label, "Action label does not contain label")
	}
	return a
}

func (a *walletActionAssertion) WithTxID(expected string) WalletActionAssertion {
	a.Helper()
	assert.Equal(a, expected, a.action.Txid.String(), "Action transaction ID does not match")
	return a
}

func (a *walletActionAssertion) WithoutTxID() WalletActionAssertion {
	a.Helper()
	var zeroHash chainhash.Hash
	assert.Equal(a, zeroHash, a.action.Txid, "Action transaction ID should be empty")
	return a
}

func (a *walletActionAssertion) WithSatoshis(expected int64) WalletActionAssertion {
	a.Helper()
	assert.Equal(a, expected, a.action.Satoshis, "Action satoshis does not match")
	return a
}

func (a *walletActionAssertion) OutputAtIndex(index int) WalletActionOutputAssertion {
	a.Helper()
	require.Greater(a, len(a.action.Outputs), index, "Index out of range for action outputs")

	return &walletActionOutputAssertion{
		TB:     a.TB,
		output: &a.action.Outputs[index],
		txID:   a.action.Txid,
		wallet: a.wallet,
	}
}

type walletActionOutputAssertion struct {
	testing.TB

	wallet WalletReader
	output *sdk.ActionOutput
	txID   chainhash.Hash
}

func (a *walletActionOutputAssertion) WithSatoshis(expected uint64) WalletActionOutputAssertion {
	a.Helper()
	assert.Equal(a, expected, a.output.Satoshis, "Action output satoshis does not match")
	return a
}

func (a *walletActionOutputAssertion) WithLockingScript(expected []byte) WalletActionOutputAssertion {
	a.Helper()
	expectedLockingScript := script.NewFromBytes(expected)

	assert.Equal(a, expectedLockingScript.Bytes(), a.output.LockingScript, "Action output locking script does not match")
	return a
}

func (a *walletActionOutputAssertion) WithOutputIndex(expected uint32) WalletActionOutputAssertion {
	a.Helper()
	assert.Equal(a, expected, a.output.OutputIndex, "Action output index does not match")
	return a
}

func (a *walletActionOutputAssertion) WithTags(expected ...string) WalletActionOutputAssertion {
	a.Helper()
	assert.GreaterOrEqual(a, len(a.output.Tags), len(expected), "Tag count does not match")
	for i, tag := range expected {
		assert.Contains(a, a.output.Tags[i], tag, "Action output tag does not contain tag")
	}

	return a
}

func (a *walletActionOutputAssertion) WithCustomInstructions(expected string) WalletActionOutputAssertion {
	a.Helper()
	assert.Equal(a, expected, a.output.CustomInstructions, "Action output custom instructions do not match")
	return a
}

func (a *walletActionOutputAssertion) WithSpendable(expected bool) WalletActionOutputAssertion {
	a.Helper()
	assert.Equal(a, expected, a.output.Spendable, "Action output spendable does not match")
	if a.output.Spendable {
		a.listActionsAlignsListOutputs()
	}
	return a
}

func (a *walletActionOutputAssertion) WithBasket(expected string) WalletActionOutputAssertion {
	a.Helper()
	assert.Equal(a, expected, a.output.Basket, "Action output basket does not match")
	return a
}

func (a *walletActionOutputAssertion) listActionsAlignsListOutputs() WalletActionOutputAssertion {
	// NOTE: ListOutputs returns only outputs that are spendable, so we need to ensure that the action output is also spendable.
	listedOutputs, err := a.wallet.ListOutputs(a.Context(), sdk.ListOutputsArgs{
		Limit:                     to.Ptr[uint32](validate.MaxPaginationLimit),
		IncludeCustomInstructions: to.Ptr(true),
		IncludeTags:               to.Ptr(true),
		IncludeLabels:             to.Ptr(true),
		Include:                   sdk.OutputIncludeLockingScripts,
	}, fixtures.DefaultOriginator)
	require.NoError(a, err, "Failed to list outputs")

	assert.Equal(a, 1, seq.Count(seq.Filter(seq.FromSlice(listedOutputs.Outputs), func(output sdk.Output) bool {
		return a.compareActionOutputAndOutput(a.txID, a.output, &output)
	})), "list outputs does not align with action outputs")

	return a
}

func (a *walletActionOutputAssertion) compareActionOutputAndOutput(txID chainhash.Hash, actionOutput *sdk.ActionOutput, output *sdk.Output) bool {
	var zeroHash chainhash.Hash
	// NOTE: Not processed/signed transactions may have empty txid.
	canCompareOutpoints := !output.Outpoint.Txid.IsEqual(&zeroHash) && !txID.Equal(zeroHash)
	if canCompareOutpoints {
		if !a.equalOutpoints(output.Outpoint, txID.String(), actionOutput.OutputIndex) {
			return false
		}
	}

	return satoshi.MustEqual(output.Satoshis, actionOutput.Satoshis) &&
		output.Spendable == actionOutput.Spendable &&
		a.equalTags(output.Tags, actionOutput.Tags) &&
		bytes.Equal(output.LockingScript, actionOutput.LockingScript) &&
		output.CustomInstructions == actionOutput.CustomInstructions
}

func (a *walletActionOutputAssertion) equalOutpoints(outpoint transaction.Outpoint, txid string, outputIndex uint32) bool {
	return outpoint.Txid.String() == txid && outpoint.Index == outputIndex
}

func (a *walletActionOutputAssertion) equalTags(tags1, tags2 []string) bool {
	if len(tags1) != len(tags2) {
		return false
	}

	slices.Sort(tags1)
	slices.Sort(tags2)

	return slices.Equal(tags1, tags2)
}
