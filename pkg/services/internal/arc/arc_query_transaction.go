package arc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
)

func (s *Service) queryTransaction(ctx context.Context, txID string) (_ *TXInfo, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-queryTransaction", attribute.String("service", "arc"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result := &TXInfo{}
	arcErr := &APIError{}
	req := s.httpClient.R().
		SetContext(ctx).
		SetResult(result).
		SetError(arcErr).
		SetPathParam("txID", txID)

	response, err := req.Get(s.queryTxURL)
	if err != nil {
		var netError net.Error
		if errors.As(err, &netError) {
			return nil, fmt.Errorf("arc is unreachable: %w", netError)
		}
		return nil, fmt.Errorf("failed to send request to arc: %w", err)
	}

	switch response.StatusCode() {
	case http.StatusOK:
		return result, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("arc returned unauthorized: %w", arcErr)
	case http.StatusNotFound:
		if !arcErr.IsEmpty() {
			// ARC returns 404 when transaction is not found
			return nil, nil // By convention, nil is returned when transaction is not found
		}
		return nil, fmt.Errorf("arc %s is unreachable", s.queryTxURL)
	case http.StatusConflict:
		return nil, fmt.Errorf("arc respond with error: %w", arcErr)
	default:
		return nil, fmt.Errorf("arc returns unexpected http status [%d %s]: %w", response.StatusCode(), response.Status(), arcErr)
	}
}
