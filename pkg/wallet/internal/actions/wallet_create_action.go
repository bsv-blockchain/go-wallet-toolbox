package actions

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/wallet"
	broadcastError "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type CreateAction struct {
	KeyDeriver              *wallet.KeyDeriver
	Storage                 WalletStorageCreateAndProcessAction
	WalletOpts              *wallet_opts.Flags
	PendingSignActionsCache wdk.PendingSignActionsCache

	wdkArgs    wdk.ValidCreateActionArgs
	originator string
}

func (a *CreateAction) CreateAction(ctx context.Context, args wallet.CreateActionArgs, originator string) (*wallet.CreateActionResult, error) {
	// TODO: mapping.MapCreateActionArgs should handle known tx ids - needs some merging and validation of BEEF
	a.originator = originator
	a.wdkArgs = mapping.MapCreateActionArgs(args, *a.WalletOpts)

	if err := a.validate(); err != nil {
		return nil, err
	}

	if a.isNotNewTX() {
		return a.handleNotNewTX(ctx)
	}
	return a.handleNewTX(ctx, args)
	// TODO: merge BEEF Party ??
}

func (a *CreateAction) handleNotNewTX(ctx context.Context) (*wallet.CreateActionResult, error) {
	processActionArgs := mapping.MapProcessActionArgsForSendWith(a.wdkArgs)
	processActionResult, err := a.Storage.ProcessAction(ctx, processActionArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process created action: %w", err)
	}

	broadcastErr := a.validateProcessActionResult(processActionResult)
	if broadcastErr != nil {
		return nil, broadcastErr
	}

	result, err := mapping.MapCreateActionResultFromStorageResultsForSendWith(processActionResult)
	if err != nil {
		return nil, fmt.Errorf("failed to build result after processing created action: %w", err)
	}

	return result, nil
}

func (a *CreateAction) handleNewTX(ctx context.Context, args wallet.CreateActionArgs) (*wallet.CreateActionResult, error) {
	createActionResult, err := a.Storage.CreateAction(ctx, a.wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to create action: %w", err)
	}

	txAssembler := assembler.NewCreateActionTransactionAssembler(a.KeyDeriver, args.Inputs, createActionResult)

	tx, err := txAssembler.Assemble()
	if err != nil {
		return nil, fmt.Errorf("failed to assemble transaction from storage response: %w", err)
	}

	if a.isSignAction() {
		return a.handleSignAction(tx, createActionResult)
	}

	err = tx.Sign()
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return a.handleProcessAction(ctx, tx, createActionResult)
}

func (a *CreateAction) handleSignAction(tx *assembler.AssembledTransaction, createActionResult *wdk.StorageCreateActionResult) (*wallet.CreateActionResult, error) {
	result, err := mapping.SignableTransactionResult(tx, a.wdkArgs, createActionResult)
	if err != nil {
		return nil, fmt.Errorf("failed to build signable transaction: %w", err)
	}

	err = a.PendingSignActionsCache.Set(createActionResult.Reference, &wdk.PendingSignAction{
		Tx:               *tx,
		CreateActionArgs: a.wdkArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to cache pending sign action (reference: %s): %w", createActionResult.Reference, err)
	}

	return result, nil
}

func (a *CreateAction) handleProcessAction(ctx context.Context, tx *assembler.AssembledTransaction, createActionResult *wdk.StorageCreateActionResult) (*wallet.CreateActionResult, error) {
	txID := tx.TxID()

	processActionArgs := mapping.MapProcessActionArgsForNewTx(txID, tx, createActionResult.Reference, a.wdkArgs)

	processActionResult, err := a.Storage.ProcessAction(ctx, processActionArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process created action (txID: %s, reference: %s): %w",
			txID.String(), createActionResult.Reference, err)
	}

	broadcastErr := a.validateProcessActionResult(processActionResult)
	if broadcastErr != nil {
		return nil, broadcastErr
	}

	result, err := mapping.MapCreateActionResultFromStorageResultsForNewTx(txID, tx, createActionResult, processActionResult, a.wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build result after processing created action (txID: %s, reference: %s): %w",
			txID.String(), createActionResult.Reference, err)
	}

	return result, nil
}

func (a *CreateAction) validateProcessActionResult(processActionResult *wdk.ProcessActionResult) *broadcastError.BroadcastingError {
	if a.requiresNotDelayedResult() {
		return validate.NotDelayedProcessActionResult(processActionResult)
	}
	return nil
}

func (a *CreateAction) requiresNotDelayedResult() bool {
	return !a.wdkArgs.IsDelayed
}

func (a *CreateAction) isSignAction() bool {
	return a.wdkArgs.IsSignAction
}

func (a *CreateAction) isNotNewTX() bool {
	return !a.wdkArgs.IsNewTx
}

func (a *CreateAction) validate() error {
	if err := validate.Originator(a.originator); err != nil {
		return fmt.Errorf("invalid originator: %w", err)
	}

	if err := validate.WalletCreateActionArgs(&a.wdkArgs); err != nil {
		return fmt.Errorf("invalid create action args: %w", err)
	}
	return nil
}
