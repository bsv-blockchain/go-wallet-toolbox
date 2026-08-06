package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"

	pkgerrors "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/party"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type SignAction struct {
	Logger                  *slog.Logger
	PendingSignActionsCache pending.SignActionsRepository
	Storage                 WalletStorageProcessAndAbortAction

	wdkArgs    wdk.ValidCreateActionArgs
	reference  string
	tx         *assembler.AssembledTransaction
	txID       *chainhash.Hash
	originator string

	// processActionAttempted records whether the signed transaction was already handed
	// to storage.ProcessAction. Once it was, the action may carry broadcast evidence and
	// must not be aborted by this wallet as a compensating action.
	processActionAttempted bool
}

func (s *SignAction) SignAction(ctx context.Context, args wallet.SignActionArgs, originator string, wp *party.WalletParty) (*wallet.SignActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-SignAction", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	s.Logger = logging.Child(s.Logger, "SignAction")
	s.originator = originator
	s.reference = string(args.Reference) // TODO: Make sure, the type []byte is a good choice for this field. I have doubts.

	err = s.validate()
	if err != nil {
		return nil, s.failBeforeProcess(ctx, err)
	}

	pendingSignAction, err := s.PendingSignActionsCache.Get(s.reference)
	if err != nil {
		return nil, s.failBeforeProcess(ctx, fmt.Errorf("get pending sign action failed: %w", err))
	}

	s.mergeArgs(pendingSignAction.CreateActionArgs, args)

	s.tx = assembler.NewAssembledTxFromPendingSignAction(pendingSignAction)

	s.attachUnlockingScripts(args)
	if err = s.allInputsCanBeUnlocked(); err != nil {
		return nil, s.failBeforeProcess(ctx, fmt.Errorf("not all inputs can be unlocked: %w", err))
	}

	err = s.tx.Sign()
	if err != nil {
		return nil, s.failBeforeProcess(ctx, fmt.Errorf("sign transaction failed: %w", err))
	}

	s.txID = s.tx.TxID()
	processActionResult, err := s.handleProcessAction(ctx)
	if err != nil {
		return nil, err
	}

	result, err := mapping.MapSignActionResultFromStorageResultsForNewTx(s.txID, s.tx, processActionResult, s.wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to build result after processing signed action: %w",
			pkgerrors.NewTransactionError(*s.txID).
				Wrap(newProcessActionError(processActionResult, s.txID, s.tx, nil).
					Wrap(err)))
	}

	err = s.PendingSignActionsCache.Delete(s.reference)
	if err != nil {
		s.Logger.WarnContext(ctx, "failed to delete pending sign action from cache",
			slog.String("reference", s.reference),
			slog.String("txID", s.txID.String()),
			logging.Error(err))
	}

	if result.Tx != nil && s.wdkArgs.Options.ReturnTXIDOnly.Value() {
		tx, err := party.VerifyReturnedTxIDOnlyAtomicBEEF(wp.BeefParty, result.Txid, result.Tx)
		if err != nil {
			return nil, fmt.Errorf("failed to verify returned BEEF from storage: %w", err)
		}

		result.Tx = tx
	}

	return result, nil
}

func (s *SignAction) attachUnlockingScripts(args wallet.SignActionArgs) {
	for vin, spends := range args.Spends {
		unlockingScript := script.NewFromBytes(spends.UnlockingScript)
		s.tx.Inputs[vin].UnlockingScript = unlockingScript

		if spends.SequenceNumber != nil {
			s.tx.Inputs[vin].SequenceNumber = *spends.SequenceNumber
		}
	}
}

// failBeforeProcess handles a SignAction failure that happened before the transaction
// reached storage.ProcessAction. The action created by CreateAction is still parked in
// storage as 'unsigned' and nobody will complete it now, so its reserved inputs are
// released right away instead of waiting for the fail_abandoned sweep. The pending sign
// action is dropped as well, so the dead reference cannot be retried.
// The passed error is returned unchanged.
func (s *SignAction) failBeforeProcess(ctx context.Context, err error) error {
	if s.reference == "" {
		// Nothing identifies the action (rejected by validate), so there is nothing to abort.
		return err
	}

	if s.processActionAttempted {
		// Defensive: the transaction may already carry broadcast evidence, releasing its
		// inputs here could double-spend it.
		s.Logger.WarnContext(ctx, "skipping compensating abort: action was already sent to ProcessAction",
			slog.String("reference", s.reference),
			logging.Error(err))
		return err
	}

	abortActionAfterFailure(ctx, s.Logger, s.Storage, s.reference, err)

	if deleteErr := s.PendingSignActionsCache.Delete(s.reference); deleteErr != nil {
		s.Logger.WarnContext(ctx, "failed to delete pending sign action from cache after abort",
			slog.String("reference", s.reference),
			logging.Error(deleteErr))
	}

	return err
}

func (s *SignAction) handleProcessAction(ctx context.Context) (*wdk.ProcessActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-SignAction-handleProcessAction")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	processActionArgs := mapping.MapProcessActionArgsForNewTx(s.txID, s.tx, s.reference, s.wdkArgs)

	s.processActionAttempted = true

	processActionResult, err := s.Storage.ProcessAction(ctx, processActionArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process signed action (txID: %s, reference: %s): %w",
			s.txID.String(), s.reference, err)
	}

	if s.requiresNotDelayedResult() {
		err = validate.NotDelayedProcessActionResult(processActionResult)
		if err != nil {
			// Attach AtomicBEEF so callers can recover the signed tx from the review error
			// (TypeScript WERR_REVIEW_ACTIONS carries txid + tx).
			return nil, newProcessActionError(processActionResult, s.txID, s.tx, nil).Wrap(err)
		}
	}

	return processActionResult, nil
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
		s.wdkArgs.IsDelayed = *args.Options.AcceptDelayedBroadcast
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

func (s *SignAction) allInputsCanBeUnlocked() error {
	var missingInputVin []int
	for vin, input := range s.tx.Inputs {
		switch {
		case input.UnlockingScript != nil && len(*input.UnlockingScript) != 0:
			continue
		case input.UnlockingScriptTemplate != nil:
			continue
		default:
			missingInputVin = append(missingInputVin, vin)
		}
	}

	if len(missingInputVin) > 0 {
		return fmt.Errorf("the following inputs cannot be unlocked (missing unlocking script and unlocking script template) input indexes: %v", missingInputVin)
	}
	return nil
}

func (s *SignAction) validate() error {
	if err := validate.Originator(s.originator); err != nil {
		return fmt.Errorf("invalid originator: %w", err)
	}

	if len(s.reference) == 0 {
		return fmt.Errorf("missing reference argument for sign action")
	}

	return nil
}
