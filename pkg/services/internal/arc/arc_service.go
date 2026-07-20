package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/go-softwarelab/common/pkg/types"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Custom ARC defined http status codes
const (
	StatusNotExtendedFormat             = 460
	StatusFeeTooLow                     = 465
	StatusCumulativeFeeValidationFailed = 473
)

type Config = defs.ARC

const ServiceName = defs.ArcServiceName

type Service struct {
	name             string
	logger           *slog.Logger
	httpClient       *resty.Client
	config           Config
	broadcastURL     string
	queryTxURL       string
	broadcastHeaders httpx.Headers
}

// New creates a new arc service registered under the default ServiceName.
func New(logger *slog.Logger, httpClient *resty.Client, config Config) *Service {
	return NewNamed(ServiceName, logger, httpClient, config)
}

// NewNamed creates a new arc service reported under the given name in results
// and history notes. It allows multiple ARC instances (e.g. TAAL and
// GorillaPool) to be distinguished while sharing the same client code.
func NewNamed(name string, logger *slog.Logger, httpClient *resty.Client, config Config) *Service {
	logger = logging.Child(logger, "arc")

	headers := httpx.NewHeaders().
		AcceptJSON().
		ContentTypeJSON().
		UserAgent().Value("go-wallet-toolbox").
		Authorization().IfNotEmpty(config.Token).
		Set("XDeployment-ID").OrDefault(config.DeploymentID, "go-wallet-toolbox#"+time.Now().Format("20060102150405"))

	httpClient = httpClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger))

	service := &Service{
		name:       name,
		logger:     logger,
		httpClient: httpClient,
		config:     config,

		broadcastURL: config.URL + "/v1/tx",
		broadcastHeaders: httpx.NewHeaders().
			Set("X-CallbackUrl").IfNotEmpty(config.CallbackURL).
			Set("X-CallbackToken").IfNotEmpty(config.CallbackToken).
			Set("X-WaitFor").IfNotEmpty(config.WaitFor),

		queryTxURL: config.URL + "/v1/tx/{txID}",
	}

	return service
}

// Name returns the name under which this ARC instance is reported in results and history notes.
func (s *Service) Name() string {
	return s.name
}

// PostEF attempts to post EF with given txIDs
func (s *Service) PostEF(ctx context.Context, efHex, txID string) (_ *wdk.PostedTxID, err error) {
	ctx, span := tracing.StartTracing(ctx, "Services-PostEF", attribute.String("service", "arc"))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	response, err := s.broadcast(ctx, efHex)
	if err != nil {
		result := wdk.PostedTxID{
			TxID:   txID,
			Result: wdk.PostedTxIDResultError,
			Error:  fmt.Errorf("failed to broadcast tx: %w", err),
		}
		s.withBroadcastNote(&result, efHex, []string{txID})
		return &result, nil // nil error - error info is in the result
	}

	// if ARC returned info for this tx, use it directly
	var namedResult *internal.NamedResult[*TXInfo]
	if response != nil && response.TxID == txID {
		namedResult = internal.NewNamedResult(txID, types.SuccessResult(response))
	} else {
		// else query ARC for the tx status using getTransactionData
		namedResult = s.getTransactionData(ctx, txID)
	}

	result := toResultForPostTxID(namedResult)
	s.withBroadcastNote(&result, efHex, []string{txID})

	return &result, nil
}

func (s *Service) withBroadcastNote(result *wdk.PostedTxID, efHex string, txIDs []string) {
	switch result.Result {
	case wdk.PostedTxIDResultSuccess, wdk.PostedTxIDResultAlreadyKnown:
		result.Notes = history.NewBuilder().PostBeefSuccess(s.name, txIDs).Note().AsList()
	case wdk.PostedTxIDResultError, wdk.PostedTxIDResultDoubleSpend, wdk.PostedTxIDResultMissingInputs:
		fallthrough
	default:
		msg := fmt.Sprintf("broadcasted ef with problematic result %s", result.Result)
		if result.Error != nil {
			msg += fmt.Sprintf(" and error: %v", result.Error)
		}
		result.Notes = history.NewBuilder().PostBeefError(s.name, history.Hex(efHex), txIDs, msg).Note().AsList()
	}
}

func toResultForPostTxID(it *internal.NamedResult[*TXInfo]) wdk.PostedTxID {
	if it.IsError() {
		return wdk.PostedTxID{
			TxID:   it.Name(),
			Result: wdk.PostedTxIDResultError,
			Error:  it.MustGetError(),
		}
	}
	info := it.MustGetValue()

	doubleSpend := info.TXStatus == DoubleSpendAttempted
	result := wdk.PostedTxID{
		Result:       to.IfThen(doubleSpend, wdk.PostedTxIDResultError).ElseThen(wdk.PostedTxIDResultSuccess),
		TxID:         it.Name(),
		DoubleSpend:  doubleSpend,
		BlockHash:    info.BlockHash,
		BlockHeight:  info.BlockHeight,
		CompetingTxs: info.CompetingTxs,
	}

	if is.NotBlankString(info.MerklePath) {
		merklePath, err := transaction.NewMerklePathFromHex(info.MerklePath)
		if err != nil {
			result.Error = err
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

func (s *Service) getTransactionData(ctx context.Context, txID string) *internal.NamedResult[*TXInfo] {
	txInfo, err := s.queryTransaction(ctx, txID)
	if err != nil {
		return internal.NewNamedResult(txID, types.FailureResult[*TXInfo](fmt.Errorf("arc query tx %s failed: %w", txID, err)))
	}

	if txInfo == nil {
		return internal.NewNamedResult(txID, types.FailureResult[*TXInfo](fmt.Errorf("not found tx %s in arc", txID)))
	}

	if txInfo.TxID != txID {
		return internal.NewNamedResult(txID, types.FailureResult[*TXInfo](fmt.Errorf("got response for tx %s while querying for %s", txInfo.TxID, txID)))
	}

	return internal.NewNamedResult(txID, types.SuccessResult(txInfo))
}
