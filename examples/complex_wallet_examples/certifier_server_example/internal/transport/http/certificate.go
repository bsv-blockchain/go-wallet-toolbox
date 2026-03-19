package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/certifier-server-example/internal/config"
	"github.com/bsv-blockchain/certifier-server-example/internal/constants"
	"github.com/bsv-blockchain/certifier-server-example/internal/service"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
)

type CertificateHandler struct {
	service *service.CertificateService
	cfg     *config.Config // until we not using auth middleware we will take key from config
	logger  *slog.Logger
}

func NewCertificateHandler(svc *service.CertificateService, config *config.Config, logger *slog.Logger) *CertificateHandler {
	return &CertificateHandler{
		service: svc,
		cfg:     config,
		logger:  logger,
	}
}

func (h *CertificateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info("Received certificate signing request", "path", r.URL.Path)

	body, err := h.readRequestBody(r)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		h.writeError(w, "Bad Request", http.StatusBadRequest)
		return
	}

	var masterCert certificates.MasterCertificate
	if err = json.Unmarshal(body, &masterCert); err != nil {
		h.logger.Error("Failed to unmarshal JSON", "error", err, "body", string(body))
		h.writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.Info("Parsed certificate request", "cert_type", masterCert.Type)

	// TODO: change to take pubkey from request when middleware with auth will be attached
	counterPartyPrivKey, err := ec.PrivateKeyFromHex(h.cfg.UserWallet.PrivateKey)
	if err != nil {
		h.logger.Error("Failed to parse private key", "error", err)
		h.writeError(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	counterPartyPubKey := counterPartyPrivKey.PubKey()

	signedCert, err := h.service.SignCertificate(&masterCert, counterPartyPubKey)
	if err != nil {
		h.logger.Error("Certificate signing failed", "error", err)
		h.writeError(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, signedCert, http.StatusOK)
	h.logger.Info("Certificate signed and response sent", "status", http.StatusOK)
}

func (h *CertificateHandler) readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close() //nolint:errcheck // body close error is not actionable in a request handler
	return io.ReadAll(r.Body)
}

func (h *CertificateHandler) writeError(w http.ResponseWriter, message string, statusCode int) {
	http.Error(w, message, statusCode)
}

func (h *CertificateHandler) writeJSONResponse(w http.ResponseWriter, data []byte, statusCode int) {
	w.Header().Set("Content-Type", constants.ContentTypeJSON)
	w.WriteHeader(statusCode)
	if _, err := w.Write(data); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
}
