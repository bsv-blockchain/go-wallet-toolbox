package actions

import (
	"context"
	"iter"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

type BasketRepo interface {
	FindBasketByName(ctx context.Context, userID int, name string) (*wdk.TableOutputBasket, error)
}

type OutputRepo interface {
	FindOutputs(ctx context.Context, outputIDs iter.Seq[uint]) ([]*wdk.TableOutput, error)
	FindInputsAndOutputsOfTransaction(ctx context.Context, transactionID uint) (inputs []*wdk.TableOutput, outputs []*wdk.TableOutput, err error)
}

type TransactionsRepo interface {
	CreateTransaction(ctx context.Context, transaction *entity.NewTx) error
	FindTransactionByUserIDAndTxID(ctx context.Context, userID int, txID string) (*wdk.TableTransaction, error)
	FindTransactionByReference(ctx context.Context, userID int, reference string) (*wdk.TableTransaction, error)
	SpendTransaction(
		ctx context.Context,
		updatedTx entity.UpdatedTx,
		historyNote string,
		historyAttrs map[string]any,
	) error
	UpdateTransactionStatusForTxID(
		ctx context.Context,
		txID string,
		txStatus wdk.TxStatus,
		provenTxReqStatus wdk.ProvenTxReqStatus,
		historyNote string,
		historyAttrs map[string]any,
	) error
}

type ProvenTxRepo interface {
	UpsertProvenTxReq(ctx context.Context, req *entity.UpsertProvenTxReq, historyNote string, historyAttrs map[string]any) error
	FindProvenTxRawTX(ctx context.Context, txID string) ([]byte, error)
	FindProvenTxStatus(ctx context.Context, txID string) (wdk.ProvenTxReqStatus, error)
	FindProvenTxIDsByStatuses(ctx context.Context, limit int, txStatus ...wdk.ProvenTxReqStatus) ([]*entity.ProvenTxToSync, error)
	BuildValidBEEF(ctx context.Context, txID string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (*transaction.Beef, error)
	UpdateProvenTxAsMined(ctx context.Context, provenTxAsMined *entity.ProvenTxAsMined) error
}
