package repo

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const basketStringIDClause = "CONCAT(user_id, '.', name)"

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

type OutputBasketWithNum struct {
	models.OutputBasket
	NumID int
}

func (s *Sync) FindBasketsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutputBasket, error) {
	var resultModels []*OutputBasketWithNum

	err := s.db.Transaction(func(tx *gorm.DB) error {
		filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

		err := s.upsertNumericIDLookup(ctx, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", basketStringIDClause), s.naming.outputBasketTableName).
				Scopes(filters...).
				Find(&models.OutputBasket{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.OutputBasket{}).
			Select("*").
			Scopes(s.joinWithNumericIDLookupScope(basketStringIDClause, s.naming.outputBasketTableName)).
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

type ProvenTxReqWithNum struct {
	models.ProvenTxReq
	NumID int
}

func (s *Sync) FindProvenTxsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableProvenTxReq, []*wdk.TableProvenTx, error) {
	var resultModels []*ProvenTxReqWithNum

	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := s.upsertNumericIDLookup(ctx, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select("?, tx_id", s.naming.provenTxReqTableName).
				Scopes(scopes.FromQueryOpts(opts)...).
				Scopes(s.provenTxWhereExistsScope(userID)).
				Find(&models.ProvenTxReq{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.ProvenTxReq{}).
			Select("*").
			Scopes(s.joinWithNumericIDLookupScope("tx_id", s.naming.provenTxReqTableName)).
			Scopes(scopes.FromQueryOpts(opts)...).
			Scopes(s.provenTxWhereExistsScope(userID)).
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find proven tx requests for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableProvenTxReq), s.toApplicableProvenTxs(resultModels), nil
}

func (s *Sync) mapModelToTableProvenTxReq(model *ProvenTxReqWithNum) *wdk.TableProvenTxReq {
	return &wdk.TableProvenTxReq{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		ProvenTxReqID: model.NumID,
		ProvenTxID:    to.IfThen(model.HasMerklePath(), to.Ptr(model.NumID)).ElseThen(nil),
		Status:        model.Status,
		Attempts:      model.Attempts,
		Notified:      model.Notified,
		TxID:          model.TxID,
		Batch:         nil, // TODO: For now batch broadcasting is not supported, will be added later
		History:       "",  // TODO: History feature will be reworked later, then we can address this and think if we even want to sync "history" field
		Notify:        "",  // TODO: Notify includes transaction IDs and they are only used by JS-version of the wallet, so we can ignore it for now
		RawTx:         model.RawTx,
		InputBEEF:     model.InputBeef,
	}
}

func (s *Sync) mapModelToTableProvenTx(model *ProvenTxReqWithNum) *wdk.TableProvenTx {
	if !model.HasMerklePath() {
		return nil // If the model does not have a Merkle path, we do not create a TableProvenTx entry
	}

	if model.BlockHeight == nil || model.MerkleRoot == nil || model.BlockHash == nil {
		// if HasMerklePath() is true, it must have BlockHeight, MerkleRoot, and BlockHash set
		// this should never happen, but if it does, we panic to indicate a programming error
		panic("ProvenTxReq model must have BlockHeight, MerkleRoot, and BlockHash set when creating TableProvenTx")
	}

	return &wdk.TableProvenTx{
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
		ProvenTxID: model.NumID,
		TxID:       model.TxID,
		Height:     *model.BlockHeight,
		Index:      0, // TODO: JS version also contains an index, it could be done in separate task later
		MerklePath: model.MerklePath,
		RawTx:      model.RawTx,
		BlockHash:  *model.BlockHash,
		MerkleRoot: *model.MerkleRoot,
	}
}

// toApplicableProvenTxs produced a slice of TableProvenTx
// NOTE: In this implementation, there is only one table to hold requests (ProvenTxReq) and proven transactions (ProvenTx).
// This logic deduces if a transaction is MINED (has a Merkle path) - if so, it creates a TableProvenTx entry.
func (s *Sync) toApplicableProvenTxs(models []*ProvenTxReqWithNum) []*wdk.TableProvenTx {
	mappedSeq := seq.Map(seq.FromSlice(models), s.mapModelToTableProvenTx)
	provenTxs := seq.Filter(mappedSeq, notNil)
	return seq.Collect(provenTxs)
}

func (s *Sync) provenTxWhereExistsScope(userID int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		whereExistClause := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s as user_tx WHERE user_tx.tx_id = %s.tx_id AND user_tx.user_id = ?)",
			s.naming.transactionsTableName,
			s.naming.provenTxReqTableName,
		)

		return db.Where(whereExistClause, userID)
	}
}

// upsertNumericIDLookup inserts string IDs into the numeric ID lookup table to ensure each string ID has a corresponding numeric ID.
// It executes custom INSERT ... SELECT ... ON CONFLICT DO NOTHING based on the result of the provided stringIDsQuery function.
func (s *Sync) upsertNumericIDLookup(ctx context.Context, tx *gorm.DB, stringIDsQuery func(db *gorm.DB) *gorm.DB) error {
	dry := s.db.Session(&gorm.Session{DryRun: true, Initialized: true}) // NOTICE: Initialized to separate the dry run from the actual transaction (this makes the Session to clone the Statement)
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

func notNil[T any](v *T) bool {
	return v != nil
}
