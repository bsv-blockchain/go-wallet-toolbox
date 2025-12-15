package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Outputs struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewOutputs(db *gorm.DB, query *genquery.Query) *Outputs {
	return &Outputs{db: db, query: query}
}

type txIDsReadModel struct {
	TransactionID string `gorm:"column:tx_id"`
}

func (o *Outputs) FindTxIDsByOutputIDs(ctx context.Context, outputIDs iter.Seq[uint]) ([]string, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindTxIDsByOutputIDs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if seq.IsEmpty(outputIDs) {
		return nil, nil
	}

	var txIDsModel []*txIDsReadModel

	outTable := &o.query.Output
	txTable := &o.query.Transaction
	idsClause := seq.Collect(outputIDs)

	err = outTable.
		WithContext(ctx).
		Distinct(txTable.TxID).
		Join(txTable, txTable.ID.EqCol(outTable.TransactionID)).
		Where(outTable.ID.In(idsClause...)).
		Scan(&txIDsModel)

	if err != nil {
		return nil, fmt.Errorf("failed to find outputs: %w", err)
	}

	txIDs := slices.Map(txIDsModel, func(rm *txIDsReadModel) string {
		return rm.TransactionID
	})
	return txIDs, nil
}

func (o *Outputs) FindOutputsByIDs(ctx context.Context, outputIDs iter.Seq[uint]) ([]*pkgentity.Output, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindOutputsByIDs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if seq.IsEmpty(outputIDs) {
		return nil, nil
	}

	idsClause := seq.Collect(outputIDs)

	var outputs []*models.Output
	err = o.db.WithContext(ctx).
		Model(models.Output{}).
		Preload("Transaction", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, tx_id")
		}).
		Where("id IN ?", idsClause).
		Find(&outputs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find outputs: %w", err)
	}

	return slices.Map(outputs, o.mapModelToOutputEntity), nil
}

func needsTransactionJoin(spec *pkgentity.OutputReadSpecification) bool {
	return spec != nil && (spec.TxID != nil || spec.TxStatus != nil)
}

func (o *Outputs) FindOutputs(ctx context.Context, spec *pkgentity.OutputReadSpecification, opts ...queryopts.Options) ([]*pkgentity.Output, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindOutputs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	output := o.query.Output
	tx := o.query.Transaction
	outputPtr := &output

	dao := output.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(outputPtr, opts)...).
		Preload(output.Transaction).
		Where(o.conditionsBySpec(ctx, spec)...)

	if needsTransactionJoin(spec) {
		dao = dao.
			Select(
				output.ALL,
				tx.TxID,
				tx.Status,
			).
			Join(tx, tx.ID.EqCol(output.TransactionID))
	}

	rows, err := dao.Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs: %w", err)
	}

	return slices.Map(rows, o.mapModelToOutputEntity), nil
}

func (o *Outputs) FindOutputsByTransactionID(ctx context.Context, transactionID uint) ([]*pkgentity.Output, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindOutputsByTransactionID", attribute.String("TxID", fmt.Sprintf("%d", transactionID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	session := o.db.WithContext(ctx)

	var outputRows []*models.Output
	err = session.
		Model(models.Output{}).
		Where("transaction_id = ?", transactionID).
		Find(&outputRows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs for transactionID: %d: %w", transactionID, err)
	}

	return slices.Map(outputRows, o.mapModelToOutputEntity), nil
}

