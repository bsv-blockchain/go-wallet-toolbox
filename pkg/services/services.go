package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/options"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// WalletServices is a struct that contains services used by a wallet
type WalletServices struct {
	logger       *slog.Logger
	chain        defs.BSVNetwork
	config       *defs.WalletServices
	whatsonchain *whatsonchain.WhatsOnChain

	rawTxServices         servicequeue.Queue1[string, *wdk.RawTxResult]
	postBEEFServices      servicequeue.Queue2[*transaction.Beef, []string, *wdk.PostedBEEF]
	getMerklePathServices servicequeue.Queue1[string, *wdk.MerklePathResult]
	chainHeaderServices   servicequeue.Queue[*wdk.ChainBlockHeader]
	validatorServices     servicequeue.Queue2[*chainhash.Hash, uint32, bool]
	heightServices        servicequeue.Queue[uint32]
	scriptHistoryServices servicequeue.Queue1[string, *wdk.ScriptHistoryResult]
	hashToHeaderServices  servicequeue.Queue1[string, *wdk.ChainBlockHeader]
	// getRawTxServices: ServiceCollection<sdk.GetRawTxService>
	// postBeefServices: ServiceCollection<sdk.PostBeefService>
	// getUtxoStatusServices: ServiceCollection<sdk.GetUtxoStatusService>
	// updateFiatExchangeRateServices: ServiceCollection<sdk.UpdateFiatExchangeRateService>
}

// New will return a new WalletServices
func New(logger *slog.Logger, config defs.WalletServices, opts ...func(*options.Service)) *WalletServices {
	option := to.OptionsWithDefault(options.Default(), opts...)

	if config.Chain == "" {
		panic("chain is required")
	}

	wocService := whatsonchain.New(option.RestyClientFactory.New(), logger, config.Chain, config.WhatsOnChain)
	arcService := arc.New(logger, option.RestyClientFactory.New(), config.ArcConfig)
	bitailsService := bitails.New(option.RestyClientFactory.New(), logger, config.Chain, config.Bitails)
	bhsService := bhs.New(option.RestyClientFactory.New(), logger, config.Chain, config.BHS)

	return &WalletServices{
		chain:        config.Chain,
		config:       &config,
		logger:       logger,
		whatsonchain: wocService,

		rawTxServices: servicequeue.NewQueue1(
			logger,
			"RawTx",
			servicequeue.NewService1(whatsonchain.ServiceName, wocService.RawTx),
			servicequeue.NewService1(bitails.ServiceName, bitailsService.RawTx),
		),

		postBEEFServices: servicequeue.NewQueue2(
			logger,
			"PostBEEF",
			servicequeue.NewService2(arc.ServiceName, arcService.PostBEEF),
			servicequeue.NewService2(whatsonchain.ServiceName, wocService.PostBEEF),
			servicequeue.NewService2(bitails.ServiceName, bitailsService.PostBEEF),
		),

		getMerklePathServices: servicequeue.NewQueue1(
			logger,
			"MerklePath",
			servicequeue.NewService1(arc.ServiceName, arcService.MerklePath),
			servicequeue.NewService1(whatsonchain.ServiceName, wocService.MerklePath),
			servicequeue.NewService1(bitails.ServiceName, bitailsService.MerklePath),
		),

		chainHeaderServices: servicequeue.NewQueue(
			logger,
			"FindChainTipHeader",
			servicequeue.NewService(bitails.ServiceName, bitailsService.FindChainTipHeader),
			servicequeue.NewService(whatsonchain.ServiceName, wocService.FindChainTipHeader),
			servicequeue.NewService(bhs.ServiceName, bhsService.FindChainTipHeader),
		),

		validatorServices: servicequeue.NewQueue2(
			logger,
			"IsValidRootForHeight",
			servicequeue.NewService2(bhs.ServiceName, bhsService.IsValidRootForHeight),
			servicequeue.NewService2(whatsonchain.ServiceName, wocService.IsValidRootForHeight),
			servicequeue.NewService2(bitails.ServiceName, bitailsService.IsValidRootForHeight),
		),

		heightServices: servicequeue.NewQueue(
			logger,
			"CurrentHeight",
			servicequeue.NewService(bhs.ServiceName, bhsService.CurrentHeight),
			servicequeue.NewService(whatsonchain.ServiceName, wocService.CurrentHeight),
			servicequeue.NewService(bitails.ServiceName, bitailsService.CurrentHeight),
		),

		scriptHistoryServices: servicequeue.NewQueue1(
			logger,
			"GetScriptHashHistory",
			servicequeue.NewService1(whatsonchain.ServiceName, wocService.GetScriptHashHistory),
			servicequeue.NewService1(bitails.ServiceName, bitailsService.GetScriptHashHistory),
		),

		hashToHeaderServices: servicequeue.NewQueue1(
			logger,
			"HashToHeader",
			servicequeue.NewService1(bitails.ServiceName, bitailsService.HashToHeader),
			servicequeue.NewService1(whatsonchain.ServiceName, wocService.HashToHeader),
		),
	}
}

