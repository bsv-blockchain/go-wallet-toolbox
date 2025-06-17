package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
)

type Users struct {
	db            *gorm.DB
	settings      *Settings
	outputBaskets *OutputBaskets
}

func NewUsers(db *gorm.DB, settings *Settings, outputBaskets *OutputBaskets) *Users {
	return &Users{db: db, settings: settings, outputBaskets: outputBaskets}
}

func (u *Users) FindUser(ctx context.Context, identityKey string) (*entity.User, error) {
	user := &models.User{}
	err := u.db.WithContext(ctx).First(&user, "identity_key = ?", identityKey).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find or create user: %w", err)
	}

	return mapUserModelToEntity(user), nil
}

func (u *Users) CreateUser(ctx context.Context, identityKey, activeStorage string, baskets ...wdk.BasketConfiguration) (*entity.User, error) {
	user := models.User{
		IdentityKey:   identityKey,
		ActiveStorage: activeStorage,
		OutputBaskets: slices.Map(baskets, func(basket wdk.BasketConfiguration) *models.OutputBasket {
			return &models.OutputBasket{
				Name:                    string(basket.Name),
				NumberOfDesiredUTXOs:    basket.NumberOfDesiredUTXOs,
				MinimumDesiredUTXOValue: basket.MinimumDesiredUTXOValue,
			}
		}),
	}
	err := u.db.WithContext(ctx).Create(&user).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return mapUserModelToEntity(&user), nil
}

func (u *Users) UpdateUser(ctx context.Context, userID int, activeStorage string, updatedAt time.Time) error {
	err := u.db.WithContext(ctx).
		Model(&models.User{}).
		Scopes(scopes.UserID(userID)).
		Updates(map[string]any{
			"active_storage": activeStorage,
			"updated_at":     updatedAt,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func mapUserModelToEntity(user *models.User) *entity.User {
	return &entity.User{
		ID:            user.UserID,
		IdentityKey:   user.IdentityKey,
		ActiveStorage: user.ActiveStorage,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}
