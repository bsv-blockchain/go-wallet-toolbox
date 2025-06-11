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
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Sync struct {
	db *gorm.DB

	name *naming
}

func NewSync(db *gorm.DB) *Sync {
	return &Sync{
		db:   db,
		name: newNaming(db),
	}
}

func (s *Sync) FindUserForSync(ctx context.Context, identityKey string) (*wdk.TableUser, error) {
	user := &models.User{}
	err := s.db.WithContext(ctx).
		First(&user, "identity_key = ?", identityKey).Error
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

type OutputBasketWithNum struct {
	models.OutputBasket
	NumID int64
}

func (s *Sync) FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error) {
	var outputBaskets []*OutputBasketWithNum

	err := s.db.Transaction(func(tx *gorm.DB) error {

		/*
			INSERT INTO bsv_numeric_id_lookups (table_name, string_id) SELECT \"bsv_output_baskets\", name FROM bsv_output_baskets user_id = 1 AND name IS NOT NULL ON CONFLICT DO NOTHING
			query := fmt.Sprintf(`
					INSERT INTO %s (table_name, string_id) SELECT ?, name FROM %s WHERE user_id = ? AND name IS NOT NULL
				`, s.name.numericIDLookupTableName, s.name.outputBasketTableName)
		*/

		err := s.upsertNumericIDLookup(ctx, tx, func(db *gorm.DB) *gorm.DB {
			return db.Model(&models.OutputBasket{}).
				Select("?, name", s.name.outputBasketTableName).
				Scopes(scopes.UserID(userID)).
				Scopes(scopes.FromQueryOpts(opts)...).
				Find(&models.OutputBasket{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.OutputBasket{}).
			Select("*").
			Joins(fmt.Sprintf("LEFT JOIN %s as num on num.table_name = ? and num.string_id = name", s.name.numericIDLookupTableName), s.name.outputBasketTableName).
			Scopes(scopes.UserID(userID)).
			Scopes(scopes.FromQueryOpts(opts)...).
			Find(&outputBaskets).Error
		if err != nil {
			return fmt.Errorf("failed to find output baskets for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(outputBaskets, s.mapModelToTableOutputBasket), nil
}

func (s *Sync) mapModelToTableOutputBasket(model *OutputBasketWithNum) *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		BasketID:  must.ConvertToInt(model.NumID),
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

func (s *Sync) upsertNumericIDLookup(ctx context.Context, tx *gorm.DB, stringIDsQuery func(db *gorm.DB) *gorm.DB) error {
	dry := s.db.Session(&gorm.Session{DryRun: true})
	query := stringIDsQuery(dry)

	clauses := []clause.Expression{
		clause.Expr{SQL: "INSERT INTO " + s.name.numericIDLookupTableName + " (table_name, string_id) "},
		clause.Expr{SQL: query.Statement.SQL.String(), Vars: query.Statement.Vars},
		clause.Expr{SQL: " ON CONFLICT DO NOTHING"},
	}

	for _, c := range clauses {
		c.Build(dry.Statement)
	}

	err := tx.WithContext(ctx).Exec(dry.Statement.SQL.String(), dry.Statement.Vars...).Error
	if err != nil {
		return fmt.Errorf("failed to create numeric ID lookup rows: %w", err)
	}

	return nil
}
