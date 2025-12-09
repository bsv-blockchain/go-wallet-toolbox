package testabilities

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	"github.com/bsv-blockchain/go-sdk/auth/brc104"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/actions"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/utils"
	"github.com/go-softwarelab/common/pkg/slogx"
	"github.com/stretchr/testify/require"
)

// CertifierServerBuilder is a builder interface for configuring a test certifier server.
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
	logger := b.logger

	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("received sign certificate request", slog.String("path", r.URL.Path))

		// Parse the request
		var req actions.ProtocolIssuanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("failed to decode request", slog.Any("error", err))
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		clientPubKey, err := ec.PublicKeyFromString(r.Header.Get(brc104.HeaderIdentityKey))
		if err != nil {
			logger.Error("failed to create client public key", slog.Any("error", err))
			http.Error(w, "failed to create client public key", http.StatusBadRequest)
			return
		}
		serverNonce, err := utils.CreateNonce(b.Context(), b.serverWallet, randomizer.New(), clientPubKey, fixtures.DefaultOriginator)
		if err != nil {
			logger.Error("failed to create server nonce", slog.Any("error", err))
			http.Error(w, "failed to create server nonce", http.StatusBadRequest)
			return
		}
		hmac, err := b.serverWallet.CreateHMAC(b.Context(), sdk.CreateHMACArgs{
			EncryptionArgs: sdk.EncryptionArgs{
				ProtocolID: sdk.Protocol{
					SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
					Protocol:      "certificate issuance",
				},
				KeyID: string(serverNonce) + req.Nonce,
				Counterparty: sdk.Counterparty{
					Type:         sdk.CounterpartyTypeOther,
					Counterparty: clientPubKey,
				},
			},
			Data: bytes.Join([][]byte{[]byte(req.Nonce), serverNonce}, []byte{}),
		}, "")
		if err != nil {
			logger.Error("failed to create server hmac", slog.Any("error", err))
			http.Error(w, "failed to create server hmac", http.StatusBadRequest)
			return
		}

		certifierKey, err := b.serverWallet.GetPublicKey(b.Context(), sdk.GetPublicKeyArgs{IdentityKey: true}, fixtures.DefaultOriginator)
		if err != nil {
			logger.Error("failed to get server wallet public key", slog.Any("error", err))
			http.Error(w, "failed to get server wallet pubkey", http.StatusBadRequest)
			return
		}

		serialNumber := base64.StdEncoding.EncodeToString(hmac.HMAC[:])
		certFields, err := mapping.MapToCertificateFields(req.Fields)
		require.NoError(b.TB, err)

		signedCertificate := certificates.NewCertificate(
			sdk.StringBase64(req.Type),
			sdk.StringBase64(serialNumber),
			*clientPubKey,
			*certifierKey.PublicKey,
			&transaction.Outpoint{},
			certFields,
			nil)
		err = signedCertificate.Sign(b.Context(), b.serverWallet)
		if err != nil {
			logger.Error("failed to sign certificate", slog.Any("error", err))
			http.Error(w, "failed to sign certificate", http.StatusBadRequest)
			return
		}

		// Mock response with a certificate
		response := actions.ProtocolIssuanceResponse{
			ServerNonce: string(serverNonce),
			Certificate: &actions.Certificate{
				Type:               string(signedCertificate.Type),
				SerialNumber:       string(signedCertificate.SerialNumber),
				Subject:            signedCertificate.Subject.ToDERHex(),
				Certifier:          signedCertificate.Certifier.ToDERHex(),
				RevocationOutpoint: signedCertificate.RevocationOutpoint.String(),
				Fields:             b.convertFieldsToString(signedCertificate.Fields),
				Signature:          hex.EncodeToString(signedCertificate.Signature),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode response", slog.Any("error", err))
		}
	}
}

func (b *certifierServerBuilder) convertFieldsToString(fields map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64) map[string]string {
	stringFields := make(map[string]string)
	for k, v := range fields {
		stringFields[string(k)] = string(v)
	}
	return stringFields
}
