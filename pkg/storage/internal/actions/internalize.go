package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

type OutputToInternalize struct {
	*entity.NewOutput

	existingOutputID *uint
}

type internalize struct {
	logger             *slog.Logger
	txRepo             TransactionsRepo
	basketRepo         BasketRepo
	knownTxRepo        KnownTxRepo
	outputRepo         OutputRepo
	random             wdk.Randomizer
	beefVerifier       wdk.BeefVerifier
	scriptsVerifier    wdk.ScriptsVerifier
	blockHeaderService wdk.BlockHeaderLoader
	// process is used to reuse ProcessAction's broadcast path (BackgroundBroadcast) for unknown unproven txs.
	process *process
	uow     UnitOfWork
}

func newInternalizeAction(
	logger *slog.Logger,
	txRepo TransactionsRepo,
	basketRepo BasketRepo,
	knownTxRepo KnownTxRepo,
	outputRepo OutputRepo,
	uow UnitOfWork,
	random wdk.Randomizer,
	beefVerifier wdk.BeefVerifier,
	scriptsVerifier wdk.ScriptsVerifier,
	blockHeader wdk.BlockHeaderLoader,
	processAction *process,
) *internalize {
	logger = logging.Child(logger, "internalizeAction")
	return &internalize{
		logger:             logger,
		txRepo:             txRepo,
		basketRepo:         basketRepo,
		knownTxRepo:        knownTxRepo,
		outputRepo:         outputRepo,
		uow:                uow,
		random:             random,
		beefVerifier:       beefVerifier,
		scriptsVerifier:    scriptsVerifier,
		blockHeaderService: blockHeader,
		process:            processAction,
	}
}

