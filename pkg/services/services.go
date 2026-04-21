package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	ctConfig "github.com/bsv-blockchain/go-chaintracks/config"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracksclient"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/servicequeue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// WalletServices is a struct that contains services used by a wallet
type WalletServices struct {
	logger *slog.Logger
	chain  defs.BSVNetwork
	config *defs.WalletServices
	// NOTE: add p2p client here when arcade is implemented so they can share clients

	rawTxServices  servicequeue.Queue1[string, *wdk.RawTxResult]
	postEFServices servicequeue.Queue2[string, string, *wdk.PostedTxID]
	postTXServices servicequeue.Queue1[[]byte, *wdk.PostedTxID]

	merklePathServices           servicequeue.Queue1[string, *wdk.MerklePathResult]
	findChainTipHeaderServices   servicequeue.Queue[*wdk.ChainBlockHeader]
	isValidRootForHeightServices servicequeue.Queue2[*chainhash.Hash, uint32, bool]
	currentHeightServices        servicequeue.Queue[uint32]
	getScriptHashHistoryServices servicequeue.Queue1[string, *wdk.ScriptHistoryResult]
	chainHeaderByHeightServices  servicequeue.Queue1[uint32, *wdk.ChainBlockHeader]
	hashToHeaderServices         servicequeue.Queue1[string, *wdk.ChainBlockHeader]
	getUtxoStatusServices        servicequeue.Queue2[string, *transaction.Outpoint, *wdk.UtxoStatusResult]
	isUtxoServices               servicequeue.Queue2[string, *transaction.Outpoint, bool]
	getStatusForTxIDsServices    servicequeue.Queue1[[]string, *wdk.GetStatusForTxIDsResult]
	bsvExchangeRateServices      servicequeue.Queue[float64]

	// chaintracks integration
	chaintracks    *chaintracksclient.Adapter
	reorgBroadcast *reorgBroadcaster
	tipBroadcast   *tipBroadcaster
}

