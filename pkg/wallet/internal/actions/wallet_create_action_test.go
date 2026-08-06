package actions_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/actions"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/party"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const testOriginator = "test.originator.com"

// storageStub records calls and lets each test decide what CreateAction/ProcessAction return.
type storageStub struct {
	createResult *wdk.StorageCreateActionResult
	createErr    error

	processErr error

	abortCalls     []string
	abortErr       error
	processCalled  int
	createCalled   int
	abortCallCount int
}

func (s *storageStub) CreateAction(_ context.Context, _ wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error) {
	s.createCalled++
	return s.createResult, s.createErr
}

func (s *storageStub) ProcessAction(_ context.Context, _ wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	s.processCalled++
	if s.processErr != nil {
		return nil, s.processErr
	}
	return &wdk.ProcessActionResult{}, nil
}

func (s *storageStub) AbortAction(_ context.Context, args wdk.AbortActionArgs) (*wdk.AbortActionResult, error) {
	s.abortCallCount++
	s.abortCalls = append(s.abortCalls, string(args.Reference))
	if s.abortErr != nil {
		return nil, s.abortErr
	}
	return &wdk.AbortActionResult{Aborted: true}, nil
}

func newCreateAction(storage *storageStub) *actions.CreateAction {
	return &actions.CreateAction{
		Logger:                  slog.New(slog.DiscardHandler),
		KeyDeriver:              nil,
		Storage:                 storage,
		WalletOpts:              &wallet_opts.Flags{},
		PendingSignActionsCache: pending.NewSignActionLocalRepository(slog.New(slog.DiscardHandler), pending.DefaultPendingSignActionsTTL),
	}
}

func newWalletParty() *party.WalletParty {
	return &party.WalletParty{
		UserParty:    "user",
		StorageParty: "storage",
		BeefParty:    wdk.NewBeefParty([]string{"user", "storage"}),
	}
}

func createActionArgs() sdk.CreateActionArgs {
	return sdk.CreateActionArgs{
		Description: "test action",
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     []byte{0x76, 0xa9, 0x14},
				Satoshis:          1000,
				OutputDescription: "output",
			},
		},
	}
}

// unassemblableResult carries a corrupted inputBEEF, so the flow fails in the assembler,
// i.e. after storage.CreateAction succeeded and before anything reaches ProcessAction.
func unassemblableResult(reference string) *wdk.StorageCreateActionResult {
	return &wdk.StorageCreateActionResult{
		Reference: reference,
		InputBeef: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
}

func TestCreateAction_AbortsUnsignedTxWhenFlowFailsAfterCreate(t *testing.T) {
	// given:
	const reference = "ref-to-abort"
	storage := &storageStub{createResult: unassemblableResult(reference)}
	action := newCreateAction(storage)

	// when:
	result, err := action.CreateAction(context.Background(), createActionArgs(), testOriginator, newWalletParty())

	// then:
	require.Error(t, err)
	assert.Nil(t, result)

	// and: the created-but-never-signed tx is aborted, releasing its reserved inputs
	assert.Equal(t, 1, storage.createCalled)
	assert.Equal(t, 0, storage.processCalled)
	assert.Equal(t, []string{reference}, storage.abortCalls)
}

func TestCreateAction_AbortsOnCanceledContext(t *testing.T) {
	// given: a context that is already canceled when the failure happens
	const reference = "ref-canceled"
	storage := &storageStub{createResult: unassemblableResult(reference)}
	action := newCreateAction(storage)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when:
	_, err := action.CreateAction(ctx, createActionArgs(), testOriginator, newWalletParty())

	// then: the compensating abort still runs on a detached context
	require.Error(t, err)
	assert.Equal(t, []string{reference}, storage.abortCalls)
}

func TestCreateAction_AbortFailureDoesNotMaskOriginalError(t *testing.T) {
	// given:
	storage := &storageStub{
		createResult: unassemblableResult("ref-abort-fails"),
		abortErr:     fmt.Errorf("storage is down"),
	}
	action := newCreateAction(storage)

	// when:
	_, err := action.CreateAction(context.Background(), createActionArgs(), testOriginator, newWalletParty())

	// then: caller sees the original failure, not the abort failure
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "storage is down")
	assert.Equal(t, 1, storage.abortCallCount)
}

func TestCreateAction_DoesNotAbortWhenActionReachedProcessAction(t *testing.T) {
	// given: a create result that assembles and signs, so the flow reaches ProcessAction
	var createResult wdk.StorageCreateActionResult
	require.NoError(t, json.Unmarshal([]byte(tsgenerated.CreateActionResultJSON()), &createResult))

	storage := &storageStub{
		createResult: &createResult,
		processErr:   fmt.Errorf("broadcast under review"),
	}

	priv, err := ec.PrivateKeyFromHex(testusers.Alice.PrivKey)
	require.NoError(t, err)

	action := newCreateAction(storage)
	action.KeyDeriver = sdk.NewKeyDeriver(priv)

	// when:
	_, err = action.CreateAction(context.Background(), createActionArgs(), testOriginator, newWalletParty())

	// then: the tx may already carry broadcast evidence, so we must not release its inputs
	require.Error(t, err)
	assert.Equal(t, 1, storage.processCalled)
	assert.Empty(t, storage.abortCalls)
}

func TestCreateAction_DoesNotAbortWhenStorageCreateActionFailed(t *testing.T) {
	// given: nothing was reserved, because CreateAction itself failed
	storage := &storageStub{createErr: fmt.Errorf("not enough funds")}
	action := newCreateAction(storage)

	// when:
	_, err := action.CreateAction(context.Background(), createActionArgs(), testOriginator, newWalletParty())

	// then:
	require.Error(t, err)
	assert.Empty(t, storage.abortCalls)
}
