package actions

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type WalletStorageCreateAction interface {
	CreateAction(ctx context.Context, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error)
	ProcessAction(ctx context.Context, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error)
}

type CreateAction struct {
	KeyDeriver *wallet.KeyDeriver
	Storage    WalletStorageCreateAction
	WalletOpts *wallet_opts.Flags

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

func (a *CreateAction) handleNotNewTX(context.Context) (*wallet.CreateActionResult, error) {
	return nil, fmt.Errorf("CreateAction is not yet fully implemented")
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

func (a *CreateAction) handleSignAction(tx *transaction.Transaction, createActionResult *wdk.StorageCreateActionResult) (*wallet.CreateActionResult, error) {
	// TODO: store createActionResult and args by reference in some "cache" or "map" to enable signAction reusing it
	//  unfortunately there is no possibility to restore changes unlocking script from storage, because listAction and listOutputs doesn't return derivation pre/su-fixes
	//  Consider storing only transaction and args by reference as this looks like enough data for process action

	result, err := mapping.SignableTransactionResult(tx, a.wdkArgs, createActionResult)
	if err != nil {
		return nil, fmt.Errorf("failed to build signable transaction: %w", err)
	}
	return result, nil
}

func (a *CreateAction) handleProcessAction(ctx context.Context, tx *transaction.Transaction, createActionResult *wdk.StorageCreateActionResult) (*wallet.CreateActionResult, error) {
	txID := tx.TxID()

	processActionArgs := mapping.MapProcessActionArgs(txID, tx, createActionResult.Reference, a.wdkArgs)

	processActionResult, err := a.Storage.ProcessAction(ctx, processActionArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process created action: %w", err)
	}

	err = a.validateProcessActionResult(processActionResult)
	if err != nil {
		return nil, err
	}

	result, err := mapping.MapCreateActionResultFromStorageResults(txID, tx, createActionResult, processActionResult, a.wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build result after processing created action: %w", err)
	}

	return result, nil
}

func (a *CreateAction) validateProcessActionResult(processActionResult *wdk.ProcessActionResult) error {
	if a.requiresNotDelayedResult() {
		err := validate.NotDelayedProcessActionResult(processActionResult)
		if err != nil {
			return fmt.Errorf("failed on create action not delayed processing, %w", err)
		}
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
