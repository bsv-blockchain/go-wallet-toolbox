package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	stdslices "slices"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
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

	rawTxServices                servicequeue.Queue1[string, *wdk.RawTxResult]
	postBEEFServices             servicequeue.Queue2[*transaction.Beef, []string, *wdk.PostedBEEF]
	getMerklePathServices        servicequeue.Queue1[string, *wdk.MerklePathResult]
	chainHeaderServices          servicequeue.Queue[*wdk.ChainBlockHeader]
	validatorServices            servicequeue.Queue2[*chainhash.Hash, uint32, bool]
	heightServices               servicequeue.Queue[uint32]
	scriptHistoryServices        servicequeue.Queue1[string, *wdk.ScriptHistoryResult]
	blockHeaderForHeightServices servicequeue.Queue1[uint32, *wdk.ChainBaseBlockHeader]
	hashToHeaderServices         servicequeue.Queue1[string, *wdk.ChainBlockHeader]
	getUtxoStatusServices        servicequeue.Queue2[string, *transaction.Outpoint, *wdk.UtxoStatusResult]
	isUtxoServices               servicequeue.Queue2[string, *transaction.Outpoint, bool]
	getStatusForTxIDsServices    servicequeue.Queue1[[]string, *wdk.GetStatusForTxIDsResult]
}

func applyModifierIfExists[F any](modifier func([]NamedFunc[F]) []NamedFunc[F], predefined []NamedFunc[F]) []NamedFunc[F] {
	funcs := predefined
	if modifier != nil {
		funcs = modifier(funcs)
	}

	return funcs
}

func namedFuncsToServices1[A, R any](namedFuncs []NamedFunc[func(context.Context, A) (R, error)]) []*servicequeue.Service1[A, R] {
	return slices.Map(namedFuncs, func(it NamedFunc[func(context.Context, A) (R, error)]) *servicequeue.Service1[A, R] {
		return servicequeue.NewService1(it.Name, it.Func)
	})
}

func namedFuncsToServices2[A, B, R any](namedFuncs []NamedFunc[func(context.Context, A, B) (R, error)]) []*servicequeue.Service2[A, B, R] {
	return slices.Map(namedFuncs, func(it NamedFunc[func(context.Context, A, B) (R, error)]) *servicequeue.Service2[A, B, R] {
		return servicequeue.NewService2(it.Name, it.Func)
	})
}

func toNamedFuncs[F any](servicesDefinitions []allServicesDefinitionItem, selector func(it AllServicesDefinition) F) []NamedFunc[F] {
	var funcs []NamedFunc[F]
	for _, it := range servicesDefinitions {
		f := selector(it.AllServicesDefinition)
		if reflect.ValueOf(f).IsNil() {
			continue
		}

		funcs = append(funcs, NamedFunc[F]{Name: it.Name, Func: f})
	}
	return funcs
}

