package arcade

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// QueryTx returns the lifecycle state of a transaction from GET /tx/{txID}.
// It returns (nil, wdk.ErrNotFoundError) when the transaction was never
// submitted to this Arcade instance (404).
func (s *Service) QueryTx(ctx context.Context, txID string) (_ *TXInfo, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-QueryTx", attribute.String("service", "arcade"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result := &TXInfo{}
	apiErr := &APIError{}
	req := s.httpClient.R().
		SetContext(ctx).
		SetResult(result).
		SetError(apiErr).
		SetPathParam("txID", txID)

	response, err := req.Get(s.queryTxURL)
	if err != nil {
		var netError net.Error
		if errors.As(err, &netError) {
			return nil, fmt.Errorf("arcade is unreachable: %w", netError)
		}
		return nil, fmt.Errorf("failed to send request to arcade: %w", err)
	}

	switch response.StatusCode() {
	case http.StatusOK:
		return result, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("tx %s is not known to arcade: %w", txID, wdk.ErrNotFoundError)
	default:
		return nil, fmt.Errorf("arcade returned unexpected http status [%d %s]: %w", response.StatusCode(), response.Status(), apiErr)
	}
}
