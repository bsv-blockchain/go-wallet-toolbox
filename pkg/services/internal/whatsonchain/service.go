package whatsonchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain/internal/dto"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

const ServiceName = "WhatsOnChain"

type WhatsOnChain struct {
	httpClient *resty.Client
	url        string
	apiKey     string
	logger     *slog.Logger

	bsvExchangeRate   defs.BSVExchangeRate // TODO: possibly handle by some caching structure/redis
	bsvUpdateInterval time.Duration
	broadcastDelay    time.Duration
}

func New(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.WhatsOnChain) *WhatsOnChain {
	logger = logging.Child(logger, "WoC").With(slog.String("network", string(network)))

	err := network.Validate()
	if err != nil {
		panic(fmt.Sprintf("invalid BSV network configuration: %s", err.Error()))
	}

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	client := httpClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger))

	return &WhatsOnChain{
		httpClient:        client,
		apiKey:            config.APIKey,
		url:               fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s", network),
		logger:            logger,
		bsvExchangeRate:   config.BSVExchangeRate,
		bsvUpdateInterval: to.If(config.BSVUpdateInterval != nil, func() time.Duration { return *config.BSVUpdateInterval }).ElseThen(defs.DefaultBSVExchangeUpdateInterval),
		broadcastDelay:    config.BroadcastDelay,
	}
}

func (woc *WhatsOnChain) RawTx(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
	req := woc.httpClient.
		R().
		SetContext(ctx).
		SetHeader("Cache-Control", "no-cache")

	res, err := req.Get(fmt.Sprintf("%s/tx/%s/hex", woc.url, txID))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch raw tx hex: %w", err)
	}
	if res.StatusCode() == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve successful response from WOC. Actual status: %d", res.StatusCode())
	}

	txHexDecoded, err := hex.DecodeString(res.String())
	if err != nil {
		return nil, fmt.Errorf("failed to decode raw transaction hex: %w", err)
	}

	txIDFromRawTx := txutils.TransactionIDFromRawTx(txHexDecoded)
	if txID != txIDFromRawTx {
		return nil, fmt.Errorf("computed txid %s doesn't match requested value %s", txIDFromRawTx, txID)
	}

	return &wdk.RawTxResult{
		Name:  ServiceName,
		TxID:  txID,
		RawTx: txHexDecoded,
	}, nil
}

func (woc *WhatsOnChain) UpdateBsvExchangeRate() (defs.BSVExchangeRate, error) {
	nextUpdate := woc.bsvExchangeRate.Timestamp.Add(woc.bsvUpdateInterval)

	// Check if the rate timestamp is newer than the threshold time
	if nextUpdate.After(time.Now()) {
		return woc.bsvExchangeRate, nil
	}

	var exchangeRateResponse dto.BSVExchangeRateResponse
	req := woc.httpClient.R()

	res, err := req.
		SetResult(&exchangeRateResponse).
		Get(fmt.Sprintf("%s/exchangerate", woc.url))
	if err != nil {
		return defs.BSVExchangeRate{}, fmt.Errorf("failed to fetch exchange rate: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return defs.BSVExchangeRate{}, fmt.Errorf("failed to retrieve successful response from WOC. Actual status: %d", res.StatusCode())
	}

	if exchangeRateResponse.Currency != string(defs.USD) {
		return defs.BSVExchangeRate{}, fmt.Errorf("unsupported currency returned from Whats On Chain")
	}

	woc.bsvExchangeRate = defs.BSVExchangeRate{
		Timestamp: time.Now(),
		Base:      defs.USD,
		Rate:      exchangeRateResponse.Rate,
	}

	return woc.bsvExchangeRate, nil
}

func (woc *WhatsOnChain) FindChainTipHeader(ctx context.Context) (*wdk.ChainBlockHeader, error) {
	var blocks []dto.BlockHeader
	url := fmt.Sprintf("%s/block/headers?limit=1", woc.url)
	res, err := woc.
		httpClient.
		R().
		SetContext(ctx).
		SetResult(&blocks).
		AddRetryCondition(func(res *resty.Response, err error) bool {
			return res.StatusCode() == http.StatusTooManyRequests
		}).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("error while fetching block headers from WhatsOnChain (URL: %s): %w", url, err)
	}

	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected response from WhatsOnChain (URL: %s): status code %d", url, res.StatusCode())
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no block headers returned from WhatsOnChain (URL: %s); at least one expected", url)
	}

	first := blocks[0]
	header, err := first.ConvertToChainBlockHeader()
	if err != nil {
		return nil, fmt.Errorf("error while converting the response from WhatsOnChain (URL: %s) to the *wdk.ChainBlockHeader: %w", url, err)
	}

	return header, nil
}

// PostBEEF attempts to post beef with given txIDs
func (woc *WhatsOnChain) PostBEEF(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txids provided")
	}
	if beef == nil {
		return nil, fmt.Errorf("beef is required to post transactions")
	}

	rawTxs, err := txutils.ExtractRawTransactions(beef, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw transactions: %w", err)
	}

	txResults := make([]wdk.PostedTxID, 0, len(txIDs))

	for i, txid := range txIDs {
		if i != 0 {
			if err := waitOrCancel(ctx, woc.broadcastDelay, txid); err != nil {
				return nil, err
			}
		}
		result := woc.processSingleTx(ctx, txid, rawTxs[i])
		txResults = append(txResults, result)
	}

	return &wdk.PostedBEEF{TxIDResults: txResults}, nil
}
