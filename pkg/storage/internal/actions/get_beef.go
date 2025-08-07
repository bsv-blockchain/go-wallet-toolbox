package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"log/slog"
	stdslices "slices"
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
}

func (g *getBeef) GetBeef(ctx context.Context, txID string, options wdk.StorageGetBeefOptions) (*transaction.Beef, error) {
	if stdslices.Contains(options.KnownTxids, txID) {
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
			if err != nil && !errors.Is(err, wdk.NotFoundError)  {
				return nil, nil, fmt.Errorf("failed to get merkle path for txID %s: %w", txID, err)
			}

			serviceFetchedTransactions[txID] = rawTxWithMerklePath{
				rawTx:      rawTxResult.RawTx,
				merklePath: merklePathResult.MerklePath,
			}

			return rawTxResult.RawTx, merklePathResult.MerklePath, nil
		}
		getBeefOptions = append(getBeefOptions, entity.WithTxGetterFcn(txGetter))
	}

	if !options.IgnoreStorage {
		beef, err := g.knownTxRepo.GetBEEFForTxID(ctx, txID, getBeefOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to get beef for transaction %s: %w", txID, err)
		}

		if !options.IgnoreNewProven {
			// TODO: Store the transactions that have been fetched from the services
			_ = serviceFetchedTransactions // NOTE: Only the
		}

		return beef, nil
	} else if !options.IgnoreServices {
		// TODO: Add support for getting the BEEF all from the services
		return nil, fmt.Errorf("ignoring storage is not supported for GetBEEF")
	}

	return nil, fmt.Errorf("no storage or services provided to get BEEF for transaction %s", txID)

}
