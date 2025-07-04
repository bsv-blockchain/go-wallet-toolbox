package actions

import (
	"context"
	"iter"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

type BasketRepo interface {
	FindBasketByName(ctx context.Context, userID int, name string) (*entity.OutputBasket, error)
}

type OutputRepo interface {
	FindOutputs(ctx context.Context, outputIDs iter.Seq[uint]) ([]*entity.Output, error)
	FindOutput(ctx context.Context, userID int, outpoint wdk.OutPoint) (*entity.Output, error)
	FindOutputsByTransactionID(ctx context.Context, transactionID uint) ([]*entity.Output, error)
	ListAndCountOutputs(ctx context.Context, filter entity.ListOutputsFilter) ([]*entity.Output, int64, error)
	FindInputsAndOutputsWithBaskets(ctx context.Context, txIDs []uint, includeLockingScripts bool) (inputs map[uint][]*entity.Output, outputs map[uint][]*entity.Output, err error)
	SaveOutput(ctx context.Context, output *entity.Output) error
}

type TransactionsRepo interface {
	CreateTransaction(ctx context.Context, transaction *entity.NewTx) error
	FindTransactionByUserIDAndTxID(ctx context.Context, userID int, txID string) (*entity.Transaction, error)
	FindTransactionByReference(ctx context.Context, userID int, reference string) (*entity.Transaction, error)
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
	ListAndCountActions(ctx context.Context, userID int, filter entity.ListActionsFilter) ([]*entity.Transaction, int64, error)
	GetLabelsForTransactions(ctx context.Context, txIDs []uint) (map[uint][]string, error)
}

type KnownTxRepo interface {
	UpsertKnownTx(ctx context.Context, req *entity.UpsertKnownTx, historyNote string, historyAttrs map[string]any) error
	FindKnownTxRawTx(ctx context.Context, txID string) ([]byte, error)
	FindKnownTxStatus(ctx context.Context, txID string) (wdk.ProvenTxReqStatus, error)
	FindKnownTxIDsByStatuses(ctx context.Context, limit int, txStatus ...wdk.ProvenTxReqStatus) ([]*entity.KnownTxForStatusSync, error)
	BuildValidBEEF(ctx context.Context, txID string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (*transaction.Beef, error)
	UpdateKnownTxAsMined(ctx context.Context, provenTxAsMined *entity.KnownTxAsMined) error
	GetBEEFForTxIDs(ctx context.Context, txids iter.Seq[string], knownTxIDs []string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) ([]byte, error)
	AllKnownTxsExist(ctx context.Context, txIDs []string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (bool, error)
	IncreaseKnownTxAttemptsForTxIDs(ctx context.Context, txIDs []string) error
	SetStatusForKnownTxsAboveAttempts(ctx context.Context, attempts uint64, status wdk.ProvenTxReqStatus) error
	FindKnownTxRawTxs(ctx context.Context, txIDs []string) (map[string][]byte, error)
}

type KeyValueRepo interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte) error
}

type CommissionRepo interface {
	AddCommission(ctx context.Context, commission *entity.Commission) error
}