// New will return a new WalletServices
func New(logger *slog.Logger, config defs.WalletServices, opts ...func(*Options)) *WalletServices {
	options := to.OptionsWithDefault(Options{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}, opts...)

	if config.Chain == "" {
		panic("chain is required")
	}

	wocService := whatsonchain.New(options.RestyClientFactory.New(), logger, config.Chain, config.WhatsOnChain)
	arcService := arc.New(logger, options.RestyClientFactory.New(), config.ArcConfig)
	bitailsService := bitails.New(options.RestyClientFactory.New(), logger, config.Chain, config.Bitails)
	bhsService := bhs.New(options.RestyClientFactory.New(), logger, config.Chain, config.BHS)

	var servicesDefinition []allServicesDefinitionItem

	if config.WhatsOnChain.Enabled {
		servicesDefinition = append(servicesDefinition, allServicesDefinitionItem{
			Name:     whatsonchain.ServiceName,
			Priority: 4,
			AllServicesDefinition: AllServicesDefinition{
				RawTx: wocService.RawTx,
			},
		})
	}

	if config.Bitails.Enabled {
		servicesDefinition = append(servicesDefinition, allServicesDefinitionItem{
			Name:     bitails.ServiceName,
			Priority: 2,
			AllServicesDefinition: AllServicesDefinition{
				RawTx: bitailsService.RawTx,
			},
		})
	}

	for _, item := range options.servicesDefinitionItems {
		// TODO: Check if names collide
		servicesDefinition = append(servicesDefinition, item)
	}
	// Sort by priority descending
	stdslices.SortFunc(servicesDefinition, func(a, b allServicesDefinitionItem) int {
		return b.Priority - a.Priority
	})

	return &WalletServices{
		chain:        config.Chain,
		config:       &config,
		logger:       logger,
		whatsonchain: wocService,

		rawTxServices: servicequeue.NewQueue1(
			logger,
			"RawTx",
			namedFuncsToServices1(
				applyModifierIfExists(options.RawTxMethodsModifier,
					toNamedFuncs(servicesDefinition, func(it AllServicesDefinition) RawTxFunc {
						return it.RawTx
					})))...,
		),

		postBEEFServices: servicequeue.NewQueue2(
			logger,
			"PostBEEF",
			namedFuncsToServices2(applyModifierIfExists(options.PostBEEFMethodsModifier, []NamedFunc[PostBEEFFunc]{
				{arc.ServiceName, arcService.PostBEEF},
				{whatsonchain.ServiceName, wocService.PostBEEF},
				{bitails.ServiceName, bitailsService.PostBEEF},
			}))...,
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

		blockHeaderForHeightServices: servicequeue.NewQueue1(
			logger,
			"GetChainHeaderByHeight",
			servicequeue.NewService1(bhs.ServiceName, bhsService.GetChainHeaderByHeight),
			servicequeue.NewService1(whatsonchain.ServiceName, wocService.GetChainHeaderByHeight),
			servicequeue.NewService1(bitails.ServiceName, bitailsService.GetChainHeaderByHeight),
		),

		getStatusForTxIDsServices: servicequeue.NewQueue1(
			logger,
			"GetStatusForTxIDs",
			servicequeue.NewService1(whatsonchain.ServiceName, wocService.GetStatusForTxIDs),
			servicequeue.NewService1(bitails.ServiceName, bitailsService.GetStatusForTxIDs),
		),

		getUtxoStatusServices: servicequeue.NewQueue2(
			logger,
			"GetUtxoStatus",
			servicequeue.NewService2(whatsonchain.ServiceName, wocService.GetUtxoStatus),
		),

		isUtxoServices: servicequeue.NewQueue2(
			logger,
			"IsUtxo",
			servicequeue.NewService2(whatsonchain.ServiceName, wocService.IsUtxo),
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
func (s *WalletServices) RawTx(ctx context.Context, txID string) (wdk.RawTxResult, error) {
	result, err := s.rawTxServices.OneByOne(ctx, txID)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return wdk.RawTxResult{}, fmt.Errorf("transaction with txID: %s not found", txID)
		}
		return wdk.RawTxResult{}, fmt.Errorf("couldn't get rawtx for id %s: %w", txID, err)
	}
	return *result, nil
}

// GetChainHeaderByHeight returns serialized block header for given height on active chain.
func (s *WalletServices) GetChainHeaderByHeight(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error) {
	h, err := s.blockHeaderForHeightServices.OneByOne(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("unable to determine block header: all block header height services failed to return a result: %w", err)
	}
	return h, nil
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
			return nil, fmt.Errorf("transaction with txID: %s not found: %w", txid, wdk.NotFoundError)
		}
		return nil, fmt.Errorf("couldn't get merkle path for id %s: %w", txid, err)
	}
	return result, nil
}

// PostBEEF attempts to post beef with given txIDs
func (s *WalletServices) PostBEEF(ctx context.Context, beef *transaction.Beef, txIDs []string) (wdk.PostBeefResult, error) {
	res, err := s.postBEEFServices.All(ctx, beef, txIDs)
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

// GetUtxoStatus retrieves the UTXO status for a given script hash and outpoint.
func (s *WalletServices) GetUtxoStatus(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error) {
	result, err := s.getUtxoStatusServices.OneByOne(ctx, scriptHash, outpoint)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("no UTXO status found for script hash %s", scriptHash)
		}
		return nil, fmt.Errorf("failed to get UTXO status: %w", err)
	}
	return result, nil
}

