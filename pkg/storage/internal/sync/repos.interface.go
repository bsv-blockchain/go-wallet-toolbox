package sync

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type Repository interface {
	FindUser(ctx context.Context, identityKey string) (*wdk.TableUser, error)
	FindBasketsByUserID(ctx context.Context, userID int, opts ...queryopts.QueryOptsUnion) ([]*wdk.TableOutputBasket, error)
}
