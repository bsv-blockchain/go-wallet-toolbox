package repo

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"
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

	err = p.recursiveBuildValidBEEF(ctx, 0, beef, txID, options)
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

	for txID := range txIDs {
		if beef.FindTransaction(txID) != nil {
			continue
		}

		if err := p.recursiveBuildValidBEEF(ctx, 0, beef, txID, options); err != nil {
			return nil, fmt.Errorf("failed for txid %s: %w", txID, err)
		}
	}

	return beef, nil
}

func (p *KnownTx) recursiveBuildValidBEEF(
	ctx context.Context,
	depth int,
	mergeToBeef *transaction.Beef,
	txID string,
	options entity.GetBEEFOptions,
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
	query := p.db.WithContext(ctx).
		Model(&model).
		Select("raw_tx, input_beef, merkle_path")

	if len(options.StatusesToFilterOut) > 0 {
		query = query.Where("status NOT IN ? ", options.StatusesToFilterOut)
	}

	err := query.First(&model, "tx_id = ? ", txID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if options.TxGetterFcn == nil {
			return fmt.Errorf("transaction txID: %q is not known to storage: %w", txID, wdk.ErrNotFoundError)
		}

		rawTx, merklePath, err := options.TxGetterFcn(ctx, txID)
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
		txIDHash, err := chainhash.NewHashFromHex(txID)
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

	ignoreMerkleProof := options.MinProofLevel > 0 && depth < options.MinProofLevel // If enabled, we intentionally skip attaching the merkle proof at this depth
	if model.HasMerklePath() && !ignoreMerkleProof {
		merklePath, err := transaction.NewMerklePathFromBinary(model.MerklePath)
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

	if len(model.InputBeef) > 0 {
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
			err = p.recursiveBuildValidBEEF(ctx, depth+1, mergeToBeef, input.SourceTXID.String(), options)
			if err != nil {
				return fmt.Errorf("failed to recursively find known tx and merge into BEEF: %w", err)
			}
		}
	}

	// Result is in mergeToBeef
	return nil
}
