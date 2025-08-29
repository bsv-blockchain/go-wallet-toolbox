package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type Users struct {
	db            *gorm.DB
	query         *genquery.Query
	settings      *Settings
	outputBaskets *OutputBaskets
}

func NewUsers(db *gorm.DB, query *genquery.Query, settings *Settings, outputBaskets *OutputBaskets) *Users {
	return &Users{db: db, query: query, settings: settings, outputBaskets: outputBaskets}
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

func (u *Users) UpdateUserByValues(ctx context.Context, userID int, activeStorage string, updatedAt time.Time) error {
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

func (u *Users) AddUser(ctx context.Context, user *entity.User) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}

	model := &models.User{
		UserID:        user.ID,
		IdentityKey:   user.IdentityKey,
		ActiveStorage: user.ActiveStorage,
	}
	return u.db.WithContext(ctx).Create(model).Error
}

func (u *Users) UpdateUser(ctx context.Context, spec *entity.UserUpdateSpecification) error {
	table := &u.query.User

	updates := map[string]any{}

	if spec.ActiveStorage != nil {
		updates[table.ActiveStorage.ColumnName().String()] = *spec.ActiveStorage
	}
	if spec.IdentityKey != nil {
		updates[table.IdentityKey.ColumnName().String()] = *spec.IdentityKey
	}

	if spec.UpdatedAt != nil {
		updates[table.UpdatedAt.ColumnName().String()] = *spec.UpdatedAt
	}

	if len(updates) == 0 {
		return nil
	}

	_, err := table.WithContext(ctx).Where(table.UserID.Eq(spec.ID)).Updates(updates)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (u *Users) FindUsers(ctx context.Context, spec *entity.UserReadSpecification, opts ...queryopts.Options) ([]*entity.User, error) {
	table := &u.query.User

	users, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(u.conditionsBySpec(spec)...).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find users: %w", err)
	}

	return slices.Map(users, mapUserModelToEntity), nil
}

func (u *Users) CountUsers(ctx context.Context, spec *entity.UserReadSpecification, opts ...queryopts.Options) (int64, error) {
	table := &u.query.User

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(u.conditionsBySpec(spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

func (u *Users) conditionsBySpec(spec *entity.UserReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}

	table := &u.query.User
	if spec.ID != nil {
		return []gen.Condition{table.UserID.Eq(*spec.ID)}
	}

	var conditions []gen.Condition
	if spec.IdentityKey != nil {
		conditions = append(conditions, cmpCondition(table.IdentityKey, spec.IdentityKey))
	}

	return conditions
}
