package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Outputs struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewOutputs(db *gorm.DB, query *genquery.Query) *Outputs {
	return &Outputs{db: db, query: query}
}

type txIDsReadModel struct {
	TransactionID string `gorm:"column:txid"`
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
		Join(txTable, txTable.TransactionID.EqCol(outTable.TransactionID)).
		Where(outTable.OutputID.In(idsClause...)).
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
			return db.Select("transactionId, txid")
		}).
		Where("outputId IN ?", idsClause).
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
			Join(tx, tx.TransactionID.EqCol(output.TransactionID))
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
		Where("transactionId = ?", transactionID).
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
			Where("userId = ?", filter.UserID).
			Preload("Transaction", func(db *gorm.DB) *gorm.DB {
				return db.Select("outputId, txid")
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
		query = query.Where(
			"transactionId IN (?)",
		).
		Where(func(db *gorm.DB) *gorm.DB {
			return db.
				Where("userId = ?", filter.UserID).
				Where("status IN ?", allowedStatuses)
		})

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
			Select("outputId").
			Scopes(scopes.UserID(userID)).
			Where("vout = ?", outpoint.Vout).
			Where(
				"transactionId IN (?)",
				tx.Model(&models.Transaction{}).
					Select("outputId").
					Scopes(scopes.UserID(userID)).
					Where("txid = ?", outpoint.TxID),
			)

		if basketName != nil {
			query = query.Where("basket_name = ?", *basketName)
		}

		var output models.Output
		if err = query.First(&output).Error; err != nil {
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
			Where("outputId = ?", output.OutputID).
			Update("basketId", nil)

		if result.Error != nil {
			return fmt.Errorf("failed to unlink output from basket: %w", result.Error)
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
			Select(fmt.Sprintf("%s.*, tx.txid as txid", outputTableName)).
			Joins(fmt.Sprintf("INNER JOIN %s tx ON tx.transactionId = %s.transactionId", transactionTableName, outputTableName)).
			Where(fmt.Sprintf("%s.userId = ?", outputTableName), userID),
	).Where("(txid,vout) IN (?)", outpointStrings)

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
		Where(
			"transactionId IN (?)",
			o.db.Model(&models.Transaction{}).
				Select("outputId").
				Scopes(scopes.UserID(userID)).
				Where("txid = ?", outpoint.TxID),
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
func (o *Outputs) FindInputsAndOutputsWithBaskets(ctx context.Context, txIDs []uint, includeLockingScripts bool) (inputs, outputs map[uint][]*pkgentity.Output, err error) {
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-FindInputsAndOutputsWithBaskets")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return inputs, outputs, err
	}

	query := o.db.WithContext(ctx).
		Model(&models.Output{}).
		Preload("Transaction", func(db *gorm.DB) *gorm.DB {
			return db.Select("outputId, txid")
		}).
		Preload("Basket").
		Preload("Tags").
		Where("transactionId IN ? OR spentBy IN ?", txIDs, txIDs)

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

		var rows *sql.Rows
		rows, err = dbq.Rows()
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
		Select("transactionId").
		Where("userId = ?", userID)
	if len(filter.Status) > 0 {
		selected = selected.Where("status IN ?", filter.Status)
	}
	if len(filter.Labels) > 0 {
		subQuery := tx.Table("bsv_tx_labels_map tlm").
			Select("tlm.transaction_id").
			Joins("JOIN bsv_tx_labels tl ON tl.txLabelId = tlm.tx_label_id").
			Where("tl.label IN ?", filter.Labels).
			Where("tl.userId = ?", userID).
			Where("tl.isDeleted = ?", false).
			Where("tlm.isDeleted = ?", false)
		if filter.LabelQueryMode == defs.QueryModeAll {
			subQuery = subQuery.Group("tlm.transaction_id").Having("COUNT(DISTINCT tl.label) = ?", len(filter.Labels))
		}
		selected = selected.Where("transactionId IN (?)", subQuery)
	}
	return selected.Order("transactionId ASC").Limit(filter.Limit).Offset(filter.Offset)
}

// buildOutputsJoinQuery constructs the JOIN query to fetch outputs (and tags) for selected actions
func (o *Outputs) buildOutputsJoinQuery(tx, selected *gorm.DB, userID int, includeLockingScripts bool) *gorm.DB {
	outputTable := o.query.Output.TableName()
	txTable := o.query.Transaction.TableName()

	dbq := tx.
		Table(outputTable+" o").
		Joins("JOIN (?) s ON s.transactionId = o.transactionId OR s.transactionId = o.spentBy", selected).
		Joins("LEFT JOIN "+txTable+" t ON t.transactionId = o.transactionId").
		Joins("LEFT JOIN bsv_output_tags_map otm ON otm.output_id = o.outputId AND otm.isDeleted = false").
		Joins("LEFT JOIN bsv_output_tags ot ON ot.outputTagId = otm.output_tag_id AND ot.userId = o.userId AND ot.isDeleted = false").
		Where("o.userId = ?", userID).
		Order("o.outputId ASC").
		Select("o.*, t.txid as txid, ot.tag as tag")

	if !includeLockingScripts {
		dbq = dbq.Omit("o.lockingScript")
	}
	return dbq
}

// readOutputsIntoMaps scans streamed rows and groups them into input/output maps with tag de-duplication
func (o *Outputs) readOutputsIntoMaps(tx *gorm.DB, rows *sql.Rows) (map[uint][]*pkgentity.Output, map[uint][]*pkgentity.Output, error) {
	type readRow struct {
		models.Output

		TxID *string `gorm:"column:txid"`
		Tag  *string `gorm:"column:tag"`
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
				OutputID:           output.ID,
				UserID:             output.UserID,
				TransactionID:      output.TransactionID,
				SpentBy:            output.SpentBy,
				Vout:               output.Vout,
				Satoshis:           output.Satoshis,
				LockingScript:      output.LockingScript,
				CustomInstructions: output.CustomInstructions,
				DerivationPrefix:   output.DerivationPrefix,
				DerivationSuffix:   output.DerivationSuffix,
				BasketID:           output.BasketID,
				Spendable:          output.Spendable,
				Change:             output.Change,
				Description:        output.Description,
				ProvidedBy:         output.ProvidedBy,
				Purpose:            output.Purpose,
				Type:               output.Type,
				SenderIdentityKey:  output.SenderIdentityKey,
			},
			Tags: slices.Map(output.Tags, func(tag string) any {
				return &models.OutputTag{
					Tag:    tag,
					UserID: output.UserID,
				}
			}),
		}

		return res
	})

	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, model := range modelsToStore {
			err = tx.Save(&model.Output).Error
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
		allSpentScope := func(dao gen.Dao) gen.Dao {
			return dao.Where(query.Output.SpentBy.Eq(spendingTransactionID))
		}

		changeSpentScope := func(dao gen.Dao) gen.Dao {
			return dao.
				Where(query.Output.SpentBy.Eq(spendingTransactionID)).
				Scopes(isChangeDaoScope(query))
		}

		_, err = getOutputsWithTxStatus(ctx, query, changeSpentScope)
		err = makeOutputsSpendable(ctx, query, allSpentScope)

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
			Where(outTable.BasketID.IsNotNull()).
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
			outTable.OutputID,
			outTable.BasketID,
			outTable.Satoshis,
			outTable.Type,
			outTable.UserID,
			txTable.Status.As("tx_status"),
		).
		Join(txTable, txTable.TransactionID.EqCol(outTable.TransactionID)).
		Scopes(filterScope).
		Scan(&changeOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction outputs: %w", err)
	}

	return changeOutputs, nil
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
		ID:                 model.OutputID,
		UserID:             model.UserID,
		TransactionID:      model.TransactionID,
		SpentBy:            model.SpentBy,
		BasketID:           model.BasketID,
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
		Tags:               slices.Map(model.Tags, func(tag *models.OutputTag) string { return tag.Tag }),
	}
	if model.Transaction != nil && model.Transaction.TxID != nil {
		output.TxID = model.Transaction.TxID
		output.TxStatus = model.Transaction.Status
	}
	return output
}

