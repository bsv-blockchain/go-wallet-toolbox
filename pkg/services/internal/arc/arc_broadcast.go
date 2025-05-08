package arc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/go-softwarelab/common/pkg/is"
)

type broadcastRequestBody struct {
	// Even though the name suggests that it is a raw transaction,
	// it is actually a hex encoded transaction
	// and can be in Raw, Extended Format or BEEF format.
	RawTx string `json:"rawTx"`
}

func (s *Service) broadcast(ctx context.Context, txHex string) (*TXInfo, error) {
	result := &TXInfo{}
	arcErr := &APIError{}

	req := s.httpClient.R().
		SetContext(ctx).
		SetHeaders(s.broadcastHeaders).
		SetBody(broadcastRequestBody{
			RawTx: txHex,
		}).
		SetResult(result).
		SetError(arcErr)

	response, err := req.Post(s.broadcastURL)
	if err != nil {
		var netError net.Error
		if errors.As(err, &netError) {
			return nil, fmt.Errorf("arc is unreachable: %w", netError)
		}
		return nil, fmt.Errorf("failed to send request to arc: %w", err)
	}

	if result != nil && is.BlankString(result.TxID) {
		result = nil
	}

	switch response.StatusCode() {
	case http.StatusOK:
		if result != nil && is.BlankString(result.TxID) {
			return nil, nil
		}
		return result, nil
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return nil, fmt.Errorf("arc returned unauthorized: %w", arcErr)
	case StatusNotExtendedFormat:
		return nil, fmt.Errorf("arc expects transaction in extended format: %w", arcErr)
	case StatusFeeTooLow, StatusCumulativeFeeValidationFailed:
		return nil, fmt.Errorf("arc rejected transaction because of wrong fee: %w", arcErr)
	default:
		return nil, fmt.Errorf("arc cannot process provided transaction: %w", arcErr)
	}
}
