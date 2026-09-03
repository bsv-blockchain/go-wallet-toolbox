package repo

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func (p *KnownTx) GetBEEFForTxID(ctx context.Context, txID string, opts ...entity.GetBEEFOption) (*transaction.Beef, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-GetBEEFForTxID", attribute.String("TxID", txID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	options := to.OptionsWithDefault(entity.GetBEEFOptions{}, opts...)
	beef := transaction.NewBeefV2()
	if options.MergeToBEEF != nil {
		beef = options.MergeToBEEF
	}

	err = p.recursiveBuildValidBEEF(ctx, 0, beef, txID, options, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build valid BEEF: %w", err)
	}

	return beef, nil
}

func (p *KnownTx) GetBEEFForTxIDs(ctx context.Context, txIDs iter.Seq[string], opts ...entity.GetBEEFOption) (*transaction.Beef, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-KnownTx-GetBEEFForTxIDs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	options := to.OptionsWithDefault(entity.GetBEEFOptions{}, opts...)
	beef := transaction.NewBeefV2()
	if options.MergeToBEEF != nil {
		beef = options.MergeToBEEF
	}

	var missingTxIDs []string
	for txID := range txIDs {
		if beef.FindTransaction(txID) != nil {
			continue
		}
		missingTxIDs = append(missingTxIDs, txID)
	}

	var preFetched map[string]models.KnownTx
	if len(missingTxIDs) > 0 {
		preFetched = make(map[string]models.KnownTx)
		if err := p.preFetchAncestry(ctx, preFetched, missingTxIDs, options); err != nil {
			return nil, err
		}
	}

	for _, txID := range missingTxIDs {
		if err := p.recursiveBuildValidBEEF(ctx, 0, beef, txID, options, preFetched); err != nil {
			return nil, fmt.Errorf("failed for txid %s: %w", txID, err)
		}
	}

	return beef, nil
}

