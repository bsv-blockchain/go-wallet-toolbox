package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
)

type Sync struct {
	db *gorm.DB
}

func NewSync(db *gorm.DB) *Sync {
	return &Sync{db: db}
}

func (s *Sync) FindUserForSync(ctx context.Context, identityKey string) (*wdk.TableUser, error) {
	user := &models.User{}
	err := s.db.WithContext(ctx).First(&user, "identity_key = ?", identityKey).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find or create user: %w", err)
	}

	return &wdk.TableUser{
		UserID:        user.UserID,
		IdentityKey:   user.IdentityKey,
		ActiveStorage: user.ActiveStorage,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}, nil
}

func (s *Sync) FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error) {
	var outputBaskets []*models.OutputBasket
	err := s.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Scopes(scopes.FromQueryOpts(opts)...).
		Find(&outputBaskets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find output baskets: %w", err)
	}

	return slices.Map(outputBaskets, mapModelToTableOutputBasket), nil
}

func mapModelToTableOutputBasket(model *models.OutputBasket) *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		BasketID:  0, // TODO !!!!!!!
		UserID:    model.UserID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		BasketConfiguration: wdk.BasketConfiguration{
			Name:                    primitives.StringUnder300(model.Name),
			NumberOfDesiredUTXOs:    model.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: model.MinimumDesiredUTXOValue,
		},
	}
}
