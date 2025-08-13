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
		beef := transaction.NewBeefV2()
		txIDHash, err := chainhash.NewHashFromHex(txID)
		if err != nil {
			return nil, fmt.Errorf("failed to create hash from txID %s: %w", txID, err)
		}
		beef.MergeTxidOnly(txIDHash)
		return beef, nil
	}

	var getBeefOptions []entity.GetBEEFOption
	serviceFetchedTransactions := make(map[string]rawTxWithMerklePath)
	if !options.IgnoreServices {
		txGetter := func(ctx context.Context, txID string) (rawTx []byte, merklePath *transaction.MerklePath, err error) {
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
		getBeefOptions = append(getBeefOptions, entity.WithTxGetterFcn(txGetter))
	}

	if len(options.KnownTxIDs) > 0 {
		getBeefOptions = append(getBeefOptions, entity.WithKnownTxIDs(options.KnownTxIDs...))
	}
	if options.TrustSelf != "" {
		getBeefOptions = append(getBeefOptions, entity.WithTrustSelf(options.TrustSelf))
	}
	if options.MinProofLevel > 0 {
		getBeefOptions = append(getBeefOptions, entity.WithMinProofLevel(options.MinProofLevel))
	}

	if !options.IgnoreStorage {
		beef, err := g.knownTxRepo.GetBEEFForTxID(ctx, txID, getBeefOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to get beef for transaction %s: %w", txID, err)
		}

		if !options.IgnoreNewProven {
			for id, fetched := range serviceFetchedTransactions {
				if fetched.merklePath == nil || fetched.header == nil || len(fetched.rawTx) == 0 {
					continue
				}
				emptyBeef, _ := transaction.NewBeefV2().Bytes()
				_ = g.knownTxRepo.UpsertKnownTx(ctx, &entity.UpsertKnownTx{
					TxID:      id,
					RawTx:     fetched.rawTx,
					InputBeef: emptyBeef,
					Status:    wdk.ProvenTxStatusCompleted,
				}, history.NewBuilder().GetMerklePathSuccess("services"))

				if fetched.merklePath != nil {
					merklePathBytes := fetched.merklePath.Bytes()
					_ = g.knownTxRepo.UpdateKnownTxAsMined(ctx, &entity.KnownTxAsMined{
						TxID:        id,
						BlockHeight: fetched.header.Height,
						MerklePath:  merklePathBytes,
						MerkleRoot:  fetched.header.MerkleRoot,
						BlockHash:   fetched.header.Hash,
						Notes:       []history.Builder{history.NewBuilder().GetMerklePathSuccess("services")},
					})
				}
			}
		}

		return beef, nil
	}

	if !options.IgnoreServices {
		beef, err := g.services.GetBEEF(ctx, txID, options.KnownTxIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get beef for transaction %s using services: %w", txID, err)
		}

		return beef, nil
	}

	return nil, fmt.Errorf("no storage or services provided to get BEEF for transaction %s", txID)

}
