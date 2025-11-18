package testabilities

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/go-softwarelab/common/pkg/slogx"
)

// CertifierServerBuilder is a builder interface for configuring a test certifier server.
//
// Usage examples:
//
//   // Basic usage with default auth middleware:
//   server := given.
//       CertifierServer().
//       WithCertifierWallet(certifierWallet).
//       Started()
//
//   // With custom auth middleware options:
//   server := given.
//       CertifierServer().
//       WithCertifierWallet(certifierWallet).
//       WithAuthMiddlewareOpts(middleware.WithAuthAllowUnauthenticated()).
//       Started()
//
//   // With pre-configured auth middleware:
//   authMiddleware := middleware.NewAuth(wallet, middleware.WithAuthAllowUnauthenticated())
//   server := given.
//       CertifierServer().
//       WithCertifierWallet(certifierWallet).
//       WithAuthMiddleware(authMiddleware).
//       Started()
//
//   // With custom certificate handler:
//   server := given.
//       CertifierServer().
//       WithCertifierWallet(certifierWallet).
//       WithSignCertHandler(myCustomHandler).
//       Started()
type CertifierServerBuilder interface {
	WithCertifierWallet(wallet sdk.Interface) CertifierServerBuilder
	WithSignCertHandler(handler http.HandlerFunc) CertifierServerBuilder
	WithLogger(logger *slog.Logger) CertifierServerBuilder
	WithAuthMiddleware(authMiddleware *middleware.AuthMiddlewareFactory) CertifierServerBuilder
	WithAuthMiddlewareOpts(opts ...func(*middleware.AuthMiddlewareConfig)) CertifierServerBuilder
	Started() CertifierServerFixture
}

// CertifierServerFixture represents a running test certifier server
type CertifierServerFixture interface {
	URL() string
}

type certifierServerBuilder struct {
	testing.TB
	serverWallet       sdk.Interface
	logger             *slog.Logger
	signCertHandler    http.HandlerFunc
	authMiddleware     *middleware.AuthMiddlewareFactory
	authMiddlewareOpts []func(*middleware.AuthMiddlewareConfig)
}

type certifierServerFixture struct {
	testing.TB
	server *httptest.Server
}

func (b *certifierServerBuilder) WithCertifierWallet(wallet sdk.Interface) CertifierServerBuilder {
	b.serverWallet = wallet
	return b
}

func (b *certifierServerBuilder) WithSignCertHandler(handler http.HandlerFunc) CertifierServerBuilder {
	b.signCertHandler = handler
	return b
}

func (b *certifierServerBuilder) WithLogger(logger *slog.Logger) CertifierServerBuilder {
	b.logger = logger
	return b
}

// WithAuthMiddleware allows you to provide a pre-configured auth middleware factory.
// This is useful when you need full control over middleware configuration.
// If not provided, a default middleware will be created with the wallet and any options from WithAuthMiddlewareOpts.
func (b *certifierServerBuilder) WithAuthMiddleware(authMiddleware *middleware.AuthMiddlewareFactory) CertifierServerBuilder {
	b.authMiddleware = authMiddleware
	return b
}

// WithAuthMiddlewareOpts allows you to configure the default auth middleware with custom options.
func (b *certifierServerBuilder) WithAuthMiddlewareOpts(opts ...func(*middleware.AuthMiddlewareConfig)) CertifierServerBuilder {
	b.authMiddlewareOpts = append(b.authMiddlewareOpts, opts...)
	return b
}

func (b *certifierServerBuilder) Started() CertifierServerFixture {
	b.Helper()

	if b.serverWallet == nil {
		b.Fatal("certifier wallet must be provided via WithCertifierWallet()")
	}

	// Use provided handler or default mock handler
	signCertHandler := b.signCertHandler
	if signCertHandler == nil {
		signCertHandler = b.defaultSignCertificateHandler()
	}

	// Create the HTTP handler with auth middleware
	handler := b.createHandler(signCertHandler)

	// Start test server
	server := httptest.NewServer(handler)

	b.Cleanup(func() {
		server.Close()
	})

	return &certifierServerFixture{
		TB:     b.TB,
		server: server,
	}
}

// URL returns the base URL of the test server
func (f *certifierServerFixture) URL() string {
	return f.server.URL
}

// createHandler creates the HTTP handler with auth middleware
func (b *certifierServerBuilder) createHandler(signCertHandler http.HandlerFunc) http.Handler {
	mux := http.NewServeMux()

	// Register the certificate signing endpoint
	mux.HandleFunc("/signCertificate", signCertHandler)

	// Use provided auth middleware or create default one
	var authMiddleware *middleware.AuthMiddlewareFactory
	if b.authMiddleware != nil {
		authMiddleware = b.authMiddleware
	} else {
		// Build default auth middleware with configured options
		opts := []func(*middleware.AuthMiddlewareConfig){
			middleware.WithAuthLogger(slogx.NewTestLogger(b)),
		}
		opts = append(opts, b.authMiddlewareOpts...)
		authMiddleware = middleware.NewAuth(b.serverWallet, opts...)
	}

	return authMiddleware.HTTPHandler(mux)
}

// defaultSignCertificateHandler provides a default mock implementation
func (b *certifierServerBuilder) defaultSignCertificateHandler() http.HandlerFunc {
	// TODO: proper implementation of the mock
	logger := b.logger
	
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Received sign certificate request", slog.String("path", r.URL.Path))

		// Parse the request
		var req wallet.ProtocolIssuanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("Failed to decode request", slog.Any("error", err))
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Mock response with a certificate
		response := wallet.ProtocolIssuanceResponse{
			ServerNonce: "mock-server-nonce-12345678901234567890123456789012",
			Certificate: &wallet.Certificate{
				Type:               req.Type,
				SerialNumber:       "mock-serial-123",
				Subject:            "mock-subject-pubkey",
				Certifier:          "mock-certifier-pubkey",
				RevocationOutpoint: "mock-txid:0",
				Fields:             req.Fields,
				Signature:          "mock-signature-hex",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode response", slog.Any("error", err))
		}
	}
}