// preFetchInto reads the given known txs in one query and adds them to dst,
// leaving entries already present untouched.
func (p *KnownTx) preFetchInto(ctx context.Context, dst map[string]models.KnownTx, txIDs []string, options entity.GetBEEFOptions) ([]models.KnownTx, error) {
	wanted := make([]string, 0, len(txIDs))
	for _, txID := range txIDs {
		if _, ok := dst[txID]; !ok {
			wanted = append(wanted, txID)
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}

	query := p.db.WithContext(ctx).
		Model(&models.KnownTx{}).
		Select("tx_id, raw_tx, input_beef, merkle_path")

	if len(options.StatusesToFilterOut) > 0 {
		query = query.Where("status NOT IN ? ", options.StatusesToFilterOut)
	}

	var modelsBatch []models.KnownTx
	if err := query.Where("tx_id IN ?", wanted).Find(&modelsBatch).Error; err != nil {
		return nil, fmt.Errorf("failed to pre-fetch known txs: %w", err)
	}

	for _, m := range modelsBatch {
		dst[m.TxID] = m
	}
	return modelsBatch, nil
}

// preFetchAncestry reads the ancestry breadth-first, one query per generation.
//
// The recursive build reads any ancestor it has not already been handed with an
// individual First(). Pre-reading two generations covers a build whose subjects
// are already proved, but not one that has to walk back through unmined
// history - and on a chain that is not confirming promptly, that is every
// build. The walk then costs one round trip per ancestor, which is dominated by
// per-statement overhead rather than by the lookup itself.
//
// The rows read here are exactly the ones the recursion would have read anyway;
// only the number of statements changes. Note that index scan counts do not
// fall as a result - each value in an IN list still costs a probe - which is
// the point: the probe was never the expensive part.
//
// Bounded by maxDepthOfRecursion, which the recursion enforces too, and it
// stops as soon as a generation adds nothing.
func (p *KnownTx) preFetchAncestry(ctx context.Context, dst map[string]models.KnownTx, seedIDs []string, options entity.GetBEEFOptions) error {
	frontier := seedIDs

	for depth := 0; depth < maxDepthOfRecursion && len(frontier) > 0; depth++ {
		added, err := p.preFetchInto(ctx, dst, frontier, options)
		if err != nil {
			return err
		}
		if len(added) == 0 {
			return nil
		}

		// DirectSourcesOnly makes the parents terminal, so nothing below them is
		// ever read and pre-reading it would be waste.
		if options.DirectSourcesOnly && depth >= 1 {
			return nil
		}

		frontier = parentIDsOf(added, dst, options)
	}

	return nil
}

// needsInputBEEF reports whether the stored input beef has to be merged to
// build this transaction.
//
// That blob is the whole ancestry the transaction was submitted with, and for
// busy wallets it reaches hundreds of kilobytes. A DirectSourcesOnly build only
// wants the immediate parents, so when every one of them is a known tx in its
// own right they can be read individually instead — far cheaper than parsing
// the blob to reach the same few rows.
//
// It is not optional in general: inputs supplied by the caller are recorded
// only inside this blob, never as known txs, so dropping it would lose them.
func needsInputBEEF(tx *transaction.Transaction, options entity.GetBEEFOptions, preFetched map[string]models.KnownTx) bool {
	if !options.DirectSourcesOnly {
		return true
	}
	for _, input := range tx.Inputs {
		if input.SourceTXID == nil {
			return true
		}
		if _, known := preFetched[input.SourceTXID.String()]; !known {
			return true
		}
	}
	return false
}

// parentIDsOf returns the txids spent by one generation of fetched transactions,
// skipping any already present in fetched.
// A raw tx that cannot be parsed is skipped: the build reports that properly.
func parentIDsOf(generation []models.KnownTx, fetched map[string]models.KnownTx, options entity.GetBEEFOptions) []string {
	// A proved transaction ends the walk, so its parents are never read - unless
	// MinProofLevel deliberately withholds the proof and forces the walk on.
	provenIsTerminal := options.MinProofLevel == 0

	seen := make(map[string]struct{}, len(generation))
	var parents []string
	for _, model := range generation {
		if model.RawTx == nil {
			continue
		}
		if provenIsTerminal && model.HasMerklePath() {
			continue
		}
		tx, err := transaction.NewTransactionFromBytes(model.RawTx)
		if err != nil {
			continue
		}
		for _, input := range tx.Inputs {
			if input.SourceTXID == nil {
				continue
			}
			sourceID := input.SourceTXID.String()
			if _, ok := fetched[sourceID]; ok {
				continue
			}
			if _, dup := seen[sourceID]; dup {
				continue
			}
			seen[sourceID] = struct{}{}
			parents = append(parents, sourceID)
		}
	}
	return parents
}

func (p *KnownTx) recursiveBuildValidBEEF(
	ctx context.Context,
	depth int,
	mergeToBeef *transaction.Beef,
	txID string,
	options entity.GetBEEFOptions,
	preFetched map[string]models.KnownTx,
) error {
	if depth > maxDepthOfRecursion {
		return fmt.Errorf("max depth of recursion reached: %d", maxDepthOfRecursion)
	}

	if options.IsKnownTxID(txID) {
		h, err := chainhash.NewHashFromHex(txID)
		if err != nil {
			return fmt.Errorf("failed to parse string txID Hex to chainhash %s: %w", txID, err)
		}
		mergeToBeef.MergeTxidOnly(h)
		return nil
	}

	var model models.KnownTx
	var err error

	if cachedModel, ok := preFetched[txID]; ok {
		model = cachedModel
	} else {
		query := p.db.WithContext(ctx).
			Model(&model).
			Select("raw_tx, input_beef, merkle_path")

		if len(options.StatusesToFilterOut) > 0 {
			query = query.Where("status NOT IN ? ", options.StatusesToFilterOut)
		}

		err = query.First(&model, "tx_id = ? ", txID).Error
	}

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		if options.TxGetterFcn == nil {
			return fmt.Errorf("transaction txID: %q is not known to storage: %w", txID, wdk.ErrNotFoundError)
		}

		var rawTx []byte
		var merklePath *transaction.MerklePath
		rawTx, merklePath, err = options.TxGetterFcn(ctx, txID)
		if err != nil {
			return fmt.Errorf("failed to get raw tx and merkle path for tx (TxID: %q) using services: %w", txID, err)
		}

		inputBeef, _ := transaction.NewBeefV2().Bytes()

		model = models.KnownTx{
			TxID:       txID,
			RawTx:      rawTx,
			MerklePath: to.If(merklePath != nil, merklePath.Bytes).ElseThen(nil),
			InputBeef:  inputBeef,
		}
	} else if err != nil {
		return fmt.Errorf("failed to find known tx, raw tx and input beef for tx (id: %s): %w", txID, err)
	} else if options.TrustsSelfAsKnown() {
		var txIDHash *chainhash.Hash
		txIDHash, err = chainhash.NewHashFromHex(txID)
		if err != nil {
			return fmt.Errorf("failed to parse txid %s: %w", txID, err)
		}
		mergeToBeef.MergeTxidOnly(txIDHash)
		return nil
	}

	if model.RawTx == nil {
		return fmt.Errorf("raw tx is nil in transaction %s", txID)
	}

	tx, err := transaction.NewTransactionFromBytes(model.RawTx)
	if err != nil {
		return fmt.Errorf("failed to build transaction object from raw tx (id: %s): %w", txID, err)
	}

	// DirectSourcesOnly: parents are terminal — merge the raw tx alone. No
	// merkle proof (skips BUMP root validation, the hot spot at high TPS), no
	// input-beef merge, no deeper recursion. Script verification and EF
	// construction only need the parent's outputs.
	if options.DirectSourcesOnly && depth >= 1 {
		if model.RawTx == nil {
			return fmt.Errorf("raw tx is nil in transaction %s", txID)
		}
		if _, mergeErr := mergeToBeef.MergeRawTx(model.RawTx, nil); mergeErr != nil {
			return fmt.Errorf("failed to merge raw source tx (id: %s) into BEEF object: %w", txID, mergeErr)
		}
		return nil
	}

	ignoreMerkleProof := options.MinProofLevel > 0 && depth < options.MinProofLevel // If enabled, we intentionally skip attaching the merkle proof at this depth
	if model.HasMerklePath() && !ignoreMerkleProof {
		var merklePath *transaction.MerklePath
		merklePath, err = transaction.NewMerklePathFromBinary(model.MerklePath)
		if err != nil {
			return fmt.Errorf("failed to build merkle path from binary for tx (id: %s): %w", txID, err)
		}
		err = tx.AddMerkleProof(merklePath)
		if err != nil {
			return fmt.Errorf("failed to add merkle proof to transaction (id: %s): %w", txID, err)
		}

		_, err = mergeToBeef.MergeTransaction(tx)
		if err != nil {
			return fmt.Errorf("failed to merge transaction (id: %s) into BEEF object: %w", txID, err)
		}

		return nil
	}

	for i := range tx.Inputs {
		if len(tx.Inputs[i].SourceTXID) == 0 {
			return fmt.Errorf("input of tx (id: %s) has empty SourceTXID at index %d ", txID, i)
		}
	}

	_, err = mergeToBeef.MergeRawTx(model.RawTx, nil)
	if err != nil {
		return fmt.Errorf("failed to merge raw tx (id: %s) into BEEF object: %w", txID, err)
	}

	if len(model.InputBeef) > 0 && needsInputBEEF(tx, options, preFetched) {
		err = mergeToBeef.MergeBeefBytes(model.InputBeef)
		if err != nil {
			return fmt.Errorf("failed to merge input beef into BEEF object: %w", err)
		}
	}

	subjectTx := mergeToBeef.FindTransaction(txID)
	if subjectTx == nil {
		return fmt.Errorf("transaction %q has not been merged into BEEF object, even though its raw tx was merged", txID)
	}

	if subjectTx.MerklePath != nil {
		// The Transaction already has a merkle path, no need to recursively build it
		return nil
	}

	for _, input := range tx.Inputs {
		beefTx := mergeToBeef.Transactions[*input.SourceTXID]
		if beefTx == nil || beefTx.DataFormat == transaction.TxIDOnly {
			err = p.recursiveBuildValidBEEF(ctx, depth+1, mergeToBeef, input.SourceTXID.String(), options, preFetched)
			if err != nil {
				return fmt.Errorf("failed to recursively find known tx and merge into BEEF: %w", err)
			}
		}
	}

	// Result is in mergeToBeef
	return nil
}
