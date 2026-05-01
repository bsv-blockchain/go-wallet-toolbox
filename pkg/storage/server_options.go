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

	// CORS controls optional browser cross-origin access. It is disabled by default.
	CORS servercommon.CORSConfig

	// Monetize - should the payment middleware be applied to the server
	Monetize bool

	// CalculateRequestPrice optional custom implementation of function that calculates the price of the request.
	// Used only if the Monetize option is set to true.
	CalculateRequestPrice func(r *http.Request) (int, error)
}

// DefaultCORSConfig returns a disabled CORS config populated with the headers
// required by Authrite and storage request payment flows.
func DefaultCORSConfig() servercommon.CORSConfig {
	return servercommon.CORSConfig{
		Enabled:        false,
		AllowedMethods: []string{http.MethodPost},
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
