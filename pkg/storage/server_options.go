package storage

import (
	"net/http"

	"github.com/bsv-blockchain/go-sdk/auth/brc104"

	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/server"
)

const (
	headerBSVPayment                 = "X-BSV-Payment"
	headerBSVPaymentVersion          = "X-BSV-Payment-Version"
	headerBSVPaymentSatoshisRequired = "X-BSV-Payment-Satoshis-Required"
	headerBSVPaymentSatoshisPaid     = "X-BSV-Payment-Satoshis-Paid"
	headerBSVPaymentDerivationPrefix = "X-BSV-Payment-Derivation-Prefix"
)

// ServerOptions represents configurable options for the storage server
type ServerOptions struct {
	Port uint
	// MaxRequestBodyBytes caps request bodies before they reach auth or JSON-RPC handlers.
	// When zero, server.DefaultMaxRequestBodyBytes is used.
	MaxRequestBodyBytes int64

	// CORS controls browser cross-origin access. Storage servers allow all origins by default
	// because wallet applications are expected to run from arbitrary web origins.
	CORS servercommon.CORSConfig
	// DisableCORS disables the default storage CORS middleware.
	DisableCORS bool

	// Monetize - should the payment middleware be applied to the server
	Monetize bool

	// CalculateRequestPrice optional custom implementation of function that calculates the price of the request.
	// Used only if the Monetize option is set to true.
	CalculateRequestPrice func(r *http.Request) (int, error)

	// AllowUnauthenticated permits requests without valid BRC-104 auth (e.g. the synthetic
	// "Bearer brc103-session-token-abc123" bearer used by storage adapter conformance vectors)
	// to reach the inner v1adapter handler. When false (default) the auth middleware
	// rejects with 401. This is enabled only for adapter_conformance_test.go.
	AllowUnauthenticated bool
}

// DefaultCORSConfig returns an open-origin CORS config populated with the headers
// required by Authrite and storage request payment flows.
func DefaultCORSConfig() servercommon.CORSConfig {
	return servercommon.CORSConfig{
		Enabled:             true,
		AllowAllOrigins:     true,
		AllowPrivateNetwork: true,
		AllowedMethods:      []string{http.MethodPost},
		AllowedHeaders: []string{
			brc104.HeaderContentType,
			brc104.HeaderAuthorization,
			brc104.HeaderVersion,
			brc104.HeaderMessageType,
			brc104.HeaderIdentityKey,
			brc104.HeaderNonce,
			brc104.HeaderYourNonce,
			brc104.HeaderSignature,
			brc104.HeaderRequestID,
			brc104.HeaderRequestedCertificates,
			headerBSVPayment,
		},
		ExposedHeaders: []string{
			brc104.HeaderAuthorization,
			brc104.HeaderVersion,
			brc104.HeaderMessageType,
			brc104.HeaderIdentityKey,
			brc104.HeaderNonce,
			brc104.HeaderYourNonce,
			brc104.HeaderSignature,
			brc104.HeaderRequestID,
			headerBSVPaymentVersion,
			headerBSVPaymentSatoshisRequired,
			headerBSVPaymentSatoshisPaid,
			headerBSVPaymentDerivationPrefix,
		},
	}
}