func (o *Outputs) ListAndCountOutputs(ctx context.Context, filter entity.ListOutputsFilter) ([]*pkgentity.Output, int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-ListAndCountOutputs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var outputs []*models.Output
	var total int64

	if err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.
			Model(&models.Output{}).
			Where("user_id = ?", filter.UserID).
			Preload("Transaction", func(db *gorm.DB) *gorm.DB {
				return db.Select("id, tx_id")
			})

		var omitFields []string

		if !filter.IncludeLockingScripts {
			omitFields = append(omitFields, "locking_script")
		}

		if !filter.IncludeCustomInstructions {
			omitFields = append(omitFields, "custom_instructions")
		}

		if len(omitFields) > 0 {
			query = query.Omit(omitFields...)
		}

		if !filter.IncludeSpent {
			query = query.Where(o.query.Output.Spendable.Value(true))
		}

		if filter.Basket != "" {
			query = query.Where("basket_name = ?", filter.Basket)
		}

		if filter.IncludeTags {
			query = query.Preload("Tags")
		}

		if len(filter.Tags) > 0 {
			query = query.Scopes(o.tagFilterScope(tx, filter))
		}

		allowedStatuses := []wdk.TxStatus{
			wdk.TxStatusCompleted, wdk.TxStatusUnprocessed, wdk.TxStatusSending, wdk.TxStatusUnproven,
			wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal,
		}
		query = query.Where("transaction_id IN (?)",
			tx.Model(&models.Transaction{}).
				Select("id").
				Where("user_id = ?", filter.UserID).
				Where("status IN ?", allowedStatuses),
		)

		if err := query.Count(&total).Error; err != nil {
			return fmt.Errorf("count failed: %w", err)
		}

		if err := query.Limit(filter.Limit).Offset(filter.Offset).Order("bsv_outputs.id ASC").Find(&outputs).Error; err != nil {
			return fmt.Errorf("query failed: %w", err)
		}
		return nil
	}); err != nil {
		return nil, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(outputs, o.mapModelToOutputEntity), total, nil
}

func (o *Outputs) UnlinkOutputFromBasketByOutpoint(ctx context.Context, userID int, basketName *string, outpoint wdk.OutPoint) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-UnlinkOutputFromBasketByOutpoint", attribute.Int("UserID", userID), attribute.String("TxID", outpoint.TxID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.Output{}).
			Select("id").
			Scopes(scopes.UserID(userID)).
			Where("vout = ?", outpoint.Vout).
			Where("transaction_id IN (?)",
				tx.Model(&models.Transaction{}).
					Select("id").
					Scopes(scopes.UserID(userID)).
					Where("tx_id = ?", outpoint.TxID),
			)

		if basketName != nil {
			query = query.Where("basket_name = ?", *basketName)
		}

		var output models.Output
		if err := query.First(&output).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				var basketMsg string
				if basketName != nil {
					basketMsg = fmt.Sprintf(" for basket: %s", *basketName)
				}
				return fmt.Errorf("no output found with vout %d and txid %s%s", outpoint.Vout, outpoint.TxID, basketMsg)
			}

			return fmt.Errorf("failed to fetch outputs for unlink: %w", err)
		}

		result := tx.Model(&models.Output{}).
			Where("id = ?", output.ID).
			Update("basket_name", nil)

		if result.Error != nil {
			return fmt.Errorf("failed to unlink output from basket: %w", result.Error)
		}

		err := tx.Delete(models.UserUTXO{}, "reserved_by_id IS NULL and output_id = ?", output.ID).Error
		if err != nil {
			return fmt.Errorf("failed to delete user utxo for output %d (it can be reserved): %w", output.ID, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to unlink output from basket: %w", err)
	}

	return nil
}

func (o *Outputs) FindOutputsByOutpoints(ctx context.Context, userID int, outpoints []wdk.OutPoint) ([]*pkgentity.Output, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindOutputsByOutpoints", attribute.Int("UserID", userID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(outpoints) == 0 {
		return nil, nil
	}

	outpointStrings := slices.Map(outpoints, func(op wdk.OutPoint) []any {
		return []any{op.TxID, op.Vout}
	})
	outputTableName := o.query.Output.TableName()
	transactionTableName := o.query.Transaction.TableName()
	query := o.db.WithContext(ctx).Table(
		"(?) as out",
		o.db.Model(&models.Output{}).
			Select(fmt.Sprintf("%s.*, tx.tx_id as tx_id", outputTableName)).
			Joins(fmt.Sprintf("INNER JOIN %s tx ON tx.id = %s.transaction_id", transactionTableName, outputTableName)).
			Where(fmt.Sprintf("%s.user_id = ?", outputTableName), userID),
	).Where("(tx_id,vout) IN (?)", outpointStrings)

	type outputWithTxID struct {
		*models.Output
		TxID *string
	}

	var readModels []*outputWithTxID
	if err = query.Find(&readModels).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
			return nil, nil
		}

		return nil, fmt.Errorf("failed to fetch outputs: %w", err)
	}

	return slices.Map(readModels, func(readModel *outputWithTxID) *pkgentity.Output {
		readModel.Transaction = &models.Transaction{
			TxID: readModel.TxID,
		}
		return o.mapModelToOutputEntity(readModel.Output)
	}), nil
}

