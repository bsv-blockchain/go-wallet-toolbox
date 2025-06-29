package bitails

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/utils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
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
		AddRetryCondition(utils.RetryOnTooManyRequestsStatus).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logger.Enabled(context.Background(), slog.LevelDebug))

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

	rawTxs, err := extractRawTransactions(beef, txIDs)
	if err != nil {
		return nil, err
	}

	var results []wdk.PostedTxID

	for i, txID := range txIDs {
		raw := rawTxs[i]
		broadcastResult, err := b.broadcast(ctx, raw)

		switch {
		case err != nil:
			results = append(results, wdk.PostedTxID{
				TxID:   txID,
				Result: wdk.PostedTxIDResultError,
				Error:  fmt.Errorf("failed to broadcast tx %s: %w", txID, err),
				Notes:  convertNotes([]string{err.Error()}),
			})

		case broadcastResult == nil:
			results = append(results, wdk.PostedTxID{
				TxID:   txID,
				Result: wdk.PostedTxIDResultError,
				Error:  fmt.Errorf("nil broadcast result for txid %s", txID),
				Notes:  convertNotes([]string{"broadcast returned nil"}),
			})

		default:
			results = append(results, *broadcastResult)
		}
	}

	return &wdk.PostedBEEF{TxIDResults: results}, nil
}
