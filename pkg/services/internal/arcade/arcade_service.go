// Package arcade provides a client for the Arcade transaction processor
// (https://github.com/bsv-blockchain/arcade).
//
// Preferred lifecycle:
//  1. Broadcast via POST /tx
//  2. Receive status + merkle proofs on the SSE /events stream (no polling)
//
// Fallback when SSE is unavailable: MerklePath polls GET /tx/{txID} so monitor
// status sync / check_for_proofs can still mark txs mined. That pull path is
// deliberately secondary to the event stream.
//
// Note: Arcade is NOT classic-ARC compatible - endpoints have no /v1 prefix
// and the broadcast body is binary Extended Format bytes. Do not point the
// classic ARC client at the Arcade host for proofs; use this package.
package arcade

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/httpx"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
)

// Config is the configuration of the Arcade service.
type Config = defs.Arcade

// ServiceName is the name under which the Arcade service is reported in results and history notes.
const ServiceName = defs.ArcadeServiceName

// defaultRetryAfter is used when Arcade responds 503 without a parsable Retry-After header.
const defaultRetryAfter = 5 * time.Second

// BackpressureError is returned when Arcade responds 503 (backpressure).
// RetryAfter carries the parsed Retry-After header (or a 5s default).
type BackpressureError struct {
	RetryAfter time.Duration
}

// Error returns the error string; it's the implementation of the error interface.
func (e *BackpressureError) Error() string {
	return fmt.Sprintf("arcade is applying backpressure, retry after %s", e.RetryAfter)
}

// AuthError is returned when Arcade refuses the request's credentials: 401 for a
// missing-but-required, malformed, unknown, revoked or expired API key, 403 when
// the key is not allowed to make the call. It is a service-level failure, never
// a verdict on the transaction, so the broadcast router fails over to the next
// broadcaster instead of reporting the transaction as rejected.
type AuthError struct {
	StatusCode int
	// APIKeyConfigured reports whether the request carried an API key.
	APIKeyConfigured bool
	Err              *APIError
}

// Error returns the error string; it's the implementation of the error interface.
func (e *AuthError) Error() string {
	hint := "check the configured arcade api_key"
	if !e.APIKeyConfigured {
		hint = "this arcade instance may require an api_key"
	}
	return fmt.Sprintf("arcade refused the request credentials (http status %d, %s): %s", e.StatusCode, hint, e.Err.Error())
}

// isAuthStatus reports whether an HTTP status means Arcade refused the credentials.
func isAuthStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

func (s *Service) authError(statusCode int, apiErr *APIError) *AuthError {
	return &AuthError{StatusCode: statusCode, APIKeyConfigured: s.config.APIKey != "", Err: apiErr}
}

// APIError represents an error returned by the Arcade API when status code is 4xx.
type APIError struct {
	Err string `json:"error"`
}

// Error returns the error string; it's the implementation of the error interface.
func (a *APIError) Error() string {
	if a.IsEmpty() {
		return "arcade error: empty (or not in json) response"
	}
	return "arcade error: " + a.Err
}

// IsEmpty checks if the error is empty, indicating that we could not parse the error response.
func (a *APIError) IsEmpty() bool {
	return a == nil || a.Err == ""
}

// Service is the Arcade client.
type Service struct {
	logger           *slog.Logger
	httpClient       *resty.Client
	sseClient        *http.Client
	config           Config
	broadcastURL     string
	queryTxURL       string
	healthURL        string
	eventsURL        string
	broadcastHeaders httpx.Headers
	// sseReadWatchdogTimeout is the read-liveness watchdog of the SSE stream:
	// when no line is read for this long the connection is dropped and redialed.
	// Defaults to readWatchdogTimeout; overridable in tests.
	sseReadWatchdogTimeout time.Duration
	// chainTipHeight, when wired via SetChainTipHeight, provides the tip used
	// to compute confirmation depth in GetStatusForTxIDs. Atomic: it is set
	// after service construction (the height source is built later in the
	// services wiring) and read from request goroutines.
	chainTipHeight atomic.Pointer[ChainTipHeightFunc]
}

// New creates a new arcade service.
func New(logger *slog.Logger, httpClient *resty.Client, config Config) *Service {
	logger = logging.Child(logger, "arcade")

	headers := httpx.NewHeaders().
		AcceptJSON().
		AuthorizationBearer().IfNotEmpty(config.APIKey).
		UserAgent().Value("go-wallet-toolbox")

	httpClient = httpClient.
		SetHeaders(headers).
		SetLogger(logging.RestyAdapter(logger)).
		SetDebug(logging.IsDebug(logger)).
		// debug request logs dump every header: never let the API key reach the logs.
		// RequestLog.Header is a copy, so this does not alter the request sent.
		OnRequestLog(redactAuthorization)

	broadcastHeaders := httpx.NewHeaders().
		ContentType().Value("application/octet-stream").
		Set("X-CallbackToken").IfNotEmpty(config.CallbackToken).
		Set("X-CallbackUrl").IfNotEmpty(config.CallbackURL)
	if config.FullStatusUpdates {
		broadcastHeaders.Set("X-FullStatusUpdates").Value("true")
	}

	return &Service{
		logger:     logger,
		httpClient: httpClient,
		sseClient:  newSSEClient(),
		config:     config,

		broadcastURL:           config.URL + "/tx",
		queryTxURL:             config.URL + "/tx/{txID}",
		healthURL:              config.URL + "/health",
		eventsURL:              eventsURL(config),
		broadcastHeaders:       broadcastHeaders,
		sseReadWatchdogTimeout: readWatchdogTimeout,
	}
}

// redactAuthorization masks the Authorization header in resty debug request logs.
func redactAuthorization(rl *resty.RequestLog) error {
	if rl.Header.Get("Authorization") != "" {
		rl.Header.Set("Authorization", "[REDACTED]")
	}
	return nil
}

// newSSEClient creates an HTTP client for the long-lived SSE stream:
// no overall timeout (cancellation is driven by context), but sane
// dial/TLS/response-header timeouts on the transport.
func newSSEClient() *http.Client {
	return &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

// eventsURL builds the full SSE endpoint URL, falling back to the base URL
// when EventsURL is not configured and scoping the stream with the callback token.
func eventsURL(config Config) string {
	base := config.EventsURL
	if base == "" {
		base = config.URL
	}
	endpoint := base + "/events"
	if config.CallbackToken != "" {
		endpoint += "?callbackToken=" + url.QueryEscape(config.CallbackToken)
	}
	return endpoint
}

// Healthy probes GET /health and reports whether Arcade responded with success.
func (s *Service) Healthy(ctx context.Context) bool {
	ctx, span := tracing.StartTracing(ctx, "Services-Healthy", attribute.String("service", "arcade"))
	var err error
	defer func() {
		tracing.EndTracing(span, err)
	}()

	response, err := s.httpClient.R().SetContext(ctx).Get(s.healthURL)
	if err != nil {
		s.logger.WarnContext(ctx, "arcade health probe failed", slog.String("error", err.Error()))
		return false
	}
	if !response.IsSuccess() {
		err = fmt.Errorf("arcade health probe returned http status [%d %s]", response.StatusCode(), response.Status())
		s.logger.WarnContext(ctx, "arcade health probe failed", slog.String("error", err.Error()))
		return false
	}
	return true
}
