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
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
)

// bsvExchangeRateResponse is the response from WhatsOnChain for bsv exchange range
type bsvExchangeRateResponse struct {
	Time     int     `json:"time"`
	Rate     float64 `json:"rate"`
	Currency string  `json:"currency"`
}

const ServiceName = "WhatsOnChain"

type WhatsOnChain struct {
	httpClient *resty.Client
	url        string
	apiKey     string
	logger     *slog.Logger

	bsvExchangeRate   defs.BSVExchangeRate // TODO: possibly handle by some caching structure/redis
	bsvUpdateInterval time.Duration
}

func New(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.WhatsOnChain) *WhatsOnChain {
	logger = logging.Child(logger, "WoC").With(slog.String("network", string(network)))

	if httpClient == nil {
		panic("httpClient is required")
	}

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	client := httpClient.Clone().
		SetRetryCount(Retries).
		SetRetryWaitTime(RetriesWaitTime).
		SetRetryMaxWaitTime(Retries * RetriesWaitTime).
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logger.Enabled(nil, slog.LevelDebug))

	return &WhatsOnChain{
		httpClient:        client,
		apiKey:            config.APIKey,
		url:               fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s", network),
		logger:            logger,
		bsvExchangeRate:   config.BSVExchangeRate,
		bsvUpdateInterval: to.IfThen(config.BSVUpdateInterval != nil, *config.BSVUpdateInterval).ElseThen(defs.DefaultBSVExchangeUpdateInterval),
	}
}

func (woc *WhatsOnChain) RawTx(ctx context.Context, txID string) (*wdk.RawTxResult, error) {
	req := woc.httpClient.
		R().
		SetContext(ctx).
		AddRetryCondition(func(res *resty.Response, err error) bool {
			return res.StatusCode() == http.StatusTooManyRequests
		})
	req.SetHeader("Cache-Control", "no-cache")

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

	var exchangeRateResponse bsvExchangeRateResponse
	req := woc.httpClient.
		R().
		AddRetryCondition(func(res *resty.Response, err error) bool {
			return res.StatusCode() == http.StatusTooManyRequests
		})

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