func (o *Outputs) FindOutput(ctx context.Context, userID int, outpoint wdk.OutPoint) (*pkgentity.Output, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindOutput", attribute.Int("UserID", userID), attribute.String("TxID", outpoint.TxID), attribute.String("Vout", fmt.Sprintf("%d", outpoint.Vout)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var output models.Output
	err = o.db.WithContext(ctx).
		Model(&models.Output{}).
		Scopes(scopes.UserID(userID)).
		Where("vout = ?", outpoint.Vout).
		Where("transaction_id IN (?)",
			o.db.Model(&models.Transaction{}).
				Select("id").
				Scopes(scopes.UserID(userID)).
				Where("tx_id = ?", outpoint.TxID),
		).
		First(&output).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find output: %w", err)
	}

	tableOutput := o.mapModelToOutputEntity(&output)
	tableOutput.TxID = &outpoint.TxID
	return tableOutput, nil
}

// FindInputsAndOutputsWithBaskets retrieves inputs and outputs for given transaction IDs, including basket information.
// It returns two maps: one for inputs keyed by SpentBy ID and another for outputs keyed by TransactionID.
// Each map contains slices of TableOutput, which include basket details if available.
func (o *Outputs) FindInputsAndOutputsWithBaskets(ctx context.Context, txIDs []uint, includeLockingScripts bool) (inputs map[uint][]*pkgentity.Output, outputs map[uint][]*pkgentity.Output, err error) {
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindInputsAndOutputsWithBaskets")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return
	}

	query := o.db.WithContext(ctx).
		Model(&models.Output{}).
		Preload("Transaction", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, tx_id")
		}).
		Preload("Basket").
		Preload("Tags").
		Where("transaction_id IN ? OR spent_by IN ?", txIDs, txIDs)

	if !includeLockingScripts {
		query = query.Omit("locking_script")
	}

	var allOutputs []*models.Output
	if err := query.Find(&allOutputs).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch inputs/outputs: %w", err)
	}

	inputMap := make(map[uint][]*pkgentity.Output)
	outputMap := make(map[uint][]*pkgentity.Output)

	for _, out := range allOutputs {
		tableOut := o.mapModelToOutputEntity(out)
		if out.SpentBy != nil {
			inputMap[*out.SpentBy] = append(inputMap[*out.SpentBy], tableOut)
		}
		outputMap[out.TransactionID] = append(outputMap[out.TransactionID], tableOut)
	}

	return inputMap, outputMap, nil
}

// FindInputsAndOutputsForSelectedActions retrieves inputs and outputs for the current page of actions
// using JOINs against the selected actions subquery, avoiding large IN clauses and extra preloads.
func (o *Outputs) FindInputsAndOutputsForSelectedActions(ctx context.Context, userID int, filter entity.ListActionsFilter, includeLockingScripts bool) (map[uint][]*pkgentity.Output, map[uint][]*pkgentity.Output, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindInputsAndOutputsForSelectedActions", attribute.Int("UserID", userID), attribute.Bool("IncludeLockingScripts", includeLockingScripts))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var inMap, outMap map[uint][]*pkgentity.Output
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		selected := o.selectedActionsSubquery(tx, userID, filter)
		dbq := o.buildOutputsJoinQuery(tx, selected, userID, includeLockingScripts)

		rows, err := dbq.Rows()
		if err != nil {
			return fmt.Errorf("failed to fetch inputs/outputs via joins: %w", err)
		}
		var closeErr error
		defer func() {
			if cerr := rows.Close(); cerr != nil {
				closeErr = fmt.Errorf("rows close failed: %w", cerr)
			}
		}()

		inMap, outMap, err = o.readOutputsIntoMaps(tx, rows)
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		return nil, nil, fmt.Errorf("transaction failed in FindInputsAndOutputsForSelectedActions: %w", err)
	}

	return inMap, outMap, nil
}