// FindChainTipHeader queries multiple chain header services in sequence
// and returns the most recent block header (chain tip) available.
func (s *WalletServices) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	result, err := s.chainHeaderServices.OneByOne(ctx)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("unable to determine chain tip: all chain header services failed to return a result: %w", err)
		}
		return nil, fmt.Errorf("failed to retrieve latest block header from chain header services: %w", err)
	}
	return result, nil
}

// RawTx attempts to obtain the raw transaction bytes associated with a 32 byte transaction hash (txid).
func (s *WalletServices) RawTx(txID string) (wdk.RawTxResult, error) {
	result, err := s.rawTxServices.OneByOne(context.TODO(), txID)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return wdk.RawTxResult{}, fmt.Errorf("transaction with txID: %s not found", txID)
		}
		return wdk.RawTxResult{}, fmt.Errorf("couldn't get rawtx for id %s: %w", txID, err)
	}
	return *result, nil
}

// ChainTracker returns service, which requires `options.chaintracks` be valid.
func (s *WalletServices) ChainTracker() chaintracker.ChainTracker {
	panic("Not implemented yet")
}

// HeaderForHeight returns serialized block header for height on active chain
func (s *WalletServices) HeaderForHeight(height int64) ([]int64, error) {
	panic("Not implemented yet")
}

// CurrentHeight returns the height of the active chain
func (s *WalletServices) CurrentHeight(ctx context.Context) (uint32, error) {
	h, err := s.heightServices.OneByOne(ctx)
	if err != nil {
		return 0, fmt.Errorf("all CurrentHeight providers failed: %w", err)
	}
	return h, nil
}

// BsvExchangeRate returns approximate exchange rate US Dollar / BSV, USD / BSV
// This is the US Dollar price of one BSV
func (s *WalletServices) BsvExchangeRate() (float64, error) {
	bsvExchangeRate, err := s.whatsonchain.UpdateBsvExchangeRate()
	if err != nil {
		return 0, fmt.Errorf("error during bsvExchangeRate: %w", err)
	}

	return bsvExchangeRate.Rate, nil
}

// FiatExchangeRate returns approximate exchange rate currency per base.
func (s *WalletServices) FiatExchangeRate(currency defs.Currency, base *defs.Currency) float64 {
	panic("Not implemented yet")
}

// MerklePath attempts to obtain the merkle proof associated with a 32 byte transaction hash (txid).
func (s *WalletServices) MerklePath(ctx context.Context, txid string) (*wdk.MerklePathResult, error) {
	result, err := s.getMerklePathServices.OneByOne(ctx, txid)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("transaction with txID: %s not found", txid)
		}
		return nil, fmt.Errorf("couldn't get merkle path for id %s: %w", txid, err)
	}
	return result, nil
}

// PostBEEF attempts to post beef with given txIDs
func (s *WalletServices) PostBEEF(ctx context.Context, beef *transaction.Beef, txids []string) (wdk.PostBeefResult, error) {
	res, err := s.postBEEFServices.All(ctx, beef, txids)
	if err != nil {
		return nil, fmt.Errorf("failed to PostBEEF: %w", err)
	}

	postBEEFResults := slices.Map(res, func(it *servicequeue.NamedResult[*wdk.PostedBEEF]) *wdk.PostBEEFServiceResult {
		if it.IsError() {
			return &wdk.PostBEEFServiceResult{
				Name:  it.Name(),
				Error: it.MustGetError(),
			}
		}
		return &wdk.PostBEEFServiceResult{
			Name:             it.Name(),
			PostedBEEFResult: it.MustGetValue(),
		}
	})

	return postBEEFResults, nil
}

// UtxoStatus attempts to determine the UTXO status of a transaction output.
//
// Cycles through configured transaction processing services attempting to get a valid response.
func (s *WalletServices) UtxoStatus(
	output string,
	outputFormat UtxoStatusOutputFormat,
	useNext bool,
) (UtxoStatusResult, error) {
	panic("Not implemented yet")
}

// NLockTimeIsFinal returns whether the locktime value allows the transaction to be mined at the current chain height
// TODO: txOrLockTime type = string | number[] | BsvTransaction | number
func (s *WalletServices) NLockTimeIsFinal(txOrLockTime any) bool {
	panic("Not implemented yet")
}

// IsValidRootForHeight verifies the Merkle-root for a block height.
func (s *WalletServices) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error) {
	ok, err := s.validatorServices.OneByOne(ctx, root, height)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return false, fmt.Errorf("all IsValidRootForHeight providers failed for height %d", height)
		}
		return false, fmt.Errorf("failed to validate Merkle root %s for height %d: %w", root, height, err)
	}
	return ok, nil
}

// GetScriptHashHistory retrieves both confirmed and unconfirmed transaction history for a script hash
func (s *WalletServices) GetScriptHashHistory(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error) {
	result, err := s.scriptHistoryServices.OneByOne(ctx, scriptHash)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("script hash %s not found in history", scriptHash)
		}
		return nil, fmt.Errorf("failed to get script history: %w", err)
	}
	return result, nil
}

// HashToHeader attempts to retrieve BlockHeader by its hash
func (s *WalletServices) HashToHeader(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error) {
	result, err := s.hashToHeaderServices.OneByOne(ctx, hash)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("block hash %s not found in any header service", hash)
		}
		return nil, fmt.Errorf("couldn't get block header for hash %s: %w", hash, err)
	}
	return result, nil
}
