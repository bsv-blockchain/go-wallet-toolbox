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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"go.opentelemetry.io/otel/attribute"
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
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Users-FindUser", attribute.String("IdentityKey", identityKey))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	user := &models.User{}
	err = u.db.WithContext(ctx).First(&user, "identity_key = ?", identityKey).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find or create user: %w", err)
	}

	return mapUserModelToEntity(user), nil
}

func (u *Users) CreateUser(ctx context.Context, identityKey, activeStorage string, baskets ...wdk.BasketConfiguration) (*entity.User, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Users-CreateUser", attribute.String("IdentityKey", identityKey), attribute.String("ActiveStorage", activeStorage))
	defer func() {
		tracing.EndTracing(span, err)
	}()

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
	err = u.db.WithContext(ctx).Create(&user).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return mapUserModelToEntity(&user), nil
}

func (u *Users) UpdateUserForSync(ctx context.Context, userID int, activeStorage string, updatedAt time.Time) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Users-UpdateUserForSync", attribute.Int("UserID", userID), attribute.String("ActiveStorage", activeStorage))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = u.db.WithContext(ctx).
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
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Users-AddUser")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if user == nil {
		err = fmt.Errorf("user cannot be nil")
		return err
	}

	model := &models.User{
		IdentityKey:   user.IdentityKey,
		ActiveStorage: user.ActiveStorage,
	}
	return u.db.WithContext(ctx).Create(model).Error
}

func (u *Users) UpdateUser(ctx context.Context, spec *entity.UserUpdateSpecification) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Users-UpdateUser", attribute.Int("UserID", spec.ID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &u.query.User

	updates := map[string]any{}

	if spec.ActiveStorage != nil {
		updates[table.ActiveStorage.ColumnName().String()] = *spec.ActiveStorage
	}
	if spec.IdentityKey != nil {
		updates[table.IdentityKey.ColumnName().String()] = *spec.IdentityKey
	}

	if len(updates) == 0 {
		return nil
	}

	_, err = table.WithContext(ctx).Where(table.UserID.Eq(spec.ID)).Updates(updates)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (u *Users) FindUsers(ctx context.Context, spec *entity.UserReadSpecification, opts ...queryopts.Options) ([]*entity.User, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Users-FindUsers")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Users-CountUsers")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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
	if spec.ActiveStorage != nil {
		conditions = append(conditions, cmpCondition(table.ActiveStorage, spec.ActiveStorage))
	}

	return conditions
}