// selectedActionsSubquery returns the current page of action IDs with applied filters
func (o *Outputs) selectedActionsSubquery(tx *gorm.DB, userID int, filter entity.ListActionsFilter) *gorm.DB {
	selected := tx.Model(&models.Transaction{}).
		Select("id").
		Where("user_id = ?", userID)
	if len(filter.Status) > 0 {
		selected = selected.Where("status IN ?", filter.Status)
	}
	if len(filter.Labels) > 0 {
		subQuery := tx.Model(&models.TransactionLabel{}).
			Select("transaction_id").
			Where("label_name IN ?", filter.Labels).
			Where("label_user_id = ?", userID)
		if filter.LabelQueryMode == defs.QueryModeAll {
			subQuery = subQuery.Group("transaction_id").Having("COUNT(DISTINCT label_name) = ?", len(filter.Labels))
		}
		selected = selected.Where("id IN (?)", subQuery)
	}
	return selected.Order("id ASC").Limit(filter.Limit).Offset(filter.Offset)
}

// buildOutputsJoinQuery constructs the JOIN query to fetch outputs (and tags) for selected actions
func (o *Outputs) buildOutputsJoinQuery(tx *gorm.DB, selected *gorm.DB, userID int, includeLockingScripts bool) *gorm.DB {
	outputTable := o.query.Output.TableName()
	txTable := o.query.Transaction.TableName()
	otTable := o.query.OutputTag.TableName()
	tagsTable := o.query.Tag.TableName()

	dbq := tx.
		Table(outputTable+" o").
		Joins("JOIN (?) s ON s.id = o.transaction_id OR s.id = o.spent_by", selected).
		Joins("LEFT JOIN "+txTable+" t ON t.id = o.transaction_id").
		Joins("LEFT JOIN "+otTable+" ot ON ot.output_id = o.id AND ot.tag_user_id = o.user_id AND ot.deleted_at IS NULL").
		Joins("LEFT JOIN "+tagsTable+" tg ON tg.name = ot.tag_name AND tg.user_id = ot.tag_user_id AND tg.deleted_at IS NULL").
		Where("o.user_id = ?", userID).
		Where("o.deleted_at IS NULL").
		Order("o.id ASC").
		Select("o.*, t.tx_id as tx_id, tg.name as tag_name")

	if !includeLockingScripts {
		dbq = dbq.Omit("o.locking_script")
	}
	return dbq
}

// readOutputsIntoMaps scans streamed rows and groups them into input/output maps with tag de-duplication
func (o *Outputs) readOutputsIntoMaps(tx *gorm.DB, rows *sql.Rows) (map[uint][]*pkgentity.Output, map[uint][]*pkgentity.Output, error) {
	type readRow struct {
		models.Output
		TxID *string `gorm:"column:tx_id"`
		Tag  *string `gorm:"column:tag_name"`
	}

	inputMap := make(map[uint][]*pkgentity.Output)
	outputMap := make(map[uint][]*pkgentity.Output)
	tmpByID := make(map[uint]*pkgentity.Output)
	orderedIDs := make([]uint, 0)
	tagSeen := make(map[uint]map[string]struct{})

	for rows.Next() {
		var r readRow
		if err := tx.ScanRows(rows, &r); err != nil {
			return nil, nil, fmt.Errorf("scan failed: %w", err)
		}
		e := o.mapModelToOutputEntity(&r.Output)
		if r.TxID != nil {
			e.TxID = r.TxID
		}
		if prev, ok := tmpByID[e.ID]; ok {
			if r.Tag != nil {
				seen := tagSeen[e.ID]
				if seen == nil {
					seen = make(map[string]struct{})
					tagSeen[e.ID] = seen
				}
				if _, exists := seen[*r.Tag]; !exists {
					prev.Tags = append(prev.Tags, *r.Tag)
					seen[*r.Tag] = struct{}{}
				}
			}
			continue
		}
		if r.Tag != nil {
			e.Tags = append(e.Tags, *r.Tag)
			seen := tagSeen[e.ID]
			if seen == nil {
				seen = make(map[string]struct{})
				tagSeen[e.ID] = seen
			}
			seen[*r.Tag] = struct{}{}
		}
		tmpByID[e.ID] = e
		orderedIDs = append(orderedIDs, e.ID)
	}

	for _, id := range orderedIDs {
		e := tmpByID[id]
		if e.SpentBy != nil {
			inputMap[*e.SpentBy] = append(inputMap[*e.SpentBy], e)
		}
		outputMap[e.TransactionID] = append(outputMap[e.TransactionID], e)
	}
	return inputMap, outputMap, nil
}

