package sync

import (
	"context"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Repository interface {
	FindUserForSync(ctx context.Context, identityKey string) (*wdk.TableUser, error)
	FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error)
}
