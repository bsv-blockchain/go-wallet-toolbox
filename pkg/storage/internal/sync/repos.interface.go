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
}