func (o *Outputs) SaveOutputs(ctx context.Context, outputs []*pkgentity.Output) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-SaveOutputs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	type outputWithTags struct {
		Output models.Output
		Tags   []any
	}

	modelsToStore := slices.Map(outputs, func(output *pkgentity.Output) *outputWithTags {
		res := &outputWithTags{
			Output: models.Output{
				Model: gorm.Model{
					ID: output.ID,
				},
				UserID:             output.UserID,
				TransactionID:      output.TransactionID,
				SpentBy:            output.SpentBy,
				Vout:               output.Vout,
				Satoshis:           output.Satoshis,
				LockingScript:      output.LockingScript,
				CustomInstructions: output.CustomInstructions,
				DerivationPrefix:   output.DerivationPrefix,
				DerivationSuffix:   output.DerivationSuffix,
				BasketName:         output.BasketName,
				Spendable:          output.Spendable,
				Change:             output.Change,
				Description:        output.Description,
				ProvidedBy:         output.ProvidedBy,
				Purpose:            output.Purpose,
				Type:               output.Type,
				SenderIdentityKey:  output.SenderIdentityKey,
			},
			Tags: slices.Map(output.Tags, func(tag string) any {
				return &models.Tag{
					Name:   tag,
					UserID: output.UserID,
				}
			}),
		}

		if output.UserUTXO != nil {
			res.Output.UserUTXO = &models.UserUTXO{
				UserID:             output.UserUTXO.UserID,
				Satoshis:           output.UserUTXO.Satoshis,
				EstimatedInputSize: output.UserUTXO.EstimatedInputSize,
				UTXOStatus:         output.UserUTXO.Status,
			}
		}

		return res
	})

	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, model := range modelsToStore {
			err := tx.Save(&model.Output).Error
			if err != nil {
				return fmt.Errorf("failed to save output: %w", err)
			}

			association := tx.
				Model(&model.Output).
				Association("Tags")

			err = association.Replace(model.Tags...)
			if err != nil {
				return fmt.Errorf("failed to save current tags for output: %w", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("db transaction failed: %w", err)
	}

	return nil
}

func (o *Outputs) RecreateSpentOutputs(ctx context.Context, spendingTransactionID uint) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-RecreateSpentOutputs", attribute.String("SpendingTxID", fmt.Sprintf("%d", spendingTransactionID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = o.query.DBTransaction(func(query *genquery.Query) error {
		filterScope := func(dao gen.Dao) gen.Dao {
			return dao.
				Where(query.Output.SpentBy.Eq(spendingTransactionID)).
				Scopes(isChangeDaoScope(query))
		}

		changeOutputs, err := getOutputsWithTxStatus(ctx, query, filterScope)
		if err != nil {
			return err
		}

		err = makeOutputsSpendable(ctx, query, filterScope)
		if err != nil {
			return err
		}

		err = createUTXOsFromOutputs(ctx, query, changeOutputs)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to restore spent outputs: %w", err)
	}

	return nil
}

func isChangeDaoScope(query *genquery.Query) func(dao gen.Dao) gen.Dao {
	outTable := &query.Output
	return func(dao gen.Dao) gen.Dao {
		return dao.
			Where(outTable.BasketName.IsNotNull()).
			Where(outTable.Change.Is(true)).
			Where(outTable.Satoshis.Gt(0))
	}
}

