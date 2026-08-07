package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/wallet"

	pkgerrors "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/party"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type CreateAction struct {
	Logger                  *slog.Logger
	KeyDeriver              *wallet.KeyDeriver
	Storage                 WalletStorageCreateAndProcessAction
	WalletOpts              *wallet_opts.Flags
	PendingSignActionsCache pending.SignActionsRepository

	// WdkArgsMutator, when set, adjusts the mapped wdk args before validation.
	// It carries toolbox extensions that have no representation in the BRC-100
	// sdk args (e.g. fuel fan-out shaping).
	WdkArgsMutator func(*wdk.ValidCreateActionArgs)

	wdkArgs    wdk.ValidCreateActionArgs
	originator string

	// release releases the storage-side action if this flow cannot complete it.
	release *release
}

func (a *CreateAction) CreateAction(ctx context.Context, args wallet.CreateActionArgs, originator string, wp *party.WalletParty) (result *wallet.CreateActionResult, err error) {
	a.Logger = logging.Child(a.Logger, "CreateAction")
	a.originator = originator

	// storage.CreateAction reserves inputs for the action it creates. Until this flow hands
	// the action to ProcessAction, it is the only one that can complete it - so any failure
	// in between must give those inputs back.
	a.release = newRelease(a.Logger, a.Storage)
	defer func() { a.release.onError(ctx, err) }()
	a.wdkArgs = mapping.MapCreateActionArgs(args, *a.WalletOpts)
	if a.WdkArgsMutator != nil {
		a.WdkArgsMutator(&a.wdkArgs)
	}

	if a.wdkArgs.Options.KnownTxids == nil {
		knownTxIDs, knownErr := wp.GetKnownTxIDs()
		if knownErr != nil {
			return nil, fmt.Errorf("failed to get known txids for auto known txids: %w", knownErr)
		}

		a.wdkArgs.Options.KnownTxids = knownTxIDs
	}

	if err = a.validate(); err != nil {
		return nil, err
	}

	if a.isNotNewTX() {
		return a.handleNotNewTX(ctx)
	}

	result, err = a.handleNewTX(ctx, args)
	if err != nil {
		return nil, err
	}

	if result.Tx == nil {
		return result, nil
	}

	if err = wp.BeefParty.MergeBeefFromParty(wp.StorageParty, result.Tx); err != nil {
		return nil, fmt.Errorf("failed to merge returned BEEF from storage: %w", err)
	}

	if a.wdkArgs.Options.ReturnTXIDOnly.Value() {
		tx, verifyErr := party.VerifyReturnedTxIDOnlyAtomicBEEF(wp.BeefParty, result.Txid, result.Tx, a.wdkArgs.Options.KnownTxids...)
		if verifyErr != nil {
			err = fmt.Errorf("failed to verify returned BEEF from storage: %w", verifyErr)
			return nil, err
		}

		result.Tx = tx
	}

	return result, nil
}

func (a *CreateAction) handleNotNewTX(ctx context.Context) (*wallet.CreateActionResult, error) {
	processActionArgs := mapping.MapProcessActionArgsForSendWith(a.wdkArgs)
	processActionResult, err := a.Storage.ProcessAction(ctx, processActionArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process created action: %w", err)
	}

	broadcastErr := a.validateProcessActionResult(processActionResult)
	if broadcastErr != nil {
		// sendWith-only path has no new transaction / noSendChange (matches TS WERR_REVIEW_ACTIONS optionals).
		return nil, pkgerrors.NewProcessActionError(processActionResult.SendWithResults, processActionResult.NotDelayedResults).Wrap(broadcastErr)
	}

	result, err := mapping.MapCreateActionResultFromStorageResultsForSendWith(processActionResult)
	if err != nil {
		return nil, fmt.Errorf("failed to build result after processing created action: %w", err)
	}

	return result, nil
}