func (o *Outputs) tagFilterScope(tx *gorm.DB, filter entity.ListOutputsFilter) func(db *gorm.DB) *gorm.DB {
	return func(query *gorm.DB) *gorm.DB {
		subQuery := tx.Model(&models.OutputTag{}).
			Select("outputId").
			Where("tag IN ?", filter.Tags).
			Where("userId = ?", filter.UserID)

		if filter.TagsQueryMode == defs.QueryModeAll {
			subQuery = subQuery.Group("outputId").Having("COUNT(DISTINCT tag) = ?", len(filter.Tags))
		}

		return query.Where("outputId IN (?)", subQuery)
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

func (o *Outputs) MarkCreatedOutputsAsNotSpendable(ctx context.Context, transactionID uint) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-MarkCreatedOutputsAsNotSpendable", attribute.String("TransactionID", fmt.Sprintf("%d", transactionID)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	outTable := &o.query.Output
	_, err = outTable.WithContext(ctx).
		Where(outTable.TransactionID.Eq(transactionID)).
		UpdateSimple(outTable.Spendable.Value(false))
	if err != nil {
		return fmt.Errorf("failed to mark created outputs as not spendable for transaction %d: %w", transactionID, err)
	}

	return nil
}

func (o *Outputs) MarkCreatedOutputsAsSpendableByTxID(ctx context.Context, txID string) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-Outputs-MarkCreatedOutputsAsSpendableByTxID", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txTable := &o.query.Transaction
	outTable := &o.query.Output
	subquery := txTable.WithContext(ctx).Select(txTable.TransactionID).Where(txTable.TxID.Eq(txID))

	_, err = outTable.WithContext(ctx).
		Where(field.ContainsSubQuery([]field.Expr{outTable.TransactionID}, subquery.UnderlyingDB())).
		UpdateSimple(outTable.Spendable.Value(true))
	if err != nil {
		return fmt.Errorf("failed to mark created outputs as spendable for tx %s: %w", txID, err)
	}

	return nil
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

	res, err := table.WithContext(ctx).Where(table.OutputID.Eq(spec.ID)).Updates(updates)
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
			Join(tx, tx.TransactionID.EqCol(table.TransactionID))
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
		return []gen.Condition{table.OutputID.Eq(*spec.ID)}
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
	if spec.BasketID != nil {
		conditions = append(conditions, cmpCondition(table.BasketID, spec.BasketID))
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
	otm := &o.query.OutputTagsMap

	if tags.Empty {
		sub := otm.WithContext(ctx).
			Select(otm.OutputID).
			Where(otm.OutputID.EqCol(table.OutputID))

		return []gen.Condition{field.Not(field.CompareSubQuery(field.ExistsOp, nil, sub.UnderlyingDB()))}
	}

	if len(tags.ContainAny) > 0 {
		sub := otm.WithContext(ctx).
			Join(ot, ot.OutputTagID.EqCol(otm.OutputTagID)).
			Select(otm.OutputID).
			Where(
				ot.Tag.In(tags.ContainAny...),
				otm.OutputID.EqCol(table.OutputID),
			)
		conds = append(conds, gen.Exists(sub))
	}

	if len(tags.ContainAll) > 0 {
		for _, tag := range tags.ContainAll {
			sub := otm.WithContext(ctx).
				Join(ot, ot.OutputTagID.EqCol(otm.OutputTagID)).
				Select(otm.OutputID).
				Where(
					ot.Tag.Eq(tag),
					otm.OutputID.EqCol(table.OutputID),
				)
			conds = append(conds, gen.Exists(sub))
		}
	}

	return conds
}

func mapEntityToModelOutput(e *pkgentity.Output) *models.Output {
	m := &models.Output{
		OutputID:           e.ID,
		UserID:             e.UserID,
		TransactionID:      e.TransactionID,
		SpentBy:            e.SpentBy,
		Vout:               e.Vout,
		Satoshis:           e.Satoshis,
		LockingScript:      e.LockingScript,
		CustomInstructions: e.CustomInstructions,
		DerivationPrefix:   e.DerivationPrefix,
		DerivationSuffix:   e.DerivationSuffix,
		BasketID:           e.BasketID,
		Spendable:          e.Spendable,
		Change:             e.Change,
		Description:        e.Description,
		ProvidedBy:         e.ProvidedBy,
		Purpose:            e.Purpose,
		Type:               e.Type,
		SenderIdentityKey:  e.SenderIdentityKey,
	}

	for _, tag := range e.Tags {
		m.Tags = append(m.Tags, &models.OutputTag{
			Tag:    tag,
			UserID: e.UserID,
		})
	}

	return m
}