type outputWithTxStatus struct {
	models.Output
	TxStatus wdk.TxStatus `gorm:"column:tx_status"`
}

func getOutputsWithTxStatus(ctx context.Context, query *genquery.Query, filterScope func(dao gen.Dao) gen.Dao) ([]*outputWithTxStatus, error) {
	outTable := &query.Output
	txTable := &query.Transaction

	var changeOutputs []*outputWithTxStatus
	err := outTable.WithContext(ctx).
		Select(
			outTable.ID,
			outTable.BasketName,
			outTable.Satoshis,
			outTable.Type,
			outTable.UserID,
			txTable.Status.As("tx_status"),
		).
		Join(txTable, txTable.ID.EqCol(outTable.TransactionID)).
		Scopes(filterScope).
		Scan(&changeOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction outputs: %w", err)
	}

	return changeOutputs, nil
}

func createUTXOsFromOutputs(ctx context.Context, query *genquery.Query, changeOutputs []*outputWithTxStatus) error {
	utxoTable := &query.UserUTXO

	if len(changeOutputs) == 0 {
		return nil
	}

	newUTXOs := make([]*models.UserUTXO, 0, len(changeOutputs))
	for _, output := range changeOutputs {
		utxoStatus := output.TxStatus.ToUTXOStatus()
		if utxoStatus == wdk.UTXOStatusUnknown {
			continue
		}

		newUTXOs = append(newUTXOs, &models.UserUTXO{
			UserID:             output.UserID,
			OutputID:           output.ID,
			BasketName:         *output.BasketName,
			Satoshis:           must.ConvertToUInt64(output.Satoshis),
			EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
			UTXOStatus:         utxoStatus,
		})
	}

	err := utxoTable.
		WithContext(ctx).
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(newUTXOs...)
	if err != nil {
		return fmt.Errorf("failed to create new UTXOs: %w", err)
	}

	return nil
}

func makeOutputsSpendable(ctx context.Context, query *genquery.Query, filterScope func(dao gen.Dao) gen.Dao) error {
	outTable := &query.Output

	_, err := outTable.WithContext(ctx).
		Scopes(filterScope).
		UpdateSimple(
			outTable.Spendable.Value(true),
			outTable.SpentBy.Null(),
		)
	if err != nil {
		return fmt.Errorf("failed to update outputs to spendable: %w", err)
	}

	return nil
}

func (o *Outputs) mapModelToOutputEntity(model *models.Output) *pkgentity.Output {
	output := &pkgentity.Output{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		ID:                 model.ID,
		UserID:             model.UserID,
		TransactionID:      model.TransactionID,
		SpentBy:            model.SpentBy,
		BasketName:         model.BasketName,
		Spendable:          model.Spendable,
		Change:             model.Change,
		Description:        model.Description,
		Vout:               model.Vout,
		Satoshis:           model.Satoshis,
		ProvidedBy:         model.ProvidedBy,
		Purpose:            model.Purpose,
		Type:               model.Type,
		DerivationPrefix:   model.DerivationPrefix,
		DerivationSuffix:   model.DerivationSuffix,
		CustomInstructions: model.CustomInstructions,
		LockingScript:      model.LockingScript,
		SenderIdentityKey:  model.SenderIdentityKey,
		Tags:               slices.Map(model.Tags, func(tag *models.Tag) string { return tag.Name }),
	}
	if model.Transaction != nil && model.Transaction.TxID != nil {
		output.TxID = model.Transaction.TxID
		output.TxStatus = model.Transaction.Status
	}
	if model.UserUTXO != nil {
		output.UserUTXO = mapModelToEntityUserUTXO(model.UserUTXO)
	}
	return output
}

func (o *Outputs) tagFilterScope(tx *gorm.DB, filter entity.ListOutputsFilter) func(db *gorm.DB) *gorm.DB {
	return func(query *gorm.DB) *gorm.DB {
		subQuery := tx.Model(&models.OutputTag{}).
			Select("output_id").
			Where("tag_name IN ?", filter.Tags).
			Where("tag_user_id = ?", filter.UserID)

		if filter.TagsQueryMode == defs.QueryModeAll {
			subQuery = subQuery.Group("output_id").Having("COUNT(DISTINCT tag_name) = ?", len(filter.Tags))
		}

		return query.Where("id IN (?)", subQuery)
	}
}

