package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo/syncrepo"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Sync struct {
	*syncrepo.SyncBasket
	*syncrepo.SyncKnownTx
	db *gorm.DB

	naming *naming
}

func NewSync(db *gorm.DB) *Sync {
	return &Sync{
		db:     db,
		naming: newNaming(db),

		SyncBasket:  syncrepo.NewSyncBasket(db),
		SyncKnownTx: syncrepo.NewSyncKnownTx(db),
	}
}

type KnownTxWithNum struct {
	models.KnownTx
	NumID int
}

type TransactionWithKnownTx struct {
	models.Transaction
	KnownTxNumID *int `gorm:"column:num_id"`
	BlockHeight  *uint32
}

func (s *Sync) FindTransactionsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTransaction, error) {
	var resultModels []*TransactionWithKnownTx

	err := s.db.Transaction(func(tx *gorm.DB) error {
		queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
			if options.Since != nil && options.Since.TableName == "" {
				// Prevent from an issue with ambiguous created_at column
				options.Since.TableName = s.naming.transactionsTableName
			}
		})
		filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

		// Make sure all numeric IDs of KnownTxs needed by user's transactions are present in the numeric ID lookup table.
		err := s.upsertNumericIDLookup(ctx, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select("?, tx_id", s.naming.knownTxTableName).
				Scopes(filters...).
				Where("tx_id IS NOT NULL").
				Find(&models.Transaction{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.Transaction{}).
			Select(fmt.Sprintf("%s.*, num.num_id, known_tx.block_height", s.naming.transactionsTableName)).
			Scopes(s.joinWithNumericIDLookupScope(fmt.Sprintf("%s.tx_id", s.naming.transactionsTableName), s.naming.knownTxTableName, clause.LeftJoin)).
			Joins(fmt.Sprintf("LEFT JOIN %s as known_tx ON known_tx.tx_id = %s.tx_id", s.naming.knownTxTableName, s.naming.transactionsTableName)).
			Scopes(filters...).
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find proven tx requests for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableTransaction), nil
}

type OutputReadModel struct {
	models.Output
	BasketNumID *int `gorm:"column:basket_num_id"`
}

func (s *Sync) FindOutputsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableOutput, error) {
	const basketStringIDClause = "CONCAT(user_id, '.', basket_name)"
	var resultModels []*OutputReadModel

	err := s.db.Transaction(func(tx *gorm.DB) error {
		queryopts.ModifyOptions(opts, func(options *queryopts.Options) {
			if options.Since != nil && options.Since.TableName == "" {
				// Prevent from an issue with ambiguous created_at column
				options.Since.TableName = s.naming.outputsTableName
			}
		})
		filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

		// Make sure all numeric IDs of OutputBaskets needed by user's outputs are present in the numeric ID lookup table.
		err := s.upsertNumericIDLookup(ctx, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", basketStringIDClause), s.naming.outputBasketTableName).
				Scopes(filters...).
				Where("basket_name IS NOT NULL").
				Find(&models.Output{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.Output{}).
			Select(fmt.Sprintf("%s.*, num.num_id as basket_num_id", s.naming.outputsTableName)).
			Scopes(filters...).
			Scopes(s.joinWithNumericIDLookupScope(basketStringIDClause, s.naming.outputBasketTableName, clause.LeftJoin)).
			Preload("Transaction", func(db *gorm.DB) *gorm.DB {
				return db.Select("id, tx_id")
			}).
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find outputs for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableOutput), nil
}

func (s *Sync) mapModelToTableOutput(model *OutputReadModel) *wdk.TableOutput {
	return &wdk.TableOutput{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		OutputID:           model.ID,
		UserID:             model.UserID,
		TransactionID:      model.TransactionID,
		Spendable:          model.Spendable,
		Change:             model.Change,
		OutputDescription:  model.Description,
		Vout:               model.Vout,
		Satoshis:           model.Satoshis,
		ProvidedBy:         model.ProvidedBy,
		Purpose:            model.Purpose,
		Type:               model.Type,
		TxID:               to.IfThen(model.Transaction != nil, model.Transaction.TxID).ElseThen(nil),
		DerivationPrefix:   model.DerivationPrefix,
		DerivationSuffix:   model.DerivationSuffix,
		CustomInstructions: model.CustomInstructions,
		LockingScript:      model.LockingScript,
		SenderIdentityKey:  model.SenderIdentityKey,
		BasketID:           model.BasketNumID,
		SpentBy:            model.SpentBy,
	}
}

func (s *Sync) mapModelToTableTransaction(model *TransactionWithKnownTx) *wdk.TableTransaction {
	return &wdk.TableTransaction{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		TransactionID: model.ID,
		UserID:        model.UserID,
		Status:        model.Status,
		Reference:     primitives.Base64String(model.Reference),
		IsOutgoing:    model.IsOutgoing,
		Satoshis:      model.Satoshis,
		Description:   model.Description,
		Version:       &model.Version,
		LockTime:      &model.LockTime,
		TxID:          model.TxID,
		InputBEEF:     model.InputBeef,

		//NOTE: ProvenTxID is set only if the transaction is known to be mined (has a numeric ID in the KnownTx table).
		ProvenTxID: to.IfThen(model.BlockHeight != nil, model.KnownTxNumID).ElseThen(nil),
	}
}

func (s *Sync) provenTxWhereExistsScope(userID int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		whereExistClause := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s as user_tx WHERE user_tx.tx_id = %s.tx_id AND user_tx.user_id = ?)",
			s.naming.transactionsTableName,
			s.naming.knownTxTableName,
		)

		return db.Where(whereExistClause, userID)
	}
}

