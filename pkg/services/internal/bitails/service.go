package bitails

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"log/slog"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
)

type Bitails struct {
	httpClient *resty.Client
	url        string
	apiKey     string
	logger     *slog.Logger
}

func New(httpClient *resty.Client, logger *slog.Logger, network defs.BSVNetwork, config defs.Bitails) *Bitails {
	logger = logging.Child(logger, "Bitails").With(slog.String("network", string(network)))

	headers := httpx.NewHeaders().
		AcceptJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.APIKey)

	client := httpClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger))

	baseURL := ProductionURL
	if strings.ToLower(string(network)) == "test" {
		baseURL = TestnetURL
	}

	return &Bitails{
		httpClient: client,
		apiKey:     config.APIKey,
		url:        baseURL,
		logger:     logger,
	}
}

// PostBEEF sends the given beef with selected txIDs to Bitails for broadcasting.
func (b *Bitails) PostBEEF(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error) {
	if beef == nil {
		return nil, fmt.Errorf("beef is required to post transactions")
	}
	if len(txIDs) == 0 {
		return nil, fmt.Errorf("no txids provided")
	}

	rawTxs, err := txutils.ExtractRawTransactions(beef, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw transactions: %w", err)
	}

	results := make([]wdk.PostedTxID, 0, len(txIDs))

	for i, txID := range txIDs {
		raw := rawTxs[i]
		broadcastResult, err := b.broadcast(ctx, raw)
		if err != nil {
			err = fmt.Errorf("problem broadcasting the transaction %s: %w", txID, err)

			failedResult := wdk.PostedTxID{
				TxID:   txID,
				Result: wdk.PostedTxIDResultError,
				Error:  err,
				Notes:  history.New().PostBeefError(ServiceName, raw, []string{txID}, err.Error()).Note().AsList(),
			}

			if broadcastResult != nil {
				failedResult.Result = broadcastResult.Result
				failedResult.AlreadyKnown = broadcastResult.AlreadyKnown
				failedResult.DoubleSpend = broadcastResult.DoubleSpend
			}

			results = append(results, failedResult)
			continue
		}

		broadcastResult.Notes = history.New().PostBeefSuccess(ServiceName, raw, []string{txID}).Note().AsList()
		results = append(results, *broadcastResult)
	}

	return &wdk.PostedBEEF{TxIDResults: results}, nil
}
