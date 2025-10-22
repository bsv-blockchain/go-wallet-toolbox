package chaintracks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/server"
	"github.com/go-softwarelab/common/pkg/to"
)

type Handler struct {
	logger *slog.Logger
	mux    *http.ServeMux
	config defs.ChaintracksServiceConfig
}

func NewHandler(logger *slog.Logger, config defs.ChaintracksServiceConfig) (*Handler, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid chaintracks service config: %w", err)
	}

	handler := &Handler{
		logger: logging.Child(logger, "chaintracks_handler"),
		mux:    http.NewServeMux(),
		config: config,
	}

	handler.mux.HandleFunc("/robots.txt", handler.handleRobotsTxt)
	handler.mux.HandleFunc("/", handler.handleRoot)
	handler.mux.HandleFunc("/getChain", handler.handleGetChain)

	return handler, nil
}

func (h *Handler) Handler() http.Handler {
	return servercommon.AllowAllCORSMiddleware(h.mux)
}

func (h *Handler) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /"))
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(fmt.Sprintf("Chaintracks %sNet Block Header Service", string(h.config.Chain))))
}

func (h *Handler) handleGetChain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := ResponseFrame[string]{
		Value:  to.Ptr(string(h.config.Chain)),
		Status: ResponseStatusSuccess,
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
