package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/wallet"
	pkgerrors "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

type SignAction struct {
	Logger                  *slog.Logger
	PendingSignActionsCache wdk.PendingSignActionsCache
	Storage                 WalletStorageProcessAction

	wdkArgs wdk.ValidCreateActionArgs
}

func (s *SignAction) SignAction(ctx context.Context, args wallet.SignActionArgs, originator string) (*wallet.SignActionResult, error) {
	s.Logger = logging.Child(s.Logger, "SignAction")
	reference := string(args.Reference) // TODO: Make sure, the type []byte is a good choice for this field. I have doubts.

	pendingSignAction, err := s.PendingSignActionsCache.Get(reference)
	if err != nil {
		return nil, fmt.Errorf("get pending sign action failed: %w", err)
	}

	tx := &pendingSignAction.Tx

	err = tx.Sign()
	if err != nil {
		return nil, fmt.Errorf("sign transaction failed: %w", err)
	}

	s.mergeArgs(pendingSignAction.CreateActionArgs, args)

	txID := tx.TxID()

	processActionArgs := mapping.MapProcessActionArgsForNewTx(txID, tx, reference, s.wdkArgs)

	processActionResult, err := s.Storage.ProcessAction(ctx, processActionArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process signed action (txID: %s, reference: %s): %w",
			txID.String(), reference, err)
	}

	if s.requiresNotDelayedResult() {
		err = validate.NotDelayedProcessActionResult(processActionResult)
		if err != nil {
			return nil, pkgerrors.NewProcessActionError(processActionResult.SendWithResults, processActionResult.NotDelayedResults).Wrap(err)
		}
	}

	result, err := mapping.MapSignActionResultFromStorageResultsForNewTx(txID, tx, processActionResult, s.wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build result after processing signed action (txID: %s, reference: %s): %w",
			txID.String(), reference, err)
	}

	err = s.PendingSignActionsCache.Delete(reference)
	if err != nil {
		s.Logger.Warn("failed to delete pending sign action from cache",
			slog.String("reference", reference),
			slog.String("txID", txID.String()),
			logging.Error(err))
	}

	return result, nil
}

func (s *SignAction) requiresNotDelayedResult() bool {
	return !s.wdkArgs.IsDelayed
}

func (s *SignAction) mergeArgs(createActionArgs wdk.ValidCreateActionArgs, args wallet.SignActionArgs) {
	s.wdkArgs = createActionArgs

	if args.Options == nil {
		return
	}

	if args.Options.AcceptDelayedBroadcast != nil {
		s.wdkArgs.Options.AcceptDelayedBroadcast = to.Ptr(primitives.BooleanDefaultTrue(*args.Options.AcceptDelayedBroadcast))
	}
	if args.Options.ReturnTXIDOnly != nil {
		s.wdkArgs.Options.ReturnTXIDOnly = to.Ptr(primitives.BooleanDefaultFalse(*args.Options.ReturnTXIDOnly))
	}
	if args.Options.NoSend != nil {
		s.wdkArgs.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(*args.Options.NoSend))
		s.wdkArgs.IsNoSend = *args.Options.NoSend
	}
	if args.Options.SendWith != nil {
		s.wdkArgs.Options.SendWith = slices.Map(args.Options.SendWith, func(s chainhash.Hash) primitives.TXIDHexString { return primitives.TXIDHexString(s.String()) })
		s.wdkArgs.IsSendWith = len(args.Options.SendWith) > 0
	}
}