// New will return a new WalletServices
func New(logger *slog.Logger, config defs.WalletServices, opts ...func(*Options)) *WalletServices {
	options := to.OptionsWithDefault(Options{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}, opts...)

	if err := config.Chain.Validate(); err != nil {
		panic(fmt.Errorf("invalid chain %q: %w", config.Chain, err))
	}

	var predefined []Named[Implementation]

	if config.ArcConfig.Enabled {
		arcService := arc.New(logger, options.RestyClientFactory.New(), config.ArcConfig)
		predefined = append(predefined, Named[Implementation]{
			Name: arc.ServiceName,
			Item: Implementation{
				PostEF:     arcService.PostEF,
				MerklePath: arcService.MerklePath,
			},
		})
	}

	if config.BHS.Enabled {
		bhsService := bhs.New(options.RestyClientFactory.New(), logger, config.Chain, config.BHS)
		predefined = append(predefined, Named[Implementation]{
			Name: bhs.ServiceName,
			Item: Implementation{
				FindChainTipHeader:   bhsService.FindChainTipHeader,
				IsValidRootForHeight: bhsService.IsValidRootForHeight,
				CurrentHeight:        bhsService.CurrentHeight,
				ChainHeaderByHeight:  bhsService.ChainHeaderByHeight,
			},
		})
	}

	if config.WhatsOnChain.Enabled {
		wocService := whatsonchain.New(options.RestyClientFactory.New(), logger, config.Chain, config.WhatsOnChain)
		predefined = append(predefined, Named[Implementation]{
			Name: whatsonchain.ServiceName,
			Item: Implementation{
				RawTx:                wocService.RawTx,
				PostTX:               wocService.PostTX,
				MerklePath:           wocService.MerklePath,
				FindChainTipHeader:   wocService.FindChainTipHeader,
				IsValidRootForHeight: wocService.IsValidRootForHeight,
				CurrentHeight:        wocService.CurrentHeight,
				GetScriptHashHistory: wocService.GetScriptHashHistory,
				HashToHeader:         wocService.HashToHeader,
				ChainHeaderByHeight:  wocService.ChainHeaderByHeight,
				GetStatusForTxIDs:    wocService.GetStatusForTxIDs,
				GetUtxoStatus:        wocService.GetUtxoStatus,
				IsUtxo:               wocService.IsUtxo,
				BsvExchangeRate:      wocService.UpdateBsvExchangeRate,
			},
		})
	}

	if config.Bitails.Enabled {
		bitailsService := bitails.New(options.RestyClientFactory.New(), logger, config.Chain, config.Bitails)
		predefined = append(predefined, Named[Implementation]{
			Name: bitails.ServiceName,
			Item: Implementation{
				RawTx:                bitailsService.RawTx,
				PostTX:               bitailsService.PostTX,
				MerklePath:           bitailsService.MerklePath,
				FindChainTipHeader:   bitailsService.FindChainTipHeader,
				IsValidRootForHeight: bitailsService.IsValidRootForHeight,
				CurrentHeight:        bitailsService.CurrentHeight,
				GetScriptHashHistory: bitailsService.GetScriptHashHistory,
				HashToHeader:         bitailsService.HashToHeader,
				ChainHeaderByHeight:  bitailsService.ChainHeaderByHeight,
				GetStatusForTxIDs:    bitailsService.GetStatusForTxIDs,
			},
		})
	}

	var chaintracksAdapter *chaintracksclient.Adapter
	var reorgBroadcast *reorgBroadcaster
	var tipBroadcast *tipBroadcaster

	if config.ChaintracksClient.Enabled {
		if options.chaintracksAdapter != nil {
			// Use injected adapter (mostly for testing)
			chaintracksAdapter = options.chaintracksAdapter
		} else {
			// Create adapter from config
			ctCfg := &ctConfig.Config{
				Mode: ctConfig.ModeRemote,
				URL:  config.ChaintracksClient.RemoteURL,
			}

			if config.ChaintracksClient.Mode == defs.ChaintracksClientModeEmbedded {
				ctCfg.Mode = ctConfig.ModeEmbedded
				ctCfg.BootstrapURL = config.ChaintracksClient.BootstrapURL
				ctCfg.BootstrapMode = ctConfig.BootstrapMode(config.ChaintracksClient.BootstrapMode)
				ctCfg.StoragePath = config.ChaintracksClient.StoragePath
				ctCfg.P2P.Network = config.ChaintracksClient.P2PNetwork
				ctCfg.P2P.StoragePath = config.ChaintracksClient.P2PStoragePath
			}

			// NOTE: when added Arcade we can add here P2P initialization if required
			adapter, err := chaintracksclient.New(logger, ctCfg)
			if err != nil {
				panic(fmt.Errorf("failed to initialize chaintracks: %w", err))
			}

			chaintracksAdapter = adapter
		}

		reorgBroadcast = newReorgBroadcaster(logger)
		tipBroadcast = newTipBroadcaster(logger)
	}

	// Register chaintracks implementation if adapter is available
	if chaintracksAdapter != nil {
		predefined = append(predefined, Named[Implementation]{
			Name: defs.ChaintracksServiceName,
			Item: Implementation{
				CurrentHeight:        chaintracksAdapter.CurrentHeight,
				ChainHeaderByHeight:  chaintracksAdapter.GetHeaderByHeight,
				HashToHeader:         chaintracksAdapter.GetHeaderByHash,
				FindChainTipHeader:   chaintracksAdapter.GetTip,
				IsValidRootForHeight: chaintracksAdapter.IsValidRootForHeight,
			},
		})
	}

	allImplementations := append(options.customImplementations, predefined...)

	walletServices := &WalletServices{
		chain:  config.Chain,
		config: &config,
		logger: logger,

		rawTxServices: servicequeue.NewQueue1(
			logger,
			"RawTx",
			namedFuncsToServices1(
				applyModifierIfExists(options.RawTxMethodsModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) RawTxFunc {
						return it.RawTx
					})))...,
		),

		postEFServices: servicequeue.NewQueue2(
			logger,
			"PostEF",
			namedFuncsToServices2(
				applyModifierIfExists(options.PostEFMethodsModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) PostEFFunc {
						return it.PostEF
					})))...,
		),

		postTXServices: servicequeue.NewQueue1(
			logger,
			"PostTX",
			namedFuncsToServices1(
				applyModifierIfExists(options.PostTXMethodsModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) PostTXFunc {
						return it.PostTX
					})))...,
		),

		merklePathServices: servicequeue.NewQueue1(
			logger,
			"MerklePath",
			namedFuncsToServices1(
				applyModifierIfExists(options.MerklePathMethodsModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) MerklePathFunc {
						return it.MerklePath
					})))...,
		),

		findChainTipHeaderServices: servicequeue.NewQueue(
			logger,
			"FindChainTipHeader",
			namedFuncsToServices(
				applyModifierIfExists(options.FindChainTipHeaderModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) FindChainTipHeaderFunc {
						return it.FindChainTipHeader
					})))...,
		),

		isValidRootForHeightServices: servicequeue.NewQueue2(
			logger,
			"IsValidRootForHeight",
			namedFuncsToServices2(
				applyModifierIfExists(options.IsValidRootForHeightModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) IsValidRootForHeightFunc {
						return it.IsValidRootForHeight
					})))...,
		),

		currentHeightServices: servicequeue.NewQueue(
			logger,
			"CurrentHeight",
			namedFuncsToServices(
				applyModifierIfExists(options.CurrentHeightModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) CurrentHeightFunc {
						return it.CurrentHeight
					})))...,
		),

		getScriptHashHistoryServices: servicequeue.NewQueue1(
			logger,
			"GetScriptHashHistory",
			namedFuncsToServices1(
				applyModifierIfExists(options.GetScriptHashHistoryModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) GetScriptHashHistoryFunc {
						return it.GetScriptHashHistory
					})))...,
		),

		hashToHeaderServices: servicequeue.NewQueue1(
			logger,
			"HashToHeader",
			namedFuncsToServices1(
				applyModifierIfExists(options.HashToHeaderModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) HashToHeaderFunc {
						return it.HashToHeader
					})))...,
		),

		chainHeaderByHeightServices: servicequeue.NewQueue1(
			logger,
			"ChainHeaderByHeight",
			namedFuncsToServices1(
				applyModifierIfExists(options.ChainHeaderByHeightModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) ChainHeaderByHeightFunc {
						return it.ChainHeaderByHeight
					})))...,
		),

		getStatusForTxIDsServices: servicequeue.NewQueue1(
			logger,
			"GetStatusForTxIDs",
			namedFuncsToServices1(
				applyModifierIfExists(options.GetStatusForTxIDsModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) GetStatusForTxIDsFunc {
						return it.GetStatusForTxIDs
					})))...,
		),

		getUtxoStatusServices: servicequeue.NewQueue2(
			logger,
			"GetUtxoStatus",
			namedFuncsToServices2(
				applyModifierIfExists(options.GetUtxoStatusModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) GetUtxoStatusFunc {
						return it.GetUtxoStatus
					})))...,
		),

		isUtxoServices: servicequeue.NewQueue2(
			logger,
			"IsUtxo",
			namedFuncsToServices2(
				applyModifierIfExists(options.IsUtxoModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) IsUtxo {
						return it.IsUtxo
					})))...,
		),

		bsvExchangeRateServices: servicequeue.NewQueue(
			logger,
			"BsvExchangeRate",
			namedFuncsToServices(
				applyModifierIfExists(options.BsvExchangeRateModifier,
					collectSingleMethodImplementations(allImplementations, func(it Implementation) BsvExchangeRateFunc {
						return it.BsvExchangeRate
					})))...,
		),

		chaintracks:    chaintracksAdapter,
		reorgBroadcast: reorgBroadcast,
		tipBroadcast:   tipBroadcast,
	}

	walletServices.logActiveServices()
	return walletServices
}