func (in *internalize) Internalize(ctx context.Context, userID int, args *wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-Internalize", attribute.Int("userID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	in.logger.DebugContext(
		ctx, "Starting internalize action",
		logging.UserID(userID),
		slog.Int("txBeefSize", len(args.Tx)),
		slog.Int("outputsCount", len(args.Outputs)),
		slog.String("description", string(args.Description)),
	)

	beef, txIDHash, err := transaction.NewBeefFromAtomicBytes(args.Tx)
	if err != nil {
		return nil, fmt.Errorf("failed to create atomic beef from bytes: %w", err)
	}

	in.logger.DebugContext(
		ctx, "Verifying beef transaction",
		logging.UserID(userID),
		slog.String("txID", txIDHash.String()),
		slog.String("description", string(args.Description)),
	)

	ok, err := in.beefVerifier.VerifyBeef(ctx, beef, false)
	if err != nil {
		return nil, fmt.Errorf("failed to verify beef: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("provided beef is not valid")
	}

	// hydrate txs in beef
	if err = txutils.HydrateBEEF(beef); err != nil {
		return nil, fmt.Errorf("failed to hydrate beef for script verification: %w", err)
	}

	// verify scripts for all unmined transactions in BEEF
	for txIDHash, beefTx := range beef.Transactions {
		// there shouldn't happen a situation when transaction will be nil in beef
		if beefTx.Transaction == nil {
			return nil, fmt.Errorf("failed to find raw tx inside beef, txHash: %s", txIDHash.String())
		}
		// skip already mined txs
		if beefTx.Transaction.MerklePath != nil {
			continue
		}

		var txScriptsOk bool
		txScriptsOk, err = in.scriptsVerifier.VerifyScripts(ctx, beefTx.Transaction)
		if err != nil {
			return nil, fmt.Errorf("script verification failed for tx %s : %w", txIDHash, err)
		}
		if !txScriptsOk {
			return nil, fmt.Errorf("scripts are not valid for tx %s", txIDHash)
		}
	}

	tx := beef.FindAtomicTransactionByHash(txIDHash)
	if tx == nil {
		return nil, fmt.Errorf("atomic beef error: transaction with hash %s not found", txIDHash)
	}

	txID := txIDHash.String()

	in.logger.DebugContext(
		ctx, "BEEF verification completed successfully",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.String("description", string(args.Description)),
	)

	in.logger.DebugContext(
		ctx, "Checking for existing transaction",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.String("description", string(args.Description)),
	)
	var outputs []*OutputToInternalize
	var cumulativeSatoshis satoshi.Value
	var isMerge bool
	var shouldBroadcast bool

	err = in.uow.Do(ctx, func(txCtx context.Context, repos Providers) error {
		var uowErr error
		storedTx, uowErr := repos.TransactionsRepo().FindTransactionByUserIDAndTxID(txCtx, userID, txID)
		if uowErr != nil {
			return fmt.Errorf("failed to find transaction by userID and txID: %w", uowErr)
		}

		isMerge = storedTx != nil

		if isMerge {
			in.logger.DebugContext(
				txCtx, "Transaction already exists - performing merge",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.String("existingStatus", string(storedTx.Status)),
				slog.String("description", string(args.Description)),
			)
		} else {
			in.logger.DebugContext(
				txCtx, "New transaction - creating fresh entry",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.String("description", string(args.Description)),
			)
		}

		if isMerge && !in.isAllowedMergeStatus(storedTx.Status) {
			return fmt.Errorf("target transaction of internalizeAction has invalid status: %q", storedTx.Status)
		}

		in.logger.DebugContext(
			txCtx, "Processing outputs",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.Int("outputsToProcess", len(args.Outputs)),
			slog.Bool("isMerge", isMerge),
			slog.String("description", string(args.Description)),
		)

		outputs, cumulativeSatoshis, uowErr = in.makeOutputs(txCtx, userID, tx, args.Outputs, isMerge, repos)
		if uowErr != nil {
			return fmt.Errorf("failed to create new outputs: %w", uowErr)
		}

		in.logger.DebugContext(
			txCtx, "Outputs processed successfully",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.Int("processedOutputsCount", len(outputs)),
			logging.Number("cumulativeSatoshis", cumulativeSatoshis),
			slog.String("description", string(args.Description)),
		)

		if isMerge {
			in.logger.DebugContext(
				txCtx, "Upserting existing transaction",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.Int("labelsCount", len(args.Labels)),
				slog.Int("outputsCount", len(outputs)),
				slog.String("description", string(args.Description)),
			)

			uowErr = in.upsertExistingTx(txCtx, storedTx, outputs, args.Labels, repos)
			if uowErr != nil {
				return fmt.Errorf("failed to upsert outputs (isMerge): %w", uowErr)
			}

			in.logger.DebugContext(
				txCtx, "Existing transaction upserted successfully",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.String("description", string(args.Description)),
			)
		} else {
			in.logger.DebugContext(
				txCtx, "Storing new transaction",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.Int("labelsCount", len(args.Labels)),
				slog.Int("outputsCount", len(outputs)),
				logging.Number("cumulativeSatoshis", cumulativeSatoshis),
				slog.String("description", string(args.Description)),
			)

			shouldBroadcast, uowErr = in.storeNewTx(txCtx, userID, args, txID, tx, cumulativeSatoshis, outputs, repos)
			if uowErr != nil {
				return fmt.Errorf("failed to store new transaction: %w", uowErr)
			}

			in.logger.DebugContext(
				txCtx, "New transaction stored successfully",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.Bool("shouldBroadcast", shouldBroadcast),
				slog.String("description", string(args.Description)),
			)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if tx.MerklePath != nil {
		if err := in.updateKnownTxAsMined(ctx, userID, txID, tx); err != nil {
			in.logger.WarnContext(
				ctx, "updateKnownTxAsMined was not completed successfully",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.String("error", err.Error()),
			)
		}
	}

	result := &wdk.InternalizeActionResult{
		Accepted: true,
		IsMerge:  isMerge,
		TxID:     txID,
		Satoshis: cumulativeSatoshis.Int64(),
	}

	// Broadcast unknown unproven txs immediately via ProcessAction's BackgroundBroadcast path
	// (same post + status apply as delayed/monitor workers), matching TypeScript shareReqsWithWorld.
	// Uses the already-verified request BEEF rather than rebuilding from KnownTx so partial/shared
	// KnownTx rows (e.g. another user's in-flight send with incomplete raw bytes) cannot corrupt
	// the post. Soft outcomes populate SendWithResults / NotDelayedResults; hard errors leave the
	// tx for SendWaitingTransactions without rejecting the internalize.
	if shouldBroadcast && in.process != nil {
		in.logger.DebugContext(
			ctx, "Broadcasting newly internalized unproven transaction",
			logging.UserID(userID),
			slog.String("txID", txID),
		)

		var reviewResults []wdk.ReviewActionResult
		reviewResults, err = in.process.BackgroundBroadcast(ctx, beef, []string{txID})
		if err != nil {
			in.logger.WarnContext(
				ctx, "broadcast after internalize failed; leaving tx for monitor retry",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.String("error", err.Error()),
			)
			// Surface that a send was attempted so callers can observe in-flight status.
			result.SendWithResults = []wdk.SendWithResult{{
				TxID:   primitives.TXIDHexString(txID),
				Status: wdk.SendWithResultStatusSending,
			}}
			err = nil
		} else {
			result.NotDelayedResults = reviewResults
			result.SendWithResults = sendWithResultsFromReview(reviewResults)
		}
	}

	in.logger.DebugContext(
		ctx, "InternalizeAction completed successfully",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.Bool("accepted", true),
		slog.Bool("isMerge", isMerge),
		logging.Number("satoshis", cumulativeSatoshis),
		slog.Int("sendWithResultsCount", len(result.SendWithResults)),
		slog.Int("notDelayedResultsCount", len(result.NotDelayedResults)),
		slog.String("description", string(args.Description)),
	)

	return result, nil
}

func (in *internalize) updateKnownTxAsMined(ctx context.Context, userID int, txID string, tx *transaction.Transaction) error {
	block, err := in.blockHeaderService.ChainHeaderByHeight(ctx, tx.MerklePath.BlockHeight)
	if err != nil {
		return fmt.Errorf("failed to get chain header by height: %w", err)
	}

	root, err := tx.MerklePath.ComputeRootHex(to.Ptr(txID))
	if err != nil {
		return fmt.Errorf("failed to compute root hex: %w", err)
	}

	err = in.knownTxRepo.UpdateKnownTxAsMined(ctx, &entity.KnownTxAsMined{
		TxID:        txID,
		BlockHeight: tx.MerklePath.BlockHeight,
		MerklePath:  tx.MerklePath.Bytes(),
		BlockHash:   block.Hash,
		MerkleRoot:  root,
		Notes:       []history.Builder{history.NewBuilder().GetMerklePathSuccess("internalize-storage")},
	})
	if err != nil {
		return fmt.Errorf("failed to update known tx as mined: %w", err)
	}

	in.logger.DebugContext(
		ctx, "UpdateKnownTxAsMined completed successfully",
		logging.UserID(userID),
		slog.String("txID", txID),
	)

	return nil
}

func convertStringLikeSlice[ResultType, ArgType ~string](input []ArgType) []ResultType {
	return slices.Map(input, func(s ArgType) ResultType { return ResultType(s) })
}

func (in *internalize) upsertExistingTx(ctx context.Context, existingTx *pkgentity.Transaction, outputs []*OutputToInternalize, labels []primitives.StringUnder300, repos Providers) error {
	err := repos.TransactionsRepo().AddLabels(ctx, existingTx.UserID, existingTx.ID, convertStringLikeSlice[string](labels)...)
	if err != nil {
		return fmt.Errorf("failed to replace labels for existing transaction: %w", err)
	}

	outputsToInternalize := make([]*pkgentity.Output, 0, len(outputs))
	for _, toInternalize := range outputs {
		outputID := optional.OfPtr(toInternalize.existingOutputID).OrZeroValue() // Zero means it's a new output

		var output *pkgentity.Output
		output, err = toInternalize.ToOutput(outputID, existingTx.UserID, existingTx.ID)
		if err != nil {
			return fmt.Errorf("failed to convert output-to-internalize spec to entity: %w", err)
		}

		if output.Spendable && output.Change {
			if is.EmptyString(output.BasketName) {
				return fmt.Errorf("basket not provided for change output")
			}

			if output.Satoshis == 0 {
				return fmt.Errorf("change output with zero satoshis")
			}
			var sats uint64
			sats, err = satoshi.Value(output.Satoshis).UInt64()
			if err != nil {
				return fmt.Errorf("failed to convert satoshis to uint64: %w", err)
			}

			var utxoStatus wdk.UTXOStatus
			utxoStatus, err = in.utxoStatusByTxStatusForMerge(existingTx.Status)
			if err != nil {
				return fmt.Errorf("failed to get UTXO status by transaction status: %w", err)
			}

			output.UserUTXO = &pkgentity.UserUTXO{
				UserID:             output.UserID,
				Satoshis:           sats,
				EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
				Status:             utxoStatus,
			}
		}

		outputsToInternalize = append(outputsToInternalize, output)
	}

	// Ensure baskets exist before saving outputs (matching TS findOrInsertOutputBasket behavior)
	seen := make(map[string]bool)
	for _, output := range outputsToInternalize {
		if output.BasketName == nil {
			continue
		}
		name := *output.BasketName
		if seen[name] {
			continue
		}
		seen[name] = true
		if basketErr := repos.BasketRepo().FindOrCreateBasket(ctx, existingTx.UserID, name); basketErr != nil {
			return fmt.Errorf("failed to ensure basket %q exists: %w", name, basketErr)
		}
	}

	err = repos.OutputRepo().SaveOutputs(ctx, outputsToInternalize)
	if err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	return nil
}

// storeNewTx persists a newly internalized transaction.
// It returns shouldBroadcast=true when the tx is unknown/unproven (no mining proof and no prior
// network-acceptance evidence) so the caller can broadcast via ProcessAction's path and surface
// SendWithResults / NotDelayedResults on the InternalizeActionResult.
func (in *internalize) storeNewTx(
	ctx context.Context,
	userID int,
	args *wdk.InternalizeActionArgs,
	txID string,
	tx *transaction.Transaction,
	cumulativeSatoshis satoshi.Value,
	outputs []*OutputToInternalize,
	repos Providers,
) (shouldBroadcast bool, err error) {
	isMined := tx.MerklePath != nil

	// check if transaction already exists in DB with a confirmed broadcast status
	statuses, err := repos.KnownTxRepo().FindKnownTxStatuses(ctx, txID)
	if err != nil {
		return false, fmt.Errorf("failed to find existing known tx status: %w", err)
	}

	alreadySent := false
	if existingStatus, ok := statuses[txID]; ok {
		if existingStatus.AlreadySent() {
			alreadySent = true
		}
	}

	knownTxStatus := wdk.ProvenTxStatusUnsent
	txStatus := wdk.TxStatusSending
	utxoStatus := wdk.UTXOStatusSending
	// Broadcast only truly unknown unproven txs. Skip when the network already accepted the
	// tx, it is mined, or another path already has an in-flight broadcast (sending/unsent/unprocessed).
	shouldBroadcast = true
	if isMined || alreadySent {
		knownTxStatus = wdk.ProvenTxStatusUnmined
		txStatus = wdk.TxStatusUnproven
		utxoStatus = wdk.UTXOStatusUnproven
		shouldBroadcast = false
	} else if existingStatus, ok := statuses[txID]; ok && existingStatus.IsInFlight() {
		// KnownTx already tracked as in-flight by another user/path — do not re-broadcast.
		shouldBroadcast = false
	}

	skipForStatuses := []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusCompleted}
	if knownTxStatus == wdk.ProvenTxStatusUnmined {
		skipForStatuses = append(skipForStatuses, wdk.ProvenTxStatusUnmined)
	} else {
		skipForStatuses = append(skipForStatuses, wdk.ProvenTxStatusUnmined, wdk.ProvenTxStatusSending, wdk.ProvenTxStatusUnsent)
	}

	err = repos.KnownTxRepo().UpsertKnownTx(ctx, &entity.UpsertKnownTx{
		TxID:            txID,
		RawTx:           tx.Bytes(),
		InputBeef:       args.Tx,
		Status:          knownTxStatus,
		SkipForStatuses: skipForStatuses,
	}, history.NewBuilder().InternalizeAction(userID))
	if err != nil {
		return false, fmt.Errorf("failed to upsert known tx: %w", err)
	}

	reference, err := in.random.Base64(referenceLength)
	if err != nil {
		return false, fmt.Errorf("failed to generate random reference: %w", err)
	}

	err = repos.TransactionsRepo().CreateTransaction(ctx, &entity.NewTx{
		UserID:      userID,
		Version:     tx.Version,
		LockTime:    tx.LockTime,
		Status:      txStatus,
		UTXOStatus:  utxoStatus,
		Reference:   reference,
		IsOutgoing:  false,
		Description: string(args.Description),
		Satoshis:    cumulativeSatoshis.Int64(),
		TxID:        to.Ptr(txID),
		Outputs: slices.Map(outputs, func(out *OutputToInternalize) *entity.NewOutput {
			return out.NewOutput
		}),
		Labels: args.Labels,
	})
	if err != nil {
		return false, fmt.Errorf("failed to create transaction: %w", err)
	}

	return shouldBroadcast, nil
}

func (in *internalize) makeOutputs(
	ctx context.Context,
	userID int,
	tx *transaction.Transaction,
	outputSpecs []*wdk.InternalizeOutput,
	isMerge bool,
	repos Providers,
) ([]*OutputToInternalize, satoshi.Value, error) {
	satoshis := satoshi.Zero()

	changeBasketVerified := false

	var newOutputs []*OutputToInternalize
	outputsCount, err := to.UInt32(len(tx.Outputs))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert outputs count to uint32: %w", err)
	}
	for _, outputSpec := range outputSpecs {
		if outputSpec.OutputIndex >= outputsCount {
			return nil, 0, fmt.Errorf("output index %d is out of range of provided tx outputs count %d", outputSpec.OutputIndex, outputsCount)
		}

		output := tx.Outputs[outputSpec.OutputIndex]

		var existingOutput *pkgentity.Output
		if isMerge {
			existingOutput, err = repos.OutputRepo().FindOutput(ctx, userID, wdk.OutPoint{
				TxID: tx.TxID().String(),
				Vout: outputSpec.OutputIndex,
			})
			if err != nil {
				return nil, 0, fmt.Errorf("failed to find existing output: %w", err)
			}
			// NOTE: FindOutput can return nil if the output is not found
		}

		wasChangeOutput := existingOutput != nil && existingOutput.BasketName != nil && *existingOutput.BasketName == wdk.BasketNameForChange

		switch outputSpec.Protocol {
		case wdk.WalletPaymentProtocol:
			if wasChangeOutput {
				// the change output has already been added to the CHANGE basket
				continue
			}

			satoshis = satoshi.MustAdd(satoshis, output.Satoshis)

			if !changeBasketVerified {
				if err := in.checkChangeBasket(ctx, userID, repos); err != nil {
					return nil, 0, fmt.Errorf("failed to check change basket: %w", err)
				}
				changeBasketVerified = true
			}

			remittance := outputSpec.PaymentRemittance
			out := &OutputToInternalize{
				NewOutput: &entity.NewOutput{
					Vout:              outputSpec.OutputIndex,
					Spendable:         true,
					LockingScript:     to.Ptr(primitives.HexString(output.LockingScript.String())),
					BasketName:        to.Ptr(wdk.BasketNameForChange),
					Satoshis:          satoshi.MustFrom(output.Satoshis),
					SenderIdentityKey: to.Ptr(string(remittance.SenderIdentityKey)),
					Type:              wdk.OutputTypeP2PKH,
					ProvidedBy:        wdk.ProvidedByStorage,
					Purpose:           wdk.ChangePurpose,
					Change:            true,
					DerivationPrefix:  to.Ptr(string(remittance.DerivationPrefix)),
					DerivationSuffix:  to.Ptr(string(remittance.DerivationSuffix)),
				},
			}
			if existingOutput != nil {
				out.existingOutputID = to.Ptr(existingOutput.ID)
			}

			newOutputs = append(newOutputs, out)

		case wdk.BasketInsertionProtocol:
			remittance := outputSpec.InsertionRemittance

			tags := slices.Map(remittance.Tags, func(tag primitives.StringUnder300) string {
				return string(tag)
			})

			out := &OutputToInternalize{
				NewOutput: &entity.NewOutput{
					Vout:               outputSpec.OutputIndex,
					Spendable:          true,
					LockingScript:      to.Ptr(primitives.HexString(output.LockingScript.String())),
					BasketName:         to.Ptr(string(remittance.Basket)),
					Satoshis:           satoshi.MustFrom(output.Satoshis),
					Type:               wdk.OutputTypeCustom,
					CustomInstructions: remittance.CustomInstructions,
					Change:             false,
					ProvidedBy:         wdk.ProvidedByYou,
					Tags:               tags,
				},
			}

			if existingOutput != nil {
				out.existingOutputID = to.Ptr(existingOutput.ID)
			}

			newOutputs = append(newOutputs, out)

			if wasChangeOutput {
				// converting a change output to a user basket CUSTOM output
				// that effectively means that user's balance (in the change basket) is reduced by the amount of this output
				satoshis = satoshi.MustSubtract(satoshis, output.Satoshis)
			}
		}
	}

	return newOutputs, satoshis, nil
}

func (in *internalize) checkChangeBasket(ctx context.Context, userID int, repos Providers) error {
	basket, err := repos.BasketRepo().FindBasketByName(ctx, userID, wdk.BasketNameForChange)
	if err != nil {
		return fmt.Errorf("failed to find basket for change: %w", err)
	}
	if basket == nil {
		return fmt.Errorf("basket for change (%s) not found", wdk.BasketNameForChange)
	}
	return nil
}

func (in *internalize) isAllowedMergeStatus(status wdk.TxStatus) bool {
	switch status {
	case wdk.TxStatusCompleted, wdk.TxStatusUnproven, wdk.TxStatusNoSend, wdk.TxStatusSending:
		return true
	case wdk.TxStatusFailed, wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		fallthrough
	default:
		return false
	}
}

func (in *internalize) utxoStatusByTxStatusForMerge(txStatus wdk.TxStatus) (wdk.UTXOStatus, error) {
	switch txStatus {
	case wdk.TxStatusCompleted:
		return wdk.UTXOStatusMined, nil
	case wdk.TxStatusUnproven:
		return wdk.UTXOStatusUnproven, nil
	case wdk.TxStatusSending:
		return wdk.UTXOStatusSending, nil
	case wdk.TxStatusFailed, wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		fallthrough
	default:
		return "", fmt.Errorf("unsupported transaction status for UTXO: %s", txStatus)
	}
}

// sendWithResultsFromReview maps ProcessAction / BackgroundBroadcast review results to the
// SendWithResult statuses callers expect on InternalizeActionResult (and ProcessActionResult).
func sendWithResultsFromReview(review []wdk.ReviewActionResult) []wdk.SendWithResult {
	out := make([]wdk.SendWithResult, 0, len(review))
	for _, r := range review {
		sw := wdk.SendWithResult{TxID: r.TxID}
		switch r.Status {
		case wdk.ReviewActionResultStatusSuccess:
			sw.Status = wdk.SendWithResultStatusUnproven
		case wdk.ReviewActionResultStatusServiceError:
			sw.Status = wdk.SendWithResultStatusSending
		case wdk.ReviewActionResultStatusDoubleSpend, wdk.ReviewActionResultStatusInvalidTx:
			sw.Status = wdk.SendWithResultStatusFailed
		default:
			sw.Status = wdk.SendWithResultStatusSending
		}
		out = append(out, sw)
	}
	return out
}
