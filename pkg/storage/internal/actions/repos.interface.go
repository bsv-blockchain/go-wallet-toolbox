package actions

import (
	"context"
	"iter"

	"github.com/bsv-blockchain/go-sdk/transaction"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
	FindOutputsByOutpoints(ctx context.Context, userID int, outpoints []wdk.OutPoint) ([]*entity.Output, error)
	SaveOutputs(ctx context.Context, output []*entity.Output) error
	MakeOutputsSpendableForTxID(ctx context.Context, txID string) error
	RecreateSpentOutputs(ctx context.Context, spendingTransactionID uint) error
	ShouldTxOutputsBeUnspent(ctx context.Context, transactionID uint) error
}

type TransactionsRepo interface {
	CreateTransaction(ctx context.Context, transaction *entity.NewTx) error
	FindTransactionByUserIDAndTxID(ctx context.Context, userID int, txID string) (*entity.Transaction, error)
	FindTransactionByReference(ctx context.Context, userID int, reference string) (*entity.Transaction, error)
	SpendTransaction(ctx context.Context, updatedTx entity.UpdatedTx, txNote history.Builder) error
	UpdateTransactionStatusByTxID(ctx context.Context, txID string, txStatus wdk.TxStatus) error
	UpdateTransactionStatusByID(ctx context.Context, transactionID uint, txStatus wdk.TxStatus) error
	ListAndCountActions(ctx context.Context, userID int, filter entity.ListActionsFilter) ([]*entity.Transaction, int64, error)
	GetLabelsForTransactions(ctx context.Context, txIDs []uint) (map[uint][]string, error)
	AddLabels(ctx context.Context, userID int, transactionID uint, labels ...string) error
	FindTransactionIDsByTxID(ctx context.Context, txID string) ([]uint, error)
	FindTransactionIDsByStatuses(ctx context.Context, txStatus []wdk.TxStatus, opts ...queryopts.Options) ([]uint, error)
}

type KnownTxRepo interface {
	UpsertKnownTx(ctx context.Context, req *entity.UpsertKnownTx, txNote history.Builder) error
	FindKnownTxRawTx(ctx context.Context, txID string) ([]byte, error)
	FindKnownTxStatuses(ctx context.Context, txIDs ...string) (map[string]wdk.ProvenTxReqStatus, error)
	FindKnownTxIDsByStatuses(ctx context.Context, txStatus []wdk.ProvenTxReqStatus, opts ...queryopts.Options) ([]*entity.KnownTxForStatusSync, error)
	GetBEEFForTxID(ctx context.Context, txID string, opts ...entity.GetBEEFOption) (*transaction.Beef, error)
	UpdateKnownTxAsMined(ctx context.Context, provenTxAsMined *entity.KnownTxAsMined) error
	GetBEEFForTxIDs(ctx context.Context, txids iter.Seq[string], opts ...entity.GetBEEFOption) (*transaction.Beef, error)
	AllKnownTxsExist(ctx context.Context, txIDs []string, sourceTxsStatusFilter []wdk.ProvenTxReqStatus) (bool, error)
	IncreaseKnownTxAttemptsForTxIDs(ctx context.Context, txIDs []string) error
	SetStatusForKnownTxsAboveAttempts(ctx context.Context, attempts uint64, status wdk.ProvenTxReqStatus) error
	FindKnownTxRawTxs(ctx context.Context, txIDs []string) (map[string][]byte, error)
	UpdateKnownTxStatus(ctx context.Context, txID string, status wdk.ProvenTxReqStatus, skipForStatuses []wdk.ProvenTxReqStatus, txNotes []history.Builder) error
	SetBatchForKnownTxs(ctx context.Context, txIDs []string, batch string) error
}

type KeyValueRepo interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte) error
}

type CommissionRepo interface {
	AddCommission(ctx context.Context, commission *pkgentity.Commission) error
	FindCommission(ctx context.Context, userID int, transactionID uint) (*pkgentity.Commission, error)
}

type UTXORepo interface {
	UnreserveUTXOsByTransactionID(ctx context.Context, transactionID uint) error
}