func (s *WalletServices) logActiveServices() {
	if !s.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	type loggable interface {
		GetNames() (methodName string, serviceNames []string)
	}

	services := []loggable{
		&s.rawTxServices,
		&s.postEFServices,
		&s.postTXServices,
		&s.merklePathServices,
		&s.findChainTipHeaderServices,
		&s.isValidRootForHeightServices,
		&s.currentHeightServices,
		&s.getScriptHashHistoryServices,
		&s.hashToHeaderServices,
		&s.chainHeaderByHeightServices,
		&s.getStatusForTxIDsServices,
		&s.getUtxoStatusServices,
		&s.isUtxoServices,
		&s.bsvExchangeRateServices,
	}

	logAttrs := slices.Map(services, func(e loggable) any {
		methodName, serviceNames := e.GetNames()
		return slog.String(methodName, strings.Join(serviceNames, ","))
	})

	s.logger.Debug("Active services by methods", logAttrs...)
}

// StartChaintracks begins background chaintracks event subscription.
// Must be called after New() to start listening for blockchain events.
func (s *WalletServices) StartChaintracks(ctx context.Context) error {
	if s.chaintracks == nil {
		return nil // chaintracks is disabled
	}

	err := s.chaintracks.Start(ctx, chaintracksclient.Callbacks{
		OnTip: func(bh *chaintracks.BlockHeader) error {
			s.logger.Debug("new chain tip received",
				"height", bh.Height,
				"hash", bh.Hash.String(),
			)
			s.tipBroadcast.broadcast(bh)
			return nil
		},
		OnReorg: func(event *chaintracks.ReorgEvent) error {
			s.logger.Info("reorg detected",
				"depth", event.Depth,
				"new_tip_hash", event.NewTip.Hash.String(),
				"orphaned_count", len(event.OrphanedHashes),
			)
			s.reorgBroadcast.broadcast(event)
			return nil
		},
	})
	if err != nil {
		return fmt.Errorf("failed to start chaintracks: %w", err)
	}

	s.logger.Info("chaintracks started")
	return nil
}

