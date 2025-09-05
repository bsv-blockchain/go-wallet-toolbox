package sync

import (
	"context"
	"time"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Repository interface {
	FindUser(ctx context.Context, identityKey string) (*pkgentity.User, error)
	UpdateUserForSync(ctx context.Context, userID int, activeStorage string, updatedAt time.Time) error

	FindSyncState(ctx context.Context, userID int, storageIdentityKey string) (*entity.SyncState, error)
	CreateSyncState(ctx context.Context, syncState *entity.SyncState) (*entity.SyncState, error)
	UpdateSyncState(ctx context.Context, syncState *entity.SyncState) error

	FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error)
	UpsertOutputBasketForSync(ctx context.Context, entity pkgentity.OutputBasket) (isNew bool, basketNumID uint, err error)
	FindBasketNameByNumIDForSync(ctx context.Context, basketNumID uint) (string, error)

	FindKnownTxsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableProvenTxReq, []*wdk.TableProvenTx, error)
	UpsertKnownTxForSync(ctx context.Context, entity *pkgentity.KnownTx) (isNew bool, err error)

	FindTransactionsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTransaction, error)
	UpsertTransactionForSync(ctx context.Context, entity *pkgentity.Transaction) (isNew bool, transactionID uint, err error)

	FindOutputsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutput, error)
	UpsertOutputForSync(ctx context.Context, entity *pkgentity.Output) (isNew bool, outputID uint, err error)

	FindLabelsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabel, error)
	UpsertLabelForSync(ctx context.Context, entity *entity.Label) (isNew bool, labelNumID uint, err error)
	DeleteLabelForSync(ctx context.Context, entity *entity.Label) (deleted bool, err error)
	FindLabelsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabelMap, error)
	FindLabelByNumIDForSync(ctx context.Context, labelNumID uint) (*entity.Label, error)
	DeleteLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (deleted bool, err error)
	UpsertLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (isNew bool, err error)

	FindTagsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputTag, error)
	UpsertTagForSync(ctx context.Context, entity *entity.Tag) (isNew bool, tagNumID uint, err error)
	DeleteTagForSync(ctx context.Context, entity *entity.Tag) (deleted bool, err error)
	FindTagsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputTagMap, error)
	FindTagByNumIDForSync(ctx context.Context, labelNumID uint) (*entity.Tag, error)
	DeleteTagMapForSync(ctx context.Context, entity *entity.TagMap) (deleted bool, err error)
	UpsertTagMapForSync(ctx context.Context, entity *entity.TagMap) (isNew bool, err error)
}
