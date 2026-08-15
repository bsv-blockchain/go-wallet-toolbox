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

	// release releases the storage-side action if this flow cannot complete it.
	release *release
}

func (s *SignAction) SignAction(ctx context.Context, args wallet.SignActionArgs, originator string, wp *party.WalletParty) (result *wallet.SignActionResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Wallet-SignAction", attribute.String("originator", originator))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	s.Logger = logging.Child(s.Logger, "SignAction")
	s.originator = originator
	s.reference = string(args.Reference) // TODO: Make sure, the type []byte is a good choice for this field. I have doubts.

	// The action was created (and its inputs reserved) by an earlier CreateAction and only
	// this flow can complete it, so any failure before it reaches ProcessAction releases it.
	// The cached pending action goes with it: the reference is dead afterwards.
	s.release = newRelease(s.Logger, s.Storage)
	s.release.onRelease = s.dropPendingSignAction
	s.release.arm(s.reference)
	defer func() { s.release.onError(ctx, err) }()

	if err = s.validate(); err != nil {
		return nil, err
	}

	pendingSignAction, err := s.PendingSignActionsCache.Get(s.reference)
	if err != nil {
		return nil, fmt.Errorf("get pending sign action failed: %w", err)
	}

	s.mergeArgs(pendingSignAction.CreateActionArgs, args)

	s.tx = assembler.NewAssembledTxFromPendingSignAction(pendingSignAction)

	s.attachUnlockingScripts(args)
	if err = s.allInputsCanBeUnlocked(); err != nil {
		return nil, fmt.Errorf("not all inputs can be unlocked: %w", err)
	}

	if err = s.tx.Sign(); err != nil {
		return nil, fmt.Errorf("sign transaction failed: %w", err)
	}

	s.txID = s.tx.TxID()
	processActionResult, err := s.handleProcessAction(ctx)
	if err != nil {
		return nil, err
	}

	result, err = mapping.MapSignActionResultFromStorageResultsForNewTx(s.txID, s.tx, processActionResult, s.wdkArgs)
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
		tx, verifyErr := party.VerifyReturnedTxIDOnlyAtomicBEEF(ctx, wp.BeefParty, result.Txid, result.Tx)
		if verifyErr != nil {
			err = fmt.Errorf("failed to verify returned BEEF from storage: %w", verifyErr)
			return nil, err
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

// dropPendingSignAction removes the cached pending action after its storage-side action was
// released: the reference is dead, so a retry with it can only fail.
func (s *SignAction) dropPendingSignAction() {
	if err := s.PendingSignActionsCache.Delete(s.reference); err != nil {
		s.Logger.Warn("failed to delete pending sign action from cache after abort",
			slog.String("reference", s.reference),
			logging.Error(err))
	}
}

func (s *SignAction) handleProcessAction(ctx context.Context) (*wdk.ProcessActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Wallet-SignAction-handleProcessAction")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	processActionArgs := mapping.MapProcessActionArgsForNewTx(s.txID, s.tx, s.reference, s.wdkArgs)

	// Point of no return: storage takes over from here and may broadcast the transaction, so
	// this wallet must not release its inputs anymore.
	s.release.disarm()

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