// UpsertKnownTxForSync updates only non-zero fields of the proven transaction request.
func (s *Sync) UpsertKnownTxForSync(ctx context.Context, entity *entity.KnownTx) (isNew bool, err error) {
	model := models.KnownTx{
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
		TxID:        entity.TxID,
		Status:      entity.Status,
		Attempts:    entity.Attempts,
		Notified:    entity.Notified,
		RawTx:       entity.RawTx,
		InputBeef:   entity.InputBEEF,
		BlockHeight: entity.BlockHeight,
		MerklePath:  entity.MerklePath,
		MerkleRoot:  entity.MerkleRoot,
		BlockHash:   entity.BlockHash,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.KnownTx{}).
			Where("tx_id = ?", entity.TxID).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update proven tx req: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create proven tx req: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, nil
}

func (s *Sync) UpsertTransactionForSync(ctx context.Context, entity *entity.Transaction) (isNew bool, transactionID uint, err error) {
	model := models.Transaction{
		Model: gorm.Model{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		UserID:      entity.UserID,
		Status:      entity.Status,
		Reference:   entity.Reference,
		IsOutgoing:  entity.IsOutgoing,
		Satoshis:    entity.Satoshis,
		Description: entity.Description,
		Version:     entity.Version,
		LockTime:    entity.LockTime,
		TxID:        entity.TxID,
		InputBeef:   entity.InputBEEF,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Transaction{}).
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
			Scopes(scopes.UserID(entity.UserID)).
			Where("reference = ?", entity.Reference).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update transaction: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			resultTxModel := models.Transaction{}
			if err = updateTx.Scan(&resultTxModel).Error; err != nil {
				return fmt.Errorf("failed to scan updated transaction: %w", err)
			}

			transactionID = resultTxModel.ID
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		isNew = true
		transactionID = model.ID

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, transactionID, nil
}

func (s *Sync) UpsertOutputForSync(ctx context.Context, entity *entity.Output) (isNew bool, outputID uint, err error) {
	model := models.Output{
		Model: gorm.Model{
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		},
		UserID:             entity.UserID,
		TransactionID:      entity.TransactionID,
		SpentBy:            entity.SpentBy,
		Satoshis:           entity.Satoshis,
		Description:        entity.Description,
		Vout:               entity.Vout,
		LockingScript:      entity.LockingScript,
		CustomInstructions: entity.CustomInstructions,
		DerivationPrefix:   entity.DerivationPrefix,
		DerivationSuffix:   entity.DerivationSuffix,
		BasketName:         entity.BasketName,
		Spendable:          entity.Spendable,
		Change:             entity.Change,
		Purpose:            entity.Purpose,
		Type:               entity.Type,
		SenderIdentityKey:  entity.SenderIdentityKey,
	}

	if entity.UserUTXO != nil {
		model.UserUTXO = &models.UserUTXO{
			UserID:             entity.UserUTXO.UserID,
			OutputID:           entity.UserUTXO.OutputID,
			BasketName:         entity.UserUTXO.BasketName,
			Satoshis:           entity.UserUTXO.Satoshis,
			EstimatedInputSize: entity.UserUTXO.EstimatedInputSize,
			CreatedAt:          entity.UserUTXO.CreatedAt,
			ReservedByID:       entity.UserUTXO.ReservedByID,
		}
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.Output{}).
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
			Where("user_id = ? AND transaction_id = ? AND vout = ?", model.UserID, model.TransactionID, model.Vout).
			Select("*").
			Updates(model)

		// NOTE: We use `Select("*")` with `Updates()` to ensure that all fields are updated, including those that might be zero values (e.g., BasketName for relinquished outputs).

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update output: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			resultOutputModel := models.Output{}
			if err = updateTx.Scan(&resultOutputModel).Error; err != nil {
				return fmt.Errorf("failed to scan updated output: %w", err)
			}

			outputID = resultOutputModel.ID
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create output: %w", err)
		}

		isNew = true
		outputID = model.ID

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, outputID, nil
}

