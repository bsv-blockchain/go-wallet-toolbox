package chaintracks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/server"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
	handler.mux.HandleFunc("GET /blockheaders/{pathname...}", handler.handleBulkBlockHeaders)

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
	if errors.Is(err, wdk.ErrNotFoundError) {
		http.Error(w, "Chain tip not found", http.StatusNotFound)
		return
	}

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
	if errors.Is(err, wdk.ErrNotFoundError) {
		http.Error(w, "Chain tip not found", http.StatusNotFound)
		return
	}

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
	if errors.Is(err, wdk.ErrNotFoundError) {
		http.Error(w, "Header not found for the specified height", http.StatusNotFound)
		return
	}

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

func (h *Handler) handleBulkBlockHeaders(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, h.bulkFilesInfoFilename()):
		h.handleFilesInfo(w, r)
	case strings.HasSuffix(r.URL.Path, ".headers"):
		h.downloadBulkFile(w, r)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (h *Handler) bulkFilesInfoFilename() string {
	return fmt.Sprintf("%sNetBlockHeaders.json", h.service.GetChain())
}

func (h *Handler) sourceURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}

	filename := h.bulkFilesInfoFilename()
	pathPrefix := r.URL.Path
	if len(pathPrefix) >= len(filename) && pathPrefix[len(pathPrefix)-len(filename):] == filename {
		pathPrefix = pathPrefix[:len(pathPrefix)-len(filename)]
	}
	sourceURL := scheme + "://" + r.Host + pathPrefix
	return sourceURL
}

func (h *Handler) handleFilesInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	filesInfo := h.service.BulkFilesInfo()
	filesInfo.JSONFilename = h.bulkFilesInfoFilename()

	sourceURL := h.sourceURL(r)
	filesInfo.RootFolder = sourceURL
	for i := range filesInfo.Files {
		filesInfo.Files[i].SourceURL = sourceURL
	}

	h.writeJSONResponse(w, http.StatusOK, filesInfo)
}

var pathRegex = regexp.MustCompile(`^.*((main|test)Net_(\d+)\.headers)$`)

func (h *Handler) downloadBulkFile(w http.ResponseWriter, r *http.Request) {
	matches := pathRegex.FindStringSubmatch(r.URL.Path)

	if matches == nil {
		http.Error(w, "Invalid file name format", http.StatusBadRequest)
		return
	}

	filename := matches[1]

	chain := matches[2]
	if chain != string(h.service.GetChain()) {
		http.Error(w, "Invalid chain in file name", http.StatusBadRequest)
		return
	}

	fileID, err := strconv.Atoi(matches[3])
	if err != nil {
		http.Error(w, "Invalid file ID in file name", http.StatusBadRequest)
		return
	}

	bulkFileData, err := h.service.BulkFileDataByIndex(fileID)
	if err != nil {
		h.logger.Error("failed to get bulk file data", slog.String("error", err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if bulkFileData == nil {
		http.Error(w, "Bulk file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	reader := bytes.NewReader(bulkFileData.Data)
	http.ServeContent(w, r, filename, bulkFileData.UpdatedAt, reader)
}

func (h *Handler) writeJSONResponse(w http.ResponseWriter, statusCode int, response any) {
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(response); err != nil {
		h.logger.Error("failed to encode JSON response", slog.String("error", err.Error()))
	}
}