// SubscribeReorgs registers a user-provided channel to receive reorg events.
// The caller is responsible for creating the channel with an appropriate buffer size
// and closing it after unsubscribing.
// Returns an unsubscribe function, or nil if chaintracks is not enabled.
func (s *WalletServices) SubscribeReorgs(ch chan *chaintracks.ReorgEvent) func() {
	if s.reorgBroadcast == nil {
		return nil
	}
	return s.reorgBroadcast.Subscribe(ch)
}

// SubscribeTips registers a user-provided channel to receive new tip events.
// The caller is responsible for creating the channel with an appropriate buffer size
// and closing it after unsubscribing.
// Returns an unsubscribe function, or nil if chaintracks is not enabled.
func (s *WalletServices) SubscribeTips(ch chan *chaintracks.BlockHeader) func() {
	if s.tipBroadcast == nil {
		return nil
	}
	return s.tipBroadcast.Subscribe(ch)
}

// FindChainTipHeader queries multiple chain header services in sequence
// and returns the most recent block header (chain tip) available.
func (s *WalletServices) FindChainTipHeader(ctx context.Context) (_ *wdk.ChainBlockHeader, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-FindChainTipHeader")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result, err := s.findChainTipHeaderServices.OneByOne(ctx)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("unable to determine chain tip: all chain header services failed to return a result: %w", err)
		}
		return nil, fmt.Errorf("failed to retrieve latest block header from chain header services: %w", err)
	}
	return result, nil
}

// RawTx attempts to obtain the raw transaction bytes associated with a 32 byte transaction hash (txid).
func (s *WalletServices) RawTx(ctx context.Context, txID string) (_ wdk.RawTxResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-RawTx")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result, err := s.rawTxServices.OneByOne(ctx, txID)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return wdk.RawTxResult{}, fmt.Errorf("transaction with txID: %s not found", txID)
		}
		return wdk.RawTxResult{}, fmt.Errorf("couldn't get rawtx for id %s: %w", txID, err)
	}
	return *result, nil
}

// ChainHeaderByHeight returns serialized block header for given height on active chain.
func (s *WalletServices) ChainHeaderByHeight(ctx context.Context, height uint32) (_ *wdk.ChainBlockHeader, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-ChainHeaderByHeight")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	h, err := s.chainHeaderByHeightServices.OneByOne(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("unable to determine block header: all block header height services failed to return a result: %w", err)
	}
	return h, nil
}

