package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"
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
	blockHeaderService wdk.BlockHeaderLoader
}

func newInternalizeAction(
	logger *slog.Logger,
	txRepo TransactionsRepo,
	basketRepo BasketRepo,
	knownTxRepo KnownTxRepo,
	outputRepo OutputRepo,
	random wdk.Randomizer,
	beefVerifier wdk.BeefVerifier,
	blockHeader wdk.BlockHeaderLoader,
) *internalize {
	logger = logging.Child(logger, "internalizeAction")
	return &internalize{
		logger:             logger,
		txRepo:             txRepo,
		basketRepo:         basketRepo,
		knownTxRepo:        knownTxRepo,
		outputRepo:         outputRepo,
		random:             random,
		beefVerifier:       beefVerifier,
		blockHeaderService: blockHeader,
	}
}

func (in *internalize) Internalize(ctx context.Context, userID int, args *wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "StorageActions-Internalize", attribute.Int("userID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	in.logger.DebugContext(ctx, "Starting internalize action",
		logging.UserID(userID),
		slog.Int("txBeefSize", len(args.Tx)),
		slog.Int("outputsCount", len(args.Outputs)),
		slog.String("description", string(args.Description)),
	)

	beef, txIDHash, err := transaction.NewBeefFromAtomicBytes(args.Tx)
	if err != nil {
		return nil, fmt.Errorf("failed to create atomic beef from bytes: %w", err)
	}

	in.logger.DebugContext(ctx, "Verifying beef transaction",
		logging.UserID(userID),
		slog.String("txID", txIDHash.String()),
		slog.String("description", string(args.Description)),
	)

	if ok, err := in.beefVerifier.VerifyBeef(ctx, beef, false); err != nil {
		return nil, fmt.Errorf("failed to verify beef: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("provided beef is not valid")
	}

	tx := beef.FindAtomicTransactionByHash(txIDHash)
	if tx == nil {
		return nil, fmt.Errorf("atomic beef error: transaction with hash %s not found", txIDHash)
	}

	txID := txIDHash.String()

	in.logger.DebugContext(ctx, "BEEF verification completed successfully",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.String("description", string(args.Description)),
	)

	in.logger.DebugContext(ctx, "Checking for existing transaction",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.String("description", string(args.Description)),
	)

	storedTx, err := in.txRepo.FindTransactionByUserIDAndTxID(ctx, userID, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction by userID and txID: %w", err)
	}

	isMerge := storedTx != nil

	if isMerge {
		in.logger.DebugContext(ctx, "Transaction already exists - performing merge",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.String("existingStatus", string(storedTx.Status)),
			slog.String("description", string(args.Description)),
		)
	} else {
		in.logger.DebugContext(ctx, "New transaction - creating fresh entry",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.String("description", string(args.Description)),
		)
	}

	if isMerge && !in.isAllowedMergeStatus(storedTx.Status) {
		return nil, fmt.Errorf("target transaction of internalizeAction has invalid status: %q", storedTx.Status)
	}

	in.logger.DebugContext(ctx, "Processing outputs",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.Int("outputsToProcess", len(args.Outputs)),
		slog.Bool("isMerge", isMerge),
		slog.String("description", string(args.Description)),
	)

	outputs, cumulativeSatoshis, err := in.makeOutputs(ctx, userID, tx, args.Outputs, isMerge)
	if err != nil {
		return nil, fmt.Errorf("failed to create new outputs: %w", err)
	}

	in.logger.DebugContext(ctx, "Outputs processed successfully",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.Int("processedOutputsCount", len(outputs)),
		logging.Number("cumulativeSatoshis", cumulativeSatoshis),
		slog.String("description", string(args.Description)),
	)

	if isMerge {
		in.logger.DebugContext(ctx, "Upserting existing transaction",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.Int("labelsCount", len(args.Labels)),
			slog.Int("outputsCount", len(outputs)),
			slog.String("description", string(args.Description)),
		)

		err = in.upsertExistingTx(ctx, storedTx, outputs, args.Labels)
		if err != nil {
			return nil, fmt.Errorf("failed to upsert outputs (isMerge): %w", err)
		}

		in.logger.DebugContext(ctx, "Existing transaction upserted successfully",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.String("description", string(args.Description)),
		)
	} else {
		in.logger.DebugContext(ctx, "Storing new transaction",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.Int("labelsCount", len(args.Labels)),
			slog.Int("outputsCount", len(outputs)),
			logging.Number("cumulativeSatoshis", cumulativeSatoshis),
			slog.String("description", string(args.Description)),
		)

		err = in.storeNewTx(ctx, userID, args, txID, tx, cumulativeSatoshis, outputs)
		if err != nil {
			return nil, fmt.Errorf("failed to store new transaction: %w", err)
		}

		in.logger.DebugContext(ctx, "New transaction stored successfully",
			logging.UserID(userID),
			slog.String("txID", txID),
			slog.String("description", string(args.Description)),
		)
	}

	if tx.MerklePath != nil {
		if err := in.updateKnownTxAsMined(ctx, userID, txID, tx); err != nil {
			in.logger.Warn("updateKnownTxAsMined was not completed successfully",
				logging.UserID(userID),
				slog.String("txID", txID),
				slog.String("error", err.Error()),
			)
		}
	}

	in.logger.DebugContext(ctx, "InternalizeAction completed successfully",
		logging.UserID(userID),
		slog.String("txID", txID),
		slog.Bool("accepted", true),
		slog.Bool("isMerge", isMerge),
		logging.Number("satoshis", cumulativeSatoshis),
		slog.String("description", string(args.Description)),
	)

	return &wdk.InternalizeActionResult{
		Accepted: true,
		IsMerge:  isMerge,
		TxID:     txID,
		Satoshis: cumulativeSatoshis.Int64(),
	}, nil
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

	in.logger.DebugContext(ctx, "UpdateKnownTxAsMined completed successfully",
		logging.UserID(userID),
		slog.String("txID", txID),
	)

	return nil
}

func convertStringLikeSlice[ResultType, ArgType ~string](input []ArgType) []ResultType {
	return slices.Map(input, func(s ArgType) ResultType { return ResultType(s) })
}

func (in *internalize) upsertExistingTx(ctx context.Context, existingTx *pkgentity.Transaction, outputs []*OutputToInternalize, labels []primitives.StringUnder300) error {
	err := in.txRepo.AddLabels(ctx, existingTx.UserID, existingTx.ID, convertStringLikeSlice[string](labels)...)
	if err != nil {
		return fmt.Errorf("failed to replace labels for existing transaction: %w", err)
	}

	outputsToInternalize := make([]*pkgentity.Output, 0, len(outputs))
	for _, toInternalize := range outputs {
		outputID := optional.OfPtr(toInternalize.existingOutputID).OrZeroValue() // Zero means it's a new output

		output, err := toInternalize.ToOutput(outputID, existingTx.UserID, existingTx.ID)
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
			sats, err := satoshi.Value(output.Satoshis).UInt64()
			if err != nil {
				return fmt.Errorf("failed to convert satoshis to uint64: %w", err)
			}

			utxoStatus, err := in.utxoStatusByTxStatusForMerge(existingTx.Status)
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

	err = in.outputRepo.SaveOutputs(ctx, outputsToInternalize)
	if err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	return nil
}

func (in *internalize) storeNewTx(
	ctx context.Context,
	userID int,
	args *wdk.InternalizeActionArgs,
	txID string,
	tx *transaction.Transaction,
	cumulativeSatoshis satoshi.Value,
	outputs []*OutputToInternalize,
) error {
	err := in.knownTxRepo.UpsertKnownTx(ctx, &entity.UpsertKnownTx{
		TxID:          txID,
		RawTx:         tx.Bytes(),
		InputBeef:     args.Tx,
		Status:        wdk.ProvenTxStatusUnmined,
		SkipForStatus: to.Ptr(wdk.ProvenTxStatusCompleted),
	}, history.NewBuilder().InternalizeAction(userID))
	if err != nil {
		return fmt.Errorf("failed to upsert known tx: %w", err)
	}

	reference, err := in.random.Base64(referenceLength)
	if err != nil {
		return fmt.Errorf("failed to generate random reference: %w", err)
	}

	err = in.txRepo.CreateTransaction(ctx, &entity.NewTx{
		UserID:      userID,
		Version:     tx.Version,
		LockTime:    tx.LockTime,
		Status:      wdk.TxStatusUnproven,
		UTXOStatus:  wdk.UTXOStatusUnproven,
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
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

func (in *internalize) makeOutputs(
	ctx context.Context,
	userID int,
	tx *transaction.Transaction,
	outputSpecs []*wdk.InternalizeOutput,
	isMerge bool,
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
			existingOutput, err = in.outputRepo.FindOutput(ctx, userID, wdk.OutPoint{
				TxID: tx.TxID().String(),
				Vout: outputSpec.OutputIndex,
			})
			if err != nil {
				return nil, 0, fmt.Errorf("failed to find existing output: %w", err)
			}
			//NOTE: FindOutput can return nil if the output is not found
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
				if err := in.checkChangeBasket(ctx, userID); err != nil {
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

func (in *internalize) checkChangeBasket(ctx context.Context, userID int) error {
	basket, err := in.basketRepo.FindBasketByName(ctx, userID, wdk.BasketNameForChange)
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
	case wdk.TxStatusCompleted, wdk.TxStatusUnproven, wdk.TxStatusNoSend:
		return true
	case wdk.TxStatusFailed, wdk.TxStatusUnprocessed, wdk.TxStatusSending, wdk.TxStatusUnsigned, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
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
	case wdk.TxStatusFailed, wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusSending, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		fallthrough
	default:
		return "", fmt.Errorf("unsupported transaction status for UTXO: %s", txStatus)
	}
}