func (o *Outputs) ShouldTxOutputsBeUnspent(ctx context.Context, transactionID uint) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-ShouldTxOutputsBeUnspent", attribute.String("TransactionID", fmt.Sprintf("%d", transactionID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var result int64
	err = o.db.WithContext(ctx).Model(&models.Output{}).
		Select("1").
		Where(o.query.Output.TransactionID.Eq(transactionID)).
		Where(o.query.Output.SpentBy.IsNotNull()).
		Take(&result).Error

	if err == nil {
		return fmt.Errorf("transaction with ID %d has spent outputs", transactionID)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
		return nil
	}

	return fmt.Errorf("failed to check for spent outputs: %w", err)
}

// AddOutput inserts a new output.
func (o *Outputs) AddOutput(ctx context.Context, out *pkgentity.Output) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-AddOutput")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if out == nil {
		err = fmt.Errorf("output cannot be nil")
		return err
	}

	model := mapEntityToModelOutput(out)
	if err := o.db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to insert output: %w", err)
	}
	return nil
}

// UpdateOutput updates an existing output by spec.
func (o *Outputs) UpdateOutput(ctx context.Context, spec *pkgentity.OutputUpdateSpecification) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-UpdateOutput")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if spec == nil {
		err = fmt.Errorf("update specification cannot be nil")
		return err
	}

	table := &o.query.Output
	updates := map[string]any{}

	if spec.Spendable != nil {
		updates[table.Spendable.ColumnName().String()] = spec.Spendable
	}
	if spec.Description != nil {
		updates[table.Description.ColumnName().String()] = spec.Description
	}
	if spec.LockingScript != nil {
		updates[table.LockingScript.ColumnName().String()] = spec.LockingScript
	}
	if spec.CustomInstr != nil {
		updates[table.CustomInstructions.ColumnName().String()] = spec.CustomInstr
	}

	if len(updates) == 0 {
		return nil
	}

	res, err := table.WithContext(ctx).Where(table.ID.Eq(spec.ID)).Updates(updates)
	if err != nil {
		return fmt.Errorf("failed to update output: %w", err)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("no rows updated for ID=%d", spec.ID)
	}

	return nil
}

// CountOutputs counts outputs matching spec + options.
func (o *Outputs) CountOutputs(ctx context.Context, spec *pkgentity.OutputReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-CountOutputs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &o.query.Output
	tx := &o.query.Transaction

	dao := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(o.conditionsBySpec(ctx, spec)...)

	if needsTransactionJoin(spec) {
		dao = dao.
			Join(tx, tx.ID.EqCol(table.TransactionID))
	}

	count, err := dao.Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count outputs: %w", err)
	}

	return count, nil
}

// conditionsBySpec builds query conditions based on the read spec.
func (o *Outputs) conditionsBySpec(ctx context.Context, spec *pkgentity.OutputReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}

	table := &o.query.Output
	if spec.ID != nil {
		return []gen.Condition{table.ID.Eq(*spec.ID)}
	}

	var conditions []gen.Condition
	if spec.UserID != nil {
		conditions = append(conditions, cmpCondition(table.UserID, spec.UserID))
	}
	if spec.TransactionID != nil {
		conditions = append(conditions, cmpCondition(table.TransactionID, spec.TransactionID))
	}
	if spec.SpentBy != nil {
		conditions = append(conditions, cmpCondition(table.SpentBy, spec.SpentBy))
	}
	if spec.BasketName != nil {
		conditions = append(conditions, cmpCondition(table.BasketName, spec.BasketName))
	}
	if spec.Spendable != nil {
		conditions = append(conditions, cmpBoolCondition(table.Spendable, spec.Spendable))
	}
	if spec.Change != nil {
		conditions = append(conditions, cmpBoolCondition(table.Change, spec.Change))
	}
	if spec.TxStatus != nil {
		conditions = append(conditions, cmpCondition(o.query.Transaction.Status, spec.TxStatus.ToStringComparable()))
	}
	if spec.Satoshis != nil {
		conditions = append(conditions, cmpCondition(table.Satoshis, spec.Satoshis))
	}
	if spec.TxID != nil {
		conditions = append(conditions, cmpCondition(o.query.Transaction.TxID, spec.TxID))
	}
	if spec.Vout != nil {
		conditions = append(conditions, cmpCondition(table.Vout, spec.Vout))
	}
	if spec.Tags != nil {
		conditions = append(conditions, o.tagConditions(ctx, spec.Tags)...)
	}

	return conditions
}

