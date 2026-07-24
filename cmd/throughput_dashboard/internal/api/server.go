// Package api serves the throughput dashboard HTTP control plane and UI.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"

	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/funding"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/metrics"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/stream"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

const headerContentType = "Content-Type"

// FuelPool is the optional FuelKeeper surface resized when the stream starts
// so inventory targets track the UI TPS setting.
type FuelPool interface {
	SetTargetPoolSize(n uint64) error
	TargetPoolSize() uint64
}

// Deps wires the HTTP server.
type Deps struct {
	Ctrl       *stream.Controller
	Sampler    *metrics.Sampler
	// Fuel is optional; when set, stream start resizes the target pool from TPS.
	Fuel       FuelPool
	Wallet     funding.InternalizeActioner
	Priv       *ec.PrivateKey
	Network    defs.BSVNetwork
	Originator string
	ServerURL  string
	Logger     *slog.Logger
	WebFS      fs.FS
	// Done is closed when the process is shutting down (not a context.Context field).
	Done <-chan struct{}
}

// Server is the dashboard HTTP API.
type Server struct {
	deps Deps
	mux  *http.ServeMux
}

// New builds the HTTP handler.
func New(deps Deps) *Server {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Done == nil {
		deps.Done = make(chan struct{}) // never closed — no process-wide cancel
	}
	s := &Server{deps: deps, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the root handler.
func (s *Server) Handler() http.Handler {
	return withCORS(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/stream/start", s.handleStreamStart)
	s.mux.HandleFunc("POST /api/stream/stop", s.handleStreamStop)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
	s.mux.HandleFunc("GET /api/funding", s.handleFunding)
	s.mux.HandleFunc("POST /api/funding/internalize", s.handleInternalize)

	if s.deps.WebFS != nil {
		fileServer := http.FileServer(http.FS(s.deps.WebFS))
		s.mux.Handle("GET /", fileServer)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	tick := s.deps.Sampler.LastTick()
	writeJSON(w, http.StatusOK, map[string]any{
		"network":    string(s.deps.Network),
		"server_url": s.deps.ServerURL,
		"originator": s.deps.Originator,
		"mainnet":    s.deps.Network == defs.NetworkMainnet,
		"tick":       tick,
		"events":     s.deps.Sampler.RecentEvents(),
	})
}

type startBody struct {
	TPS     int `json:"tps"`
	Workers int `json:"workers"`
}

func (s *Server) handleStreamStart(w http.ResponseWriter, r *http.Request) {
	var body startBody
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	// Process shutdown cancels the stream via Ctrl.Stop() in main; no long-lived
	// parent context is stored on Server (avoids context-in-struct / cancel leaks).
	err := s.deps.Ctrl.Start(context.Background(), stream.Options{
		TPS:        body.TPS,
		Workers:    body.Workers,
		Originator: s.deps.Originator,
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	stats := s.deps.Ctrl.Stats()
	// Resize FuelKeeper inventory target from the effective stream TPS so the
	// pool scales with the UI setting (not a fixed demo TargetPoolSize).
	targetPool := config.DemoTargetPoolForTPS(stats.TPS)
	if s.deps.Fuel != nil {
		if err := s.deps.Fuel.SetTargetPoolSize(targetPool); err != nil {
			s.deps.Logger.Warn("fuel keeper target pool update failed", "error", err, "target_pool", targetPool)
		}
	}
	if s.deps.Sampler != nil {
		s.deps.Sampler.SetTargetPool(targetPool, uint64(stats.TPS)) //nolint:gosec // TPS is clamped by stream
	}
	s.deps.Logger.Info(
		"stream started; fuel target pool sized from TPS",
		"tps", stats.TPS,
		"workers", stats.Workers,
		"target_pool", targetPool,
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"stats":            stats,
		"target_pool_size": targetPool,
	})
}

func (s *Server) handleStreamStop(w http.ResponseWriter, r *http.Request) {
	s.deps.Ctrl.Stop()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stats": s.deps.Ctrl.Stats()})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerContentType, "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send current tick immediately.
	if tick := s.deps.Sampler.LastTick(); tick.Timestamp != "" {
		writeSSE(w, flusher, metrics.Event{
			Type:      "tick",
			Timestamp: tick.Timestamp,
			Payload:   map[string]any{"tick": tick},
		})
	}

	ch := s.deps.Sampler.Subscribe()
	defer s.deps.Sampler.Unsubscribe(ch)

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.deps.Done:
			return
		case <-keepalive.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, flusher, ev)
		}
	}
}

func (s *Server) handleFunding(w http.ResponseWriter, r *http.Request) {
	info, err := funding.DeriveInfo(s.deps.Priv, s.deps.Network, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleInternalize(w http.ResponseWriter, r *http.Request) {
	var req funding.InternalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	info, err := funding.DeriveInfo(s.deps.Priv, s.deps.Network, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := funding.Internalize(r.Context(), funding.InternalizeParams{
		Wallet:          s.deps.Wallet,
		Network:         s.deps.Network,
		ExpectedAddress: info.Address,
		Request:         req,
		Originator:      s.deps.Originator,
		Logger:          s.deps.Logger,
	}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, ev metrics.Event) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, b)
	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"encode failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerContentType, "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(b, '\n'))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", headerContentType)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
