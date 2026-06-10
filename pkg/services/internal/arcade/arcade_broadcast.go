package arcade

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/to"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// PostEF broadcasts a single transaction: it decodes efHex to binary and POSTs it
// to /tx as application/octet-stream.
//
// Transport errors (network, 5xx except 503) result in (nil, err).
// An Arcade-level rejection (400) results in a result with Error set and err == nil.
// A 503 results in (nil, *BackpressureError) with the parsed Retry-After header
// (seconds int or HTTP date; default 5s when absent/unparseable).
// An efHex decode failure results in a wdk.PostedTxIDResultError result with err == nil.
func (s *Service) PostEF(ctx context.Context, efHex, txID string) (_ *wdk.PostedTxID, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-PostEF", attribute.String("service", "arcade"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	efBytes, decodeErr := hex.DecodeString(efHex)
	if decodeErr == nil && len(efBytes) == 0 {
		decodeErr = errors.New("empty transaction hex")
	}
	if decodeErr != nil {
		result := wdk.PostedTxID{
			TxID:   txID,
			Result: wdk.PostedTxIDResultError,
			Error:  fmt.Errorf("failed to decode ef hex of tx %s: %w", txID, decodeErr),
		}
		withBroadcastNote(&result, efHex, []string{txID})
		return &result, nil
	}

	info, rejection, err := s.broadcast(ctx, efBytes)
	if err != nil {
		return nil, err
	}

	if rejection != nil {
		result := wdk.PostedTxID{
			TxID:   txID,
			Result: wdk.PostedTxIDResultError,
			Error:  fmt.Errorf("arcade rejected tx %s: %w", txID, rejection),
		}
		withBroadcastNote(&result, efHex, []string{txID})
		return &result, nil
	}

	result := toResultForPostTxID(txID, info)
	withBroadcastNote(&result, efHex, []string{txID})

	return &result, nil
}

// broadcast POSTs the binary EF bytes to Arcade. It returns either the transaction
// info (success), an APIError (Arcade-level 4xx rejection) or an error
// (transport failure, unexpected 5xx, or *BackpressureError on 503).
func (s *Service) broadcast(ctx context.Context, efBytes []byte) (_ *TXInfo, _ *APIError, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-Broadcast", attribute.String("service", "arcade"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	result := &TXInfo{}
	apiErr := &APIError{}

	req := s.httpClient.R().
		SetContext(ctx).
		SetHeaders(s.broadcastHeaders).
		SetBody(efBytes).
		SetResult(result).
		SetError(apiErr)

	response, err := req.Post(s.broadcastURL)
	if err != nil {
		var netError net.Error
		if errors.As(err, &netError) {
			return nil, nil, fmt.Errorf("arcade is unreachable: %w", netError)
		}
		return nil, nil, fmt.Errorf("failed to send request to arcade: %w", err)
	}

	switch {
	case response.IsSuccess():
		if is.BlankString(result.TxID) {
			return nil, nil, fmt.Errorf("arcade returned success [%d %s] without transaction status", response.StatusCode(), response.Status())
		}
		return result, nil, nil
	case response.StatusCode() == http.StatusServiceUnavailable:
		return nil, nil, &BackpressureError{RetryAfter: parseRetryAfter(response.Header().Get("Retry-After"))}
	case response.StatusCode() >= http.StatusInternalServerError:
		return nil, nil, fmt.Errorf("arcade returned http status [%d %s]: %w", response.StatusCode(), response.Status(), apiErr)
	default:
		return nil, apiErr, nil
	}
}

// parseRetryAfter parses a Retry-After header value: either an integer number of
// seconds or an HTTP date. It defaults to 5s when the header is absent or unparseable.
func parseRetryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return defaultRetryAfter
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds < 0 {
			return defaultRetryAfter
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(header); err == nil {
		if until := time.Until(date); until > 0 {
			return until
		}
		// an HTTP date in the past (or "now") usually means clock skew - fall back
		// to the default instead of retrying immediately with 0.
		return defaultRetryAfter
	}
	return defaultRetryAfter
}

func toResultForPostTxID(txID string, info *TXInfo) wdk.PostedTxID {
	// DoubleSpend is set only when Arcade actually reports competing transactions.
	// A bare REJECTED (e.g. policy/fee rejection without any conflict) is NOT
	// double-spend evidence: downstream confirm-double-spends processing treats
	// DoubleSpend=true as evidence and would corrupt wallet UTXO state on a false
	// verdict (see the false-double-spend protection introduced in commit 6addd9e).
	doubleSpend := len(info.CompetingTxs) > 0
	failed := doubleSpend || info.TxStatus == StatusRejected
	result := wdk.PostedTxID{
		Result:       to.IfThen(failed, wdk.PostedTxIDResultError).ElseThen(wdk.PostedTxIDResultSuccess),
		TxID:         txID,
		DoubleSpend:  doubleSpend,
		BlockHash:    info.BlockHash,
		BlockHeight:  info.BlockHeight,
		CompetingTxs: info.CompetingTxs,
	}

	if failed {
		result.Error = fmt.Errorf("arcade reported tx %s with status %s and competing txs %v", txID, info.TxStatus, info.CompetingTxs)
	}

	if is.NotBlankString(info.MerklePath) {
		merklePath, err := transaction.NewMerklePathFromHex(info.MerklePath)
		if err != nil {
			// keep an already-set error (e.g. the double-spend message) and join the parse failure
			if result.Error == nil {
				result.Error = err
			} else {
				result.Error = errors.Join(result.Error, err)
			}
			result.Result = wdk.PostedTxIDResultError
		} else {
			result.MerklePath = merklePath
		}
	}

	dataBytes, err := json.Marshal(info)
	if err != nil {
		// fallback to string representation in very unlikely case of json marshal error.
		result.Data = fmt.Sprintf("%+v", info)
	} else {
		result.Data = string(dataBytes)
	}

	return result
}

func withBroadcastNote(result *wdk.PostedTxID, efHex string, txIDs []string) {
	switch result.Result {
	case wdk.PostedTxIDResultSuccess, wdk.PostedTxIDResultAlreadyKnown:
		result.Notes = history.NewBuilder().PostBeefSuccess(ServiceName, txIDs).Note().AsList()
	case wdk.PostedTxIDResultError, wdk.PostedTxIDResultDoubleSpend, wdk.PostedTxIDResultMissingInputs:
		fallthrough
	default:
		msg := fmt.Sprintf("broadcasted ef with problematic result %s", result.Result)
		if result.Error != nil {
			msg += fmt.Sprintf(" and error: %v", result.Error)
		}
		result.Notes = history.NewBuilder().PostBeefError(ServiceName, history.Hex(efHex), txIDs, msg).Note().AsList()
	}
}