// CurrentHeight returns the height of the active chain
func (s *WalletServices) CurrentHeight(ctx context.Context) (_ uint32, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-CurrentHeight")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	h, err := s.currentHeightServices.OneByOne(ctx)
	if err != nil {
		return 0, fmt.Errorf("all CurrentHeight providers failed: %w", err)
	}
	return h, nil
}

// BsvExchangeRate returns approximate exchange rate US Dollar / BSV, USD / BSV
// This is the US Dollar price of one BSV
func (s *WalletServices) BsvExchangeRate(ctx context.Context) (_ float64, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-BsvExchangeRate")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	bsvExchangeRate, err := s.bsvExchangeRateServices.OneByOne(ctx)
	if err != nil {
		return 0, fmt.Errorf("error during bsvExchangeRate: %w", err)
	}

	return bsvExchangeRate, nil
}

// MerklePath attempts to obtain the merkle proof associated with a 32 byte transaction hash (txid).
func (s *WalletServices) MerklePath(ctx context.Context, txid string) (_ *wdk.MerklePathResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-MerklePath")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result, err := s.merklePathServices.OneByOne(ctx, txid)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("transaction with txID: %s not found: %w", txid, wdk.ErrNotFoundError)
		}
		return nil, fmt.Errorf("couldn't get merkle path for id %s: %w", txid, err)
	}
	return result, nil
}

// PostFromBEEF attempts to broadcast transactions from BEEF to all configured services.
func (s *WalletServices) PostFromBEEF(ctx context.Context, beef *transaction.Beef, txIDs []string) (_ wdk.PostFromBeefResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-PostFromBEEF")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	allResults := make([]*wdk.PostFromBEEFServiceResult, 0)

	txutils.BindBumpsAndTransactions(beef, s.logger)

	// check if beef contains only one child transaction
	if err := txutils.ValidateSingleLeafTx(beef); err != nil {
		// Return error as service error for each txID, not as Go error
		return []*wdk.PostFromBEEFServiceResult{{
			Name:  "PostFromBEEF Validation",
			Error: err,
		}}, nil
	}

	// hydrate txs in beef
	if err := txutils.HydrateBEEF(beef); err != nil {
		return nil, fmt.Errorf("failed to hydrate beef for script verification: %w", err)
	}

	for _, txID := range txIDs {
		tx := beef.FindTransaction(txID)
		if tx == nil {
			return nil, fmt.Errorf("transaction %s not found in beef", txID)
		}

		// skip already mined txs
		if tx.MerklePath != nil {
			continue
		}

		rawTx := tx.Bytes()
		efHex, err := tx.EFHex()
		if err != nil {
			return nil, fmt.Errorf("failed to get efhex from tx %s: %w", tx.TxID().String(), err)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to convert tx %s to EF: %w", txID, err)
		}

		efResults, efErr := s.postEFServices.All(ctx, efHex, txID)

		txResults, txErr := s.postTXServices.All(ctx, rawTx)

		if errors.Is(efErr, servicequeue.ErrNoServicesRegistered) && errors.Is(txErr, servicequeue.ErrNoServicesRegistered) {
			return nil, fmt.Errorf("no services registered for broadcasting")
		}

		if efErr == nil {
			allResults = append(allResults, slices.Map(efResults, s.mapToPostBEEFServiceResult)...)
		}
		if txErr == nil {
			allResults = append(allResults, slices.Map(txResults, s.mapToPostBEEFServiceResult)...)
		}
	}

	return allResults, nil
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
func (s *WalletServices) IsValidRootForHeight(ctx context.Context, root *chainhash.Hash, height uint32) (_ bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-IsValidRootForHeight")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	ok, err := s.isValidRootForHeightServices.OneByOne(ctx, root, height)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return false, fmt.Errorf("all IsValidRootForHeight providers failed for height %d", height)
		}
		return false, fmt.Errorf("failed to validate Merkle root %s for height %d: %w", root, height, err)
	}
	return ok, nil
}

