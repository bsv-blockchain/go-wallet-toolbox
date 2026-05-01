package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaxBytesMiddlewareRejectsKnownOversizedBody(t *testing.T) {
	handler := MaxBytesMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), 4)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("12345"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestMaxBytesMiddlewareCapsStreamingBodyReads(t *testing.T) {
	handler := MaxBytesMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}), 4)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("12345"))
	req.ContentLength = -1
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
