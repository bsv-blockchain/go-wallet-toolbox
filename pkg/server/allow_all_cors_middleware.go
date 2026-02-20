package server

import "net/http"

var _ http.Handler = (*allowAllCORSMiddleware)(nil)

type allowAllCORSMiddleware struct {
	next http.Handler
}

// AllowAllCORSMiddleware creates a new middleware that allows all CORS requests.
func AllowAllCORSMiddleware(next http.Handler) http.Handler {
	return &allowAllCORSMiddleware{
		next: next,
	}
}

func (m *allowAllCORSMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	m.next.ServeHTTP(w, r)
}