// IsUtxo checks if the given outpoint is a UTXO for the specified script hash.
func (s *WalletServices) IsUtxo(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error) {
	if scriptHash == "" {
		return false, fmt.Errorf("scriptHash is required")
	}
	if outpoint == nil {
		return false, fmt.Errorf("outpoint is required")
	}

	result, err := s.isUtxoServices.OneByOne(ctx, scriptHash, outpoint)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return false, fmt.Errorf("no UTXO status found for script hash %s and outpoint %s", scriptHash, outpoint)
		}
		return false, fmt.Errorf("failed to check UTXO status: %w", err)
	}

	return result, nil
}

// GetStatusForTxIDs returns depth/status info for a list of txIDs.
func (s *WalletServices) GetStatusForTxIDs(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error) {
	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}

	res, err := s.getStatusForTxIDsServices.OneByOne(ctx, txIDs)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("no status found for provided txIDs: %w", wdk.NotFoundError)
		}
		return nil, fmt.Errorf("failed to get status for txIDs: %w", err)
	}
	return res, nil
}

// GetBEEF retrieves the BEEF structure for a given transaction ID.
// It recursively fetches transaction ancestry up to a configured depth limit and merges transaction data, merkle paths, and input ancestry into the BEEF structure.
// Use optional knownTxIDs to skip fetching of already-known transactions in the ancestry tree.
func (s *WalletServices) GetBEEF(ctx context.Context, txID string, knownTxIDs []string) (*transaction.Beef, error) {
	beef := transaction.NewBeefV2()

	knownTxIDsLookup := make(map[string]struct{}, len(knownTxIDs))
	for _, knownTxID := range knownTxIDs {
		knownTxIDsLookup[knownTxID] = struct{}{}
	}

	var txGetter func(txID string, depth uint) error
	txGetter = func(txID string, depth uint) error {
		if depth > s.config.GetBeefMaxDepth {
			return fmt.Errorf("max depth of recursion reached: %d", s.config.GetBeefMaxDepth)
		}
		rawTxResult, err := s.RawTx(ctx, txID)
		if err != nil {
			return fmt.Errorf("failed to get raw transaction for txID %q: %w", txID, err)
		}

		if rawTxResult.RawTx == nil {
			return fmt.Errorf("raw transaction for txID %s is nil", txID)
		}

		tx, err := transaction.NewTransactionFromBytes(rawTxResult.RawTx)
		if err != nil {
			return fmt.Errorf("failed to create transaction from raw bytes for txID %q: %w", txID, err)
		}

		merklePathResult, err := s.MerklePath(ctx, txID)
		if err != nil && !errors.Is(err, wdk.NotFoundError) {
			return fmt.Errorf("failed to get merkle path for txID %q: %w", txID, err)
		}

		isMined := merklePathResult != nil && merklePathResult.MerklePath != nil

		if isMined {
			tx.MerklePath = merklePathResult.MerklePath
		}

		_, err = beef.MergeTransaction(tx)
		if err != nil {
			return fmt.Errorf("failed to merge transaction txID %q: %w", txID, err)
		}

		if isMined {
			return nil
		}

		for _, input := range tx.Inputs {
			beefTx := beef.Transactions[*input.SourceTXID]
			if beefTx == nil {
				sourceTxID := input.SourceTXID.String()
				if _, exists := knownTxIDsLookup[sourceTxID]; exists {
					beef.MergeTxidOnly(input.SourceTXID)
					continue
				}

				err = txGetter(sourceTxID, depth+1)
				if err != nil {
					return fmt.Errorf("failed to get beef for txID %q at depth %d: %w", sourceTxID, depth, err)
				}
			}
		}

		return nil
	}

	err := txGetter(txID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get BEEF for subject TxID %q: %w", txID, err)
	}

	return beef, nil
}

// NLockTimeIsFinal checks if the provided value is a valid nLockTime and whether it is final.
func (s *WalletServices) NLockTimeIsFinal(ctx context.Context, txOrLockTime any) (bool, error) {
	heightProvider := s
	isFinal, err := wdk.NLockTimeIsFinal(ctx, heightProvider, txOrLockTime)
	if err != nil {
		return false, fmt.Errorf("failed to parse nLockTime or final: %w", err)
	}
	return isFinal, nil
}

// HashOutputScript returns the little-endian SHA256 hash of a hex-encoded script as a hex string.
func (s *WalletServices) HashOutputScript(scriptHex string) (string, error) {
	outputScript, err := txutils.HashOutputScript(scriptHex)
	if err != nil {
		return "", fmt.Errorf("failed to hash output script: %w", err)
	}
	return outputScript, nil
}