func (a *CreateAction) handleNewTX(ctx context.Context, args wallet.CreateActionArgs) (*wallet.CreateActionResult, error) {
	storageCreateActionResult, err := a.Storage.CreateAction(ctx, a.wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to create action: %w", err)
	}

	// The action now exists in storage with its inputs reserved.
	a.release.arm(storageCreateActionResult.Reference)

	createActionResult, err := a.handleCreatedNewTx(ctx, args, storageCreateActionResult)
	if err != nil {
		return nil, pkgerrors.NewCreateActionError(storageCreateActionResult.Reference).Wrap(err)
	}

	return createActionResult, nil
}

func (a *CreateAction) handleCreatedNewTx(ctx context.Context, args wallet.CreateActionArgs, storageCreateActionResult *wdk.StorageCreateActionResult) (*wallet.CreateActionResult, error) {
	txAssembler := assembler.NewCreateActionTransactionAssembler(a.KeyDeriver, args.Inputs, storageCreateActionResult)

	tx, err := txAssembler.Assemble()
	if err != nil {
		return nil, fmt.Errorf("failed to assemble transaction from storage response: %w", err)
	}

	if a.isSignAction() {
		return a.handleSignAction(tx, storageCreateActionResult)
	}

	err = tx.Sign()
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	processActionResult, err := a.handleProcessAction(ctx, tx, storageCreateActionResult)
	if err != nil {
		return nil, pkgerrors.NewTransactionError(*tx.TxID()).Wrap(err)
	}

	result, err := mapping.MapCreateActionResultFromStorageResultsForNewTx(tx.TxID(), tx, storageCreateActionResult, processActionResult, a.wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build result after processing created action: %w", pkgerrors.NewTransactionError(*tx.TxID()).Wrap(err))
	}

	return result, nil
}

func (a *CreateAction) handleSignAction(tx *assembler.AssembledTransaction, storageCreateActionResult *wdk.StorageCreateActionResult) (*wallet.CreateActionResult, error) {
	txAtomic, err := tx.ToAtomicBEEF(false)
	if err != nil {
		return nil, fmt.Errorf("failed to build atomic beef from assembled transaction: %w", err)
	}

	result, err := mapping.SignableTransactionResult(tx.TxID(), txAtomic, a.wdkArgs, storageCreateActionResult)
	if err != nil {
		return nil, fmt.Errorf("failed to build signable transaction: %w", err)
	}

	err = a.PendingSignActionsCache.Save(storageCreateActionResult.Reference, &pending.SignAction{
		Tx:               *tx.Transaction,
		CreateActionArgs: a.wdkArgs,
		InputBEEF:        txAtomic,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to cache pending sign action (reference: %s): %w", storageCreateActionResult.Reference, err)
	}

	return result, nil
}

func (a *CreateAction) handleProcessAction(ctx context.Context, tx *assembler.AssembledTransaction, createActionResult *wdk.StorageCreateActionResult) (*wdk.ProcessActionResult, error) {
	txID := tx.TxID()

	processActionArgs := mapping.MapProcessActionArgsForNewTx(txID, tx, createActionResult.Reference, a.wdkArgs)

	// Point of no return: storage takes over from here and may broadcast the transaction, so
	// this wallet must not release its inputs anymore.
	a.release.disarm()

	processActionResult, err := a.Storage.ProcessAction(ctx, processActionArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process created action: %w", err)
	}

	broadcastErr := a.validateProcessActionResult(processActionResult)
	if broadcastErr != nil {
		// Attach AtomicBEEF + noSendChange so callers can recover from review-required broadcasts
		// (TypeScript WERR_REVIEW_ACTIONS carries txid, tx, noSendChange).
		return nil, newProcessActionError(
			processActionResult,
			txID,
			tx,
			createActionResult.NoSendChangeOutputVouts,
		).Wrap(broadcastErr)
	}

	return processActionResult, nil
}

func (a *CreateAction) validateProcessActionResult(processActionResult *wdk.ProcessActionResult) error {
	if a.requiresNotDelayedResult() {
		err := validate.NotDelayedProcessActionResult(processActionResult)
		if err != nil {
			return fmt.Errorf("not delayed result required but missing: %w", err)
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
