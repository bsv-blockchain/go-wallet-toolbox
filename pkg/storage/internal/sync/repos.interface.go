package sync

import (
	"context"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Repository interface {
	FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error)

	FindUser(ctx context.Context, identityKey string) (*entity.User, error)
	UpdateUser(ctx context.Context, userID int, activeStorage string, updatedAt time.Time) error

	FindSyncState(ctx context.Context, userID int, storageIdentityKey string) (*entity.SyncState, error)
	CreateSyncState(ctx context.Context, syncState *entity.SyncState) (*entity.SyncState, error)
	UpdateSyncState(ctx context.Context, syncState *entity.SyncState) error

	UpsertOutputBasket(ctx context.Context, userID int, basket wdk.BasketConfiguration) (isNew bool, err error)

	FindKnownTxsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableProvenTxReq, []*wdk.TableProvenTx, error)
	UpsertKnownTxForSync(ctx context.Context, entity *entity.KnownTx) (isNew bool, err error)

	FindTransactionsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTransaction, error)
	UpsertTransactionForSync(ctx context.Context, entity *entity.Transaction) (isNew bool, transactionID uint, err error)
}