// GetScriptHashHistory retrieves both confirmed and unconfirmed transaction history for a script hash
func (s *WalletServices) GetScriptHashHistory(ctx context.Context, scriptHash string) (_ *wdk.ScriptHistoryResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetScriptHashHistory")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result, err := s.getScriptHashHistoryServices.OneByOne(ctx, scriptHash)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("script hash %s not found in history", scriptHash)
		}
		return nil, fmt.Errorf("failed to get script history: %w", err)
	}
	return result, nil
}

// HashToHeader attempts to retrieve BlockHeader by its hash
func (s *WalletServices) HashToHeader(ctx context.Context, hash string) (_ *wdk.ChainBlockHeader, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-HashToHeader")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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
func (s *WalletServices) GetUtxoStatus(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (_ *wdk.UtxoStatusResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetUtxoStatus")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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
func (s *WalletServices) IsUtxo(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (_ bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-IsUtxo")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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
func (s *WalletServices) GetStatusForTxIDs(ctx context.Context, txIDs []string) (_ *wdk.GetStatusForTxIDsResult, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetStatusForTxIDs")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txIDs provided")
	}

	res, err := s.getStatusForTxIDsServices.OneByOne(ctx, txIDs)
	if err != nil {
		if errors.Is(err, servicequeue.ErrEmptyResult) {
			return nil, fmt.Errorf("no status found for provided txIDs: %w", wdk.ErrNotFoundError)
		}
		return nil, fmt.Errorf("failed to get status for txIDs: %w", err)
	}
	return res, nil
}

// GetBEEF retrieves the BEEF structure for a given transaction ID.
// It recursively fetches transaction ancestry up to a configured depth limit and merges transaction data, merkle paths, and input ancestry into the BEEF structure.
// Use optional knownTxIDs to skip fetching of already-known transactions in the ancestry tree.
func (s *WalletServices) GetBEEF(ctx context.Context, txID string, knownTxIDs []string) (_ *transaction.Beef, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-GetBEEF")
	defer func() {
		tracing.EndTracing(span, err)
	}()

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

		var rawTxResult wdk.RawTxResult
		rawTxResult, err = s.RawTx(ctx, txID)
		if err != nil {
			return fmt.Errorf("failed to get raw transaction for txID %q: %w", txID, err)
		}

		if rawTxResult.RawTx == nil {
			return fmt.Errorf("raw transaction for txID %s is nil", txID)
		}

		var tx *transaction.Transaction
		tx, err = transaction.NewTransactionFromBytes(rawTxResult.RawTx)
		if err != nil {
			return fmt.Errorf("failed to create transaction from raw bytes for txID %q: %w", txID, err)
		}

		var merklePathResult *wdk.MerklePathResult
		merklePathResult, err = s.MerklePath(ctx, txID)
		if err != nil && !errors.Is(err, wdk.ErrNotFoundError) {
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

	err = txGetter(txID, 0)
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

// FiatExchangeRate returns approximate exchange rate currency per base.
// Uses config.FiatExchangeRates as the source.
func (s *WalletServices) FiatExchangeRate(currency defs.Currency, base *defs.Currency) float64 {
	rates := s.config.FiatExchangeRates.Rates

	baseCurrency := defs.USD
	if base != nil {
		baseCurrency = *base
	}

	currencyRate, ok1 := rates[currency]
	baseRate, ok2 := rates[baseCurrency]

	if !ok1 || !ok2 || baseRate == 0 {
		return 0
	}

	return currencyRate / baseRate
}

func (s *WalletServices) mapToPostBEEFServiceResult(r *servicequeue.NamedResult[*wdk.PostedTxID]) *wdk.PostFromBEEFServiceResult {
	if r.IsError() {
		return &wdk.PostFromBEEFServiceResult{
			Name:  r.Name(),
			Error: r.MustGetError(),
		}
	}
	return &wdk.PostFromBEEFServiceResult{
		Name:             r.Name(),
		PostedBEEFResult: &wdk.PostedBEEF{TxIDResults: []wdk.PostedTxID{*r.MustGetValue()}},
	}
}
