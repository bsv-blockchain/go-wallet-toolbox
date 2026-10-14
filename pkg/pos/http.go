package pos

import (
	"encoding/json"
	"net/http"
)

// Handler mounts a signed RPC endpoint; terminate TLS at the deployment edge.
// An acceptance error is uncertain, never authorization to discard a payment.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 12000000)
		var signed Signed
		if err := json.NewDecoder(r.Body).Decode(&signed); err != nil {
			http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
			return
		}
		result, err := s.Call(r.Context(), signed)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Request could not be completed. Check status before retrying payment."})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
}