func (o *Outputs) tagConditions(ctx context.Context, tags *pkgentity.ComparableSet[string]) []gen.Condition {
	var conds []gen.Condition
	table := &o.query.Output
	ot := &o.query.OutputTag

	if tags.Empty {
		sub := ot.WithContext(ctx).
			Select(ot.OutputID).
			Where(ot.OutputID.EqCol(table.ID))

		return []gen.Condition{field.Not(field.CompareSubQuery(field.ExistsOp, nil, sub.UnderlyingDB()))}
	}

	if len(tags.ContainAny) > 0 {
		sub := ot.WithContext(ctx).
			Select(ot.OutputID).
			Where(
				ot.TagName.In(tags.ContainAny...),
				ot.OutputID.EqCol(table.ID),
			)
		conds = append(conds, gen.Exists(sub))
	}

	if len(tags.ContainAll) > 0 {
		for _, tag := range tags.ContainAll {
			sub := ot.WithContext(ctx).
				Select(ot.OutputID).
				Where(
					ot.TagName.Eq(tag),
					ot.OutputID.EqCol(table.ID),
				)
			conds = append(conds, gen.Exists(sub))
		}
	}

	return conds
}

func mapEntityToModelOutput(e *pkgentity.Output) *models.Output {
	m := &models.Output{
		Model: gorm.Model{
			ID:        e.ID,
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
		},
		UserID:             e.UserID,
		TransactionID:      e.TransactionID,
		SpentBy:            e.SpentBy,
		Vout:               e.Vout,
		Satoshis:           e.Satoshis,
		LockingScript:      e.LockingScript,
		CustomInstructions: e.CustomInstructions,
		DerivationPrefix:   e.DerivationPrefix,
		DerivationSuffix:   e.DerivationSuffix,
		BasketName:         e.BasketName,
		Spendable:          e.Spendable,
		Change:             e.Change,
		Description:        e.Description,
		ProvidedBy:         e.ProvidedBy,
		Purpose:            e.Purpose,
		Type:               e.Type,
		SenderIdentityKey:  e.SenderIdentityKey,
	}

	for _, tag := range e.Tags {
		m.Tags = append(m.Tags, &models.Tag{
			Name:   tag,
			UserID: e.UserID,
		})
	}

	m.UserUTXO = mapEntityToModelUserUTXO(e.UserUTXO)

	return m
}

func mapEntityToModelUserUTXO(e *pkgentity.UserUTXO) *models.UserUTXO {
	if e == nil {
		return nil
	}
	return &models.UserUTXO{
		UserID:             e.UserID,
		OutputID:           e.OutputID,
		BasketName:         e.BasketName,
		Satoshis:           e.Satoshis,
		EstimatedInputSize: e.EstimatedInputSize,
		CreatedAt:          e.CreatedAt,
		ReservedByID:       e.ReservedByID,
		UTXOStatus:         e.Status,
	}
}

func mapModelToEntityUserUTXO(m *models.UserUTXO) *pkgentity.UserUTXO {
	if m == nil {
		return nil
	}
	return &pkgentity.UserUTXO{
		UserID:             m.UserID,
		OutputID:           m.OutputID,
		BasketName:         m.BasketName,
		Satoshis:           m.Satoshis,
		EstimatedInputSize: m.EstimatedInputSize,
		CreatedAt:          m.CreatedAt,
		ReservedByID:       m.ReservedByID,
		Status:             m.UTXOStatus,
	}
}
