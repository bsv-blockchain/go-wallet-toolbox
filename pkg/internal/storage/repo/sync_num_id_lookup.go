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
	"gorm.io/gorm/clause"
)

type Sync struct {
	db *gorm.DB

	naming *naming
}

func NewSync(db *gorm.DB) *Sync {
	return &Sync{
		db:     db,
		naming: newNaming(db),
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
	NumID int
}

func (s *Sync) FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error) {
	var resultModels []*OutputBasketWithNum

	err := s.db.Transaction(func(tx *gorm.DB) error {
		filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

		stringIDClause := "CONCAT(user_id, '.', name)"

		err := s.upsertNumericIDLookup(ctx, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", stringIDClause), s.naming.outputBasketTableName).
				Scopes(filters...).
				Find(&models.OutputBasket{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.OutputBasket{}).
			Select("*").
			Scopes(s.joinWithNumericIDLookupScope(stringIDClause, s.naming.outputBasketTableName)).
			Scopes(filters...).
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find output baskets for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableOutputBasket), nil
}

func (s *Sync) mapModelToTableOutputBasket(model *OutputBasketWithNum) *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		BasketID:  model.NumID,
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

// upsertNumericIDLookup inserts string IDs into the numeric ID lookup table to ensure each string ID has a corresponding numeric ID.
// It executes custom INSERT ... SELECT ... ON CONFLICT DO NOTHING based on the result of the provided stringIDsQuery function.
func (s *Sync) upsertNumericIDLookup(ctx context.Context, tx *gorm.DB, stringIDsQuery func(db *gorm.DB) *gorm.DB) error {
	dry := s.db.Session(&gorm.Session{DryRun: true, Initialized: true}) // Initialized to separate the dry run from the actual transaction (this makes the Session to clone the Statement)
	query := stringIDsQuery(dry)

	insertSelectClauses := []clause.Expression{
		clause.Expr{SQL: "INSERT INTO " + s.naming.numericIDLookupTableName + " (table_name, string_id) "},
		clause.Expr{SQL: query.Statement.SQL.String(), Vars: query.Statement.Vars},
		clause.Expr{SQL: " ON CONFLICT DO NOTHING"},
	}

	insertSelect := &gorm.Statement{DB: s.db}
	for _, c := range insertSelectClauses {
		c.Build(insertSelect)
	}

	err := tx.WithContext(ctx).Exec(insertSelect.SQL.String(), insertSelect.Vars...).Error
	if err != nil {
		return fmt.Errorf("failed to create numeric ID lookup rows: %w", err)
	}

	return nil
}

// joinWithNumericIDLookupScope returns a GORM scope to join a numeric ID lookup table based on the provided string ID clause.
// The entityName is used to specify the table_name of the entity, and the stringIDClause is used to match the string_id in the numeric ID lookup table.
func (s *Sync) joinWithNumericIDLookupScope(stringIDClause string, entityName string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		joinQuery := fmt.Sprintf("INNER JOIN %s as num on num.table_name = ? and num.string_id = %s", s.naming.numericIDLookupTableName, stringIDClause)

		return db.Joins(joinQuery, entityName)
	}
}