type LabelReadModel struct {
	models.Label
	NumID uint
}

func (s *Sync) FindLabelsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabel, error) {
	const labelStringIDClause = "CONCAT(user_id, '.', name)"
	var resultModels []*LabelReadModel

	err := s.db.Transaction(func(tx *gorm.DB) error {
		filters := append(scopes.FromQueryOpts(opts), scopes.UserID(userID))

		err := s.upsertNumericIDLookup(ctx, tx, func(db *gorm.DB) *gorm.DB {
			return db.
				Select(fmt.Sprintf("?, %s", labelStringIDClause), s.naming.labelsTableName).
				Scopes(filters...).
				Unscoped().
				Find(&models.Label{})
		})
		if err != nil {
			return err
		}

		err = tx.WithContext(ctx).
			Model(&models.Label{}).
			Select("*").
			Scopes(filters...).
			Scopes(s.joinWithNumericIDLookupScope(labelStringIDClause, s.naming.labelsTableName, clause.InnerJoin)).
			Unscoped().
			Find(&resultModels).Error
		if err != nil {
			return fmt.Errorf("failed to find labels for sync: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableTxLabel), nil
}

func (s *Sync) mapModelToTableTxLabel(model *LabelReadModel) *wdk.TableTxLabel {
	return &wdk.TableTxLabel{
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
		TxLabelID: model.NumID,
		UserID:    model.UserID,
		Label:     model.Name,
		IsDeleted: model.DeletedAt.Valid,
	}
}

func (s *Sync) UpsertLabelForSync(ctx context.Context, entity *entity.Label) (isNew bool, labelNumID uint, err error) {
	model := models.Label{
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
		UserID:    entity.UserID,
		Name:      entity.Name,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		numID, err := s.saveNumericIDForLabel(ctx, tx, entity.UserID, entity.Name)
		if err != nil {
			return err
		}

		labelNumID = numID

		updateTx := tx.Model(&models.Label{}).
			Where("user_id = ? AND name = ?", entity.UserID, model.Name).
			Updates(model)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update label: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err = tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create label: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, labelNumID, nil
}

func (s *Sync) DeleteLabelForSync(ctx context.Context, entity *entity.Label) (deleted bool, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txDelete := tx.Delete(
			&models.Label{},
			"user_id = ? AND name = ?", entity.UserID, entity.Name,
		)
		if txDelete.Error != nil {
			return fmt.Errorf("failed to delete label: %w", err)
		}

		deleted = txDelete.RowsAffected > 0

		err = tx.Delete(
			&models.TransactionLabel{},
			"label_user_id = ? AND label_name = ?", entity.UserID, entity.Name,
		).Error
		if err != nil {
			return fmt.Errorf("failed to delete label map entries: %w", err)
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return deleted, nil
}

type LabelsMapReadModel struct {
	models.TransactionLabel
	NumID uint
}

func (s *Sync) FindLabelsMapForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableTxLabelMap, error) {
	const labelStringIDClause = "CONCAT(label_user_id, '.', label_name)"
	var resultModels []*LabelsMapReadModel

	err := s.db.WithContext(ctx).
		Model(models.TransactionLabel{}).
		Select(fmt.Sprintf("%s.*, num_id", s.naming.labelsMapTableName)).
		Scopes(scopes.FromQueryOpts(opts)...).
		Scopes(s.joinWithNumericIDLookupScope(labelStringIDClause, s.naming.labelsTableName, clause.InnerJoin)).
		Where("label_user_id = ?", userID).
		Unscoped().
		Find(&resultModels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find labels map for sync: %w", err)
	}

	return slices.Map(resultModels, s.mapModelToTableTxLabelMap), nil
}

func (s *Sync) mapModelToTableTxLabelMap(model *LabelsMapReadModel) *wdk.TableTxLabelMap {
	return &wdk.TableTxLabelMap{
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		TransactionID: model.TransactionID,
		TxLabelID:     model.NumID,
		IsDeleted:     model.DeletedAt.Valid,
	}
}

func (s *Sync) FindLabelByNumIDForSync(ctx context.Context, labelNumID uint) (*entity.Label, error) {
	const labelStringIDClause = "CONCAT(user_id, '.', name)"
	var label models.Label

	err := s.db.WithContext(ctx).Model(&models.Label{}).
		Scopes(s.joinWithNumericIDLookupScope(labelStringIDClause, s.naming.labelsTableName, clause.InnerJoin)).
		Where("num.num_id = ?", labelNumID).
		First(&label).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find label name by numeric ID: %w", err)
	}

	return &entity.Label{
		CreatedAt: label.CreatedAt,
		UpdatedAt: label.UpdatedAt,
		UserID:    label.UserID,
		Name:      label.Name,
	}, nil
}

func (s *Sync) UpsertLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (isNew bool, err error) {
	model := models.TransactionLabel{
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
		TransactionID: entity.TransactionID,
		LabelUserID:   entity.UserID,
		LabelName:     entity.Name,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.TransactionLabel{}).
			Where("transaction_id = ? AND label_user_id = ? AND label_name = ?", model.TransactionID, model.LabelUserID, model.LabelName).
			UpdateColumn("updated_at", model.UpdatedAt)

		if updateTx.Error != nil {
			return fmt.Errorf("failed to update label map: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create label map: %w", err)
		}

		isNew = true

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return isNew, nil
}

func (s *Sync) DeleteLabelMapForSync(ctx context.Context, entity *entity.LabelMap) (deleted bool, err error) {
	txDelete := s.db.WithContext(ctx).Delete(
		&models.TransactionLabel{},
		"transaction_id = ? AND label_user_id = ? AND label_name = ?", entity.TransactionID, entity.UserID, entity.Name,
	)
	if txDelete.Error != nil {
		return false, fmt.Errorf("failed to delete label: %w", err)
	}

	deleted = txDelete.RowsAffected > 0
	return deleted, nil
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
func (s *Sync) joinWithNumericIDLookupScope(stringIDClause string, entityName string, join clause.JoinType) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		joinQuery := fmt.Sprintf("%s JOIN %s as num on num.table_name = ? and num.string_id = %s", join, s.naming.numericIDLookupTableName, stringIDClause)

		return db.Joins(joinQuery, entityName)
	}
}
func (s *Sync) findNumericIDLookup(ctx context.Context, tx *gorm.DB, tableName string, stringID string) (uint, error) {
	var numericID uint
	txScan := tx.WithContext(ctx).
		Model(&models.NumericIDLookup{}).
		Select("num_id").
		Where("table_name = ? AND string_id = ?", tableName, stringID).
		Scan(&numericID)
	if txScan.Error != nil {
		return 0, fmt.Errorf("failed to find numeric ID for %s: %w", stringID, txScan.Error)
	}

	if txScan.RowsAffected == 0 {
		return 0, fmt.Errorf("numeric ID not found for %s", stringID)
	}

	return numericID, nil
}

func (s *Sync) saveNumericIDLookup(ctx context.Context, tx *gorm.DB, tableName string, stringID string) error {
	stringIDLookup := &models.NumericIDLookup{
		TableName: tableName,
		StringID:  stringID,
	}

	err := tx.
		WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(stringIDLookup).Error
	if err != nil {
		return fmt.Errorf("failed to save numeric ID lookup for %s: %w", stringID, err)
	}

	return nil
}

func (s *Sync) saveNumericIDForOutputBasket(ctx context.Context, tx *gorm.DB, userID int, basketName string) (uint, error) {
	stringID := fmt.Sprintf("%d.%s", userID, basketName)

	err := s.saveNumericIDLookup(ctx, tx, s.naming.outputBasketTableName, stringID)
	if err != nil {
		return 0, fmt.Errorf("failed to save numeric ID lookup for output basket: %w", err)
	}

	return s.findNumericIDLookup(ctx, tx, s.naming.outputBasketTableName, stringID)
}

func (s *Sync) saveNumericIDForLabel(ctx context.Context, tx *gorm.DB, userID int, labelName string) (uint, error) {
	stringID := fmt.Sprintf("%d.%s", userID, labelName)

	err := s.saveNumericIDLookup(ctx, tx, s.naming.labelsTableName, stringID)
	if err != nil {
		return 0, fmt.Errorf("failed to save numeric ID lookup for label: %w", err)
	}

	return s.findNumericIDLookup(ctx, tx, s.naming.labelsTableName, stringID)
}
