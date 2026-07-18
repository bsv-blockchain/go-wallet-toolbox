package certifier

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/auth/brc104"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"

	walletcerts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/certificates"
	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/server"
)

func (s *Server) handleSignCertificate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// get client identity key from auth middleware header
	clientPubKey, err := ec.PublicKeyFromString(r.Header.Get(brc104.HeaderIdentityKey))
	if err != nil {
		s.config.Logger.ErrorContext(r.Context(), "failed to create client public key", slog.Any("error", err))
		http.Error(w, "failed to create client public key", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxRequestBodyBytes())

	var req walletcerts.ProtocolIssuanceRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.config.Logger.ErrorContext(r.Context(), "failed to decode request", "error", err)
		if servercommon.IsMaxBytesError(err) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Sign the certificate
	response, err := s.service.SignCertificate(r.Context(), &req, clientPubKey)
	if err != nil {
		s.config.Logger.ErrorContext(r.Context(), "failed to sign certificate", "error", err)
		http.Error(w, "Failed to sign certificate", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.config.Logger.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

func (s *Server) maxRequestBodyBytes() int64 {
	if s.config != nil && s.config.MaxRequestBodyBytes > 0 {
		return s.config.MaxRequestBodyBytes
	}
	return servercommon.DefaultMaxRequestBodyBytes
}
