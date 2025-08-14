package actions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdslices "slices"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type getBeef struct {
	logger      *slog.Logger
	knownTxRepo KnownTxRepo
	services    wdk.Services
}

func newGetBeef(logger *slog.Logger, knownTxRepo KnownTxRepo, services wdk.Services) *getBeef {
	return &getBeef{
		logger:      logger,
		knownTxRepo: knownTxRepo,
		services:    services,
	}
}

type rawTxWithMerklePath struct {
	rawTx      []byte
	merklePath *transaction.MerklePath
	header     *wdk.MerklePathBlockHeader
}

func (g *getBeef) GetBeef(ctx context.Context, txID string, options wdk.StorageGetBeefOptions) (*transaction.Beef, error) {
	if stdslices.Contains(options.KnownTxIDs, txID) {
		return g.beefForKnownID(txID)
	}

	serviceFetchedTransactions := make(map[string]rawTxWithMerklePath)
	getBeefOptions := g.prepareOptions(options, serviceFetchedTransactions)

	if !options.IgnoreStorage {
		return g.getFromStorage(ctx, txID, options, getBeefOptions, serviceFetchedTransactions)
	}

	if !options.IgnoreServices {
		return g.getFromServices(ctx, txID, options)
	}

	return nil, fmt.Errorf("no storage or services provided to get BEEF for transaction %s", txID)
}

func (g *getBeef) beefForKnownID(txID string) (*transaction.Beef, error) {
	beef := transaction.NewBeefV2()
	txIDHash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		return nil, fmt.Errorf("failed to create hash from txID %s: %w", txID, err)
	}
	beef.MergeTxidOnly(txIDHash)
	return beef, nil
}

func (g *getBeef) prepareOptions(options wdk.StorageGetBeefOptions, serviceFetchedTransactions map[string]rawTxWithMerklePath) []entity.GetBEEFOption {
	var getBeefOptions []entity.GetBEEFOption

	if !options.IgnoreServices {
		txGetter := g.makeTxGetter(serviceFetchedTransactions)
		getBeefOptions = append(getBeefOptions, entity.WithTxGetterFcn(txGetter))
	}

	if len(options.KnownTxIDs) > 0 {
		getBeefOptions = append(getBeefOptions, entity.WithKnownTxIDs(options.KnownTxIDs...))
		knownSet := make(map[string]struct{}, len(options.KnownTxIDs))
		for _, id := range options.KnownTxIDs {
			knownSet[id] = struct{}{}
		}
		getBeefOptions = append(getBeefOptions, entity.WithKnownTxIDsLookup(knownSet))
	}

	if options.TrustSelf != "" {
		getBeefOptions = append(getBeefOptions, entity.WithTrustSelf(options.TrustSelf))
	}
	if options.MinProofLevel > 0 {
		getBeefOptions = append(getBeefOptions, entity.WithMinProofLevel(options.MinProofLevel))
	}

	return getBeefOptions
}

func (g *getBeef) makeTxGetter(serviceFetchedTransactions map[string]rawTxWithMerklePath) entity.TxGetterFcn {
	return func(ctx context.Context, txID string) (rawTx []byte, merklePath *transaction.MerklePath, err error) {
		rawTxResult, err := g.services.RawTx(txID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get raw transaction for txID %s: %w", txID, err)
		}

		if rawTxResult.RawTx == nil {
			return nil, nil, fmt.Errorf("raw transaction for txID %s is nil", txID)
		}

		merklePathResult, err := g.services.MerklePath(ctx, txID)
		if err != nil && !errors.Is(err, wdk.NotFoundError) {
			return nil, nil, fmt.Errorf("failed to get merkle path for txID %s: %w", txID, err)
		}

		serviceFetchedTransactions[txID] = rawTxWithMerklePath{
			rawTx:      rawTxResult.RawTx,
			merklePath: merklePathResult.MerklePath,
			header:     merklePathResult.BlockHeader,
		}

		return rawTxResult.RawTx, merklePathResult.MerklePath, nil
	}
}

func (g *getBeef) getFromStorage(
	ctx context.Context,
	txID string,
	options wdk.StorageGetBeefOptions,
	getBeefOptions []entity.GetBEEFOption,
	serviceFetchedTransactions map[string]rawTxWithMerklePath,
) (*transaction.Beef, error) {
	beef, err := g.knownTxRepo.GetBEEFForTxID(ctx, txID, getBeefOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to get beef for transaction %s: %w", txID, err)
	}

	if !options.IgnoreNewProven {
		for id, fetched := range serviceFetchedTransactions {
			if err := g.persistNewProven(ctx, id, fetched); err != nil {
				return nil, err
			}
		}
	}

	return beef, nil
}

func (g *getBeef) persistNewProven(ctx context.Context, id string, fetched rawTxWithMerklePath) error {
	if fetched.merklePath == nil || fetched.header == nil || len(fetched.rawTx) == 0 {
		return nil
	}

	emptyBeef, err := transaction.NewBeefV2().Bytes()
	if err != nil {
		return fmt.Errorf("failed to serialize empty beef for transaction %s: %w", id, err)
	}
	if err := g.knownTxRepo.UpsertKnownTx(ctx, &entity.UpsertKnownTx{
		TxID:      id,
		RawTx:     fetched.rawTx,
		InputBeef: emptyBeef,
		Status:    wdk.ProvenTxStatusCompleted,
	}, history.NewBuilder().GetMerklePathSuccess("services")); err != nil {
		g.logger.Error("failed to upsert known transaction", "txID", id, "error", err)
		return fmt.Errorf("failed to upsert known transaction %s: %w", id, err)
	}

	merklePathBytes := fetched.merklePath.Bytes()
	if err := g.knownTxRepo.UpdateKnownTxAsMined(ctx, &entity.KnownTxAsMined{
		TxID:        id,
		BlockHeight: fetched.header.Height,
		MerklePath:  merklePathBytes,
		MerkleRoot:  fetched.header.MerkleRoot,
		BlockHash:   fetched.header.Hash,
		Notes:       []history.Builder{history.NewBuilder().GetMerklePathSuccess("services")},
	}); err != nil {
		g.logger.Error("failed to update known tx as mined", slog.String("txID", id), slog.Any("error", err))
	}

	return nil
}

func (g *getBeef) getFromServices(ctx context.Context, txID string, options wdk.StorageGetBeefOptions) (*transaction.Beef, error) {
	beef, err := g.services.GetBEEF(ctx, txID, options.KnownTxIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get beef for transaction %s using services: %w", txID, err)
	}
	return beef, nil
}
