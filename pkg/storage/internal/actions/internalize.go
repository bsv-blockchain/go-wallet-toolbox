package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/history"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
)

type internalize struct {
	logger       *slog.Logger
	txRepo       TransactionsRepo
	basketRepo   BasketRepo
	provenTxRepo ProvenTxRepo
	random       wdk.Randomizer
}

func newInternalizeAction(
	logger *slog.Logger,
	txRepo TransactionsRepo,
	basketRepo BasketRepo,
	provenTxRepo ProvenTxRepo,
	random wdk.Randomizer,
) *internalize {
	logger = logging.Child(logger, "internalizeAction")
	return &internalize{
		logger:       logger,
		txRepo:       txRepo,
		basketRepo:   basketRepo,
		provenTxRepo: provenTxRepo,
		random:       random,
	}
}

func (in *internalize) Internalize(ctx context.Context, userID int, args *wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
	tx, err := transaction.NewTransactionFromBEEF(args.Tx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction from BEEF: %w", err)
	}
	txID := tx.TxID().String()

	// TODO: Do SPV verification of the transaction - it requires Services::ChainTracker

	storedTx, err := in.txRepo.FindTransactionByUserIDAndTxID(ctx, userID, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction by userID and txID: %w", err)
	}
	if storedTx != nil {
		panic("not implemented yet") // TODO: Implement internalize action for known transaction (with merge)
	}

	newOutputs, cumulativeSatoshis, err := in.newOutputs(ctx, userID, tx, args.Outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to create new outputs: %w", err)
	}

	reference, err := in.random.Base64(referenceLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random reference: %w", err)
	}

	// TODO: Don't upsert ProvenTxReq if the transaction is already known in ProvenTx (not *Req)
	err = in.provenTxRepo.UpsertProvenTxReq(ctx, &entity.UpsertProvenTxReq{
		TxID:      txID,
		RawTx:     tx.Bytes(),
		InputBeef: args.Tx,
		Status:    wdk.ProvenTxStatusUnmined,
	}, history.InternalizeActionHistoryNote, history.UserIDHistoryAttr(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to upsert proven tx request: %w", err)
	}

	err = in.txRepo.CreateTransaction(ctx, &entity.NewTx{
		UserID:      userID,
		Version:     tx.Version,
		LockTime:    tx.LockTime,
		Status:      wdk.TxStatusUnproven,
		Reference:   reference,
		IsOutgoing:  false,
		Description: string(args.Description),
		Satoshis:    cumulativeSatoshis.Int64(),
		TxID:        to.Ptr(txID),
		Outputs:     newOutputs,
		Labels:      args.Labels,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	return &wdk.InternalizeActionResult{
		Accepted: true,
		IsMerge:  false,
		TxID:     txID,
		Satoshis: primitives.SatoshiValue(cumulativeSatoshis.MustUInt64()),
	}, nil
}

func (in *internalize) newOutputs(ctx context.Context, userID int, tx *transaction.Transaction, outputSpecs []*wdk.InternalizeOutput) ([]*entity.NewOutput, satoshi.Value, error) {
	satoshis := satoshi.Zero()

	changeBasketVerified := false

	var newOutputs []*entity.NewOutput
	outputsCount, err := to.UInt32(len(tx.Outputs))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert outputs count to uint32: %w", err)
	}
	for _, outputSpec := range outputSpecs {
		if outputSpec.OutputIndex >= outputsCount {
			return nil, 0, fmt.Errorf("output index %d is out of range of provided tx outputs count %d", outputSpec.OutputIndex, outputsCount)
		}

		output := tx.Outputs[outputSpec.OutputIndex]

		switch outputSpec.Protocol {
		case wdk.WalletPaymentProtocol:
			satoshis = satoshi.MustAdd(satoshis, output.Satoshis)

			if !changeBasketVerified {
				if err := in.checkChangeBasket(ctx, userID); err != nil {
					return nil, 0, fmt.Errorf("failed to check change basket: %w", err)
				}
				changeBasketVerified = true
			}

			remittance := outputSpec.PaymentRemittance
			newOutputs = append(newOutputs, &entity.NewOutput{
				Vout:              outputSpec.OutputIndex,
				Spendable:         true,
				LockingScript:     to.Ptr(primitives.HexString(output.LockingScript.String())),
				Basket:            to.Ptr(wdk.BasketNameForChange),
				Satoshis:          satoshi.MustFrom(output.Satoshis),
				SenderIdentityKey: to.Ptr(string(remittance.SenderIdentityKey)),
				Type:              wdk.OutputTypeP2PKH,
				ProvidedBy:        wdk.ProvidedByStorage,
				Purpose:           wdk.ChangePurpose,
				Change:            true,
				DerivationPrefix:  to.Ptr(string(remittance.DerivationPrefix)),
				DerivationSuffix:  to.Ptr(string(remittance.DerivationSuffix)),
			})

		case wdk.BasketInsertionProtocol:
			remittance := outputSpec.InsertionRemittance
			newOutputs = append(newOutputs, &entity.NewOutput{
				Vout:               outputSpec.OutputIndex,
				Spendable:          true,
				LockingScript:      to.Ptr(primitives.HexString(output.LockingScript.String())),
				Basket:             to.Ptr(string(remittance.Basket)),
				Satoshis:           satoshi.MustFrom(output.Satoshis),
				Type:               wdk.OutputTypeCustom,
				CustomInstructions: remittance.CustomInstructions,
				Change:             false,
				ProvidedBy:         wdk.ProvidedByYou,
			})
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
