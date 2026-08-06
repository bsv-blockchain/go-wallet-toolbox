package actions_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/actions"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const signReference = "ref-sign"

func newSignAction(t *testing.T, storage *storageStub) (*actions.SignAction, pending.SignActionsRepository) {
	t.Helper()

	cache := pending.NewSignActionLocalRepository(slog.New(slog.DiscardHandler), pending.DefaultPendingSignActionsTTL)

	return &actions.SignAction{
		Logger:                  slog.New(slog.DiscardHandler),
		PendingSignActionsCache: cache,
		Storage:                 storage,
	}, cache
}

// givenPendingSignAction caches an assembled-but-unsigned transaction under signReference,
// mirroring what CreateAction does when the caller asked for a signable transaction.
// When clearTemplates is set, the inputs carry no unlocking script template, as is the
// case for custom inputs that the caller has to unlock itself.
func givenPendingSignAction(t *testing.T, cache pending.SignActionsRepository, clearTemplates bool) {
	t.Helper()

	var createResult wdk.StorageCreateActionResult
	require.NoError(t, json.Unmarshal([]byte(tsgenerated.CreateActionResultJSON()), &createResult))

	priv, err := ec.PrivateKeyFromHex(testusers.Alice.PrivKey)
	require.NoError(t, err)

	assembled, err := assembler.NewCreateActionTransactionAssembler(sdk.NewKeyDeriver(priv), nil, &createResult).Assemble()
	require.NoError(t, err)

	inputBEEF, err := transaction.NewBeefFromBytes(createResult.InputBeef)
	require.NoError(t, err)

	if clearTemplates {
		for _, input := range assembled.Inputs {
			input.UnlockingScriptTemplate = nil
		}
	}

	require.NoError(t, cache.Save(signReference, &pending.SignAction{
		Tx:        *assembled.Transaction,
		InputBEEF: inputBEEF,
	}))
}

func TestSignAction_AbortsWhenPendingActionIsNotCached(t *testing.T) {
	// given: the pending sign action is gone (TTL expiry, restart, ...)
	storage := &storageStub{}
	action, _ := newSignAction(t, storage)

	// when:
	result, err := action.SignAction(context.Background(), sdk.SignActionArgs{
		Reference: []byte(signReference),
	}, testOriginator, newWalletParty())

	// then:
	require.Error(t, err)
	assert.Nil(t, result)

	// and: the unsigned tx left behind by CreateAction is released
	assert.Equal(t, 0, storage.processCalled)
	assert.Equal(t, []string{signReference}, storage.abortCalls)
}

func TestSignAction_AbortsWhenInputsCannotBeUnlocked(t *testing.T) {
	// given: a cached pending action, but the caller sends no unlocking scripts
	storage := &storageStub{}
	action, cache := newSignAction(t, storage)
	givenPendingSignAction(t, cache, true)

	// when:
	_, err := action.SignAction(context.Background(), sdk.SignActionArgs{
		Reference: []byte(signReference),
		Spends:    map[uint32]sdk.SignActionSpend{},
	}, testOriginator, newWalletParty())

	// then:
	require.ErrorContains(t, err, "cannot be unlocked")
	assert.Equal(t, 0, storage.processCalled)
	assert.Equal(t, []string{signReference}, storage.abortCalls)

	// and: the dead reference cannot be retried
	_, cacheErr := cache.Get(signReference)
	require.ErrorIs(t, cacheErr, wdk.ErrNotFoundError)
}

func TestSignAction_AbortsOnInvalidOriginator(t *testing.T) {
	// given:
	storage := &storageStub{}
	action, cache := newSignAction(t, storage)
	givenPendingSignAction(t, cache, false)

	// when: originator is rejected by validation, but the reference is present
	_, err := action.SignAction(context.Background(), sdk.SignActionArgs{
		Reference: []byte(signReference),
	}, "part1..part3", newWalletParty())

	// then:
	require.Error(t, err)
	assert.Equal(t, []string{signReference}, storage.abortCalls)
}

func TestSignAction_DoesNotAbortWithoutReference(t *testing.T) {
	// given: no reference at all - nothing identifies an action to abort
	storage := &storageStub{}
	action, _ := newSignAction(t, storage)

	// when:
	_, err := action.SignAction(context.Background(), sdk.SignActionArgs{}, testOriginator, newWalletParty())

	// then:
	require.Error(t, err)
	assert.Empty(t, storage.abortCalls)
}

func TestSignAction_AbortsOnCanceledContext(t *testing.T) {
	// given:
	storage := &storageStub{}
	action, _ := newSignAction(t, storage)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when:
	_, err := action.SignAction(ctx, sdk.SignActionArgs{
		Reference: []byte(signReference),
	}, testOriginator, newWalletParty())

	// then: the compensating abort still runs on a detached context
	require.Error(t, err)
	assert.Equal(t, []string{signReference}, storage.abortCalls)
}

func TestSignAction_AbortFailureDoesNotMaskOriginalError(t *testing.T) {
	// given:
	storage := &storageStub{abortErr: fmt.Errorf("storage is down")}
	action, _ := newSignAction(t, storage)

	// when:
	_, err := action.SignAction(context.Background(), sdk.SignActionArgs{
		Reference: []byte(signReference),
	}, testOriginator, newWalletParty())

	// then:
	require.ErrorContains(t, err, "get pending sign action failed")
	assert.NotContains(t, err.Error(), "storage is down")
	assert.Equal(t, 1, storage.abortCallCount)
}

func TestSignAction_DoesNotAbortWhenActionReachedProcessAction(t *testing.T) {
	// given: a fully signable pending action and a failing ProcessAction
	storage := &storageStub{processErr: fmt.Errorf("broadcast under review")}
	action, cache := newSignAction(t, storage)
	givenPendingSignAction(t, cache, false)

	// when: no spends are needed, the assembled tx carries unlocking templates for its inputs
	_, err := action.SignAction(context.Background(), sdk.SignActionArgs{
		Reference: []byte(signReference),
	}, testOriginator, newWalletParty())

	// then: the tx may already be known to the network, so its inputs must stay reserved
	require.Error(t, err)
	assert.Equal(t, 1, storage.processCalled)
	assert.Empty(t, storage.abortCalls)
}
