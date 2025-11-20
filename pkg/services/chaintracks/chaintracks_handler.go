package chaintracks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/server"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/go-softwarelab/common/pkg/to"
)

// Handler implements the HTTP API endpoints for Chaintracks services, including routing, logging, and config access.
// It embeds an HTTP multiplexer, logger, and validated service configuration for BSV network operations.
type Handler struct {
	logger  *slog.Logger
	mux     *http.ServeMux
	service *Service
}

// NewHandler creates a new Handler with the provided logger and ChaintracksServiceConfig.
// NewHandler validates the config and registers HTTP handlers for root, robots.txt, and getChain endpoints.
// Returns an initialized Handler or an error if validation fails.
func NewHandler(logger *slog.Logger, service *Service) (*Handler, error) {
	handler := &Handler{
		logger:  logging.Child(logger, "chaintracks_handler"),
		mux:     http.NewServeMux(),
		service: service,
	}

	handler.mux.HandleFunc("GET /robots.txt", handler.handleRobotsTxt)
	handler.mux.HandleFunc("GET /", handler.handleRoot)
	handler.mux.HandleFunc("GET /getChain", handler.handleGetChain)
	handler.mux.HandleFunc("GET /getInfo", handler.handleGetInfo)
	handler.mux.HandleFunc("GET /getPresentHeight", handler.handlePresentHeight)
	handler.mux.HandleFunc("GET /findChainTipHashHex", handler.handleFindTipHashHex)
	handler.mux.HandleFunc("GET /findHeaderHexForHeight", handler.handleFindHeaderHexForHeight)

	// FIXME: in TS the endpoint is named findChainTipHeaderHex but it returns full JSON, not the hex
	handler.mux.HandleFunc("GET /findChainTipHeaderHex", handler.handleFindChainTipHeader)

	return handler, nil
}

// Handler returns an http.Handler for Chaintracks endpoints with CORS enabled and internal routing applied.
func (h *Handler) Handler() http.Handler {
	return servercommon.AllowAllCORSMiddleware(h.mux)
}

func (h *Handler) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	if _, err := fmt.Fprintf(w, "User-agent: *\nDisallow: /"); err != nil {
		h.logger.Error("failed to write robots.txt response", slog.String("error", err.Error()))
	}
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	if _, err := fmt.Fprintf(w, "Chaintracks %sNet Block Header Service", string(h.service.GetChain())); err != nil {
		h.logger.Error("failed to write root response", slog.String("error", err.Error()))
	}
}

func (h *Handler) handleGetChain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := models.ResponseFrame[string]{
		Value:  to.Ptr(string(h.service.GetChain())),
		Status: models.ResponseStatusSuccess,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) handleGetInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	info, err := h.service.GetInfo(r.Context())
	if err != nil {
		h.logger.Error("failed to get info", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	response := models.ResponseFrame[models.InfoResponse]{
		Value:  info,
		Status: models.ResponseStatusSuccess,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) handlePresentHeight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	height, err := h.service.GetPresentHeight(r.Context())
	if err != nil {
		h.logger.Error("failed to get present height", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	response := models.ResponseFrame[uint]{
		Value:  to.Ptr(height),
		Status: models.ResponseStatusSuccess,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) handleFindChainTipHeader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tipHeader, err := h.service.FindChainTipHeader(r.Context())
	if err != nil {
		h.logger.Error("failed to find chain tip header hex", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	response := models.ResponseFrame[models.BlockHeader]{
		Value:  liveBlockHeaderToBlockHeaderDTO(tipHeader),
		Status: models.ResponseStatusSuccess,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) handleFindTipHashHex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tipHash, err := h.service.FindChainTipHeader(r.Context())
	if err != nil {
		h.logger.Error("failed to find chain tip hash hex", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	response := models.ResponseFrame[string]{
		Value:  to.Ptr(tipHash.Hash),
		Status: models.ResponseStatusSuccess,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) handleFindHeaderHexForHeight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	heightParam := r.URL.Query().Get("height")
	if heightParam == "" {
		http.Error(w, "Missing 'height' query parameter", http.StatusBadRequest)
		return
	}

	var height uint
	if _, err := fmt.Sscanf(heightParam, "%d", &height); err != nil {
		http.Error(w, "Invalid 'height' query parameter", http.StatusBadRequest)
		return
	}

	header, err := h.service.FindHeaderForHeight(r.Context(), height)
	if err != nil {
		h.logger.Error("failed to find header hex for height", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	response := models.ResponseFrame[models.BlockHeader]{
		Value:  liveBlockHeaderToBlockHeaderDTO(header),
		Status: models.ResponseStatusSuccess,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *Handler) writeJSONResponse(w http.ResponseWriter, statusCode int, response any) {
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(response); err != nil {
		h.logger.Error("failed to encode JSON response", slog.String("error", err.Error()))
	}
}
