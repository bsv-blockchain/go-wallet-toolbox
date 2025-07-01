package testabilities

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type WalletReader interface {
	ListActions(ctx context.Context, args sdk.ListActionsArgs, originator string) (*sdk.ListActionsResult, error)
}

type WalletStateAssertion interface {
	HasActionsCount(expected int, labels ...string) WalletStateAssertion
	ActionAtIndex(index int, labels ...string) WalletActionAssertion
}

type WalletActionAssertion interface {
	WithDescription(expected string) WalletActionAssertion
	WithLabels(expected ...string) WalletActionAssertion
	WithTxID(expected string) WalletActionAssertion
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

func (a *walletStateAssertion) ActionAtIndex(index int, labels ...string) WalletActionAssertion {
	a.Helper()
	result := a.listActions(labels...)
	require.Greater(a, len(result.Actions), index, "Index out of range")

	return &walletActionAssertion{
		TB:     a.TB,
		action: &result.Actions[index],
	}
}

func (a *walletStateAssertion) listActions(labels ...string) *sdk.ListActionsResult {
	a.Helper()
	args := fixtures.DefaultWalletListActionsArgsWithIncludes()
	args.Limit = validate.MaxPaginationLimit
	args.Labels = labels
	result, err := a.wallet.ListActions(a.Context(), args, fixtures.DefaultOriginator)
	require.NoError(a, err, "Failed to list actions")
	return result
}

type walletActionAssertion struct {
	testing.TB
	action *sdk.Action
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
	assert.Equal(a, zeroHash, a.action.Txid , "Action transaction ID should be empty")
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
	}
}

type walletActionOutputAssertion struct {
	testing.TB
	output *sdk.ActionOutput
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
	return a
}

func (a *walletActionOutputAssertion) WithBasket(expected string) WalletActionOutputAssertion {
	a.Helper()
	assert.Equal(a, expected, a.output.Basket, "Action output basket does not match")
	return a
}
