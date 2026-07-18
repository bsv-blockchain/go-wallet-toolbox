package server

import (
	"errors"
	"net/http"
)

const (
	// DefaultMaxRequestBodyBytes caps JSON HTTP request bodies at 1 MiB.
	DefaultMaxRequestBodyBytes int64 = 1 << 20
)

// MaxBytesMiddleware rejects requests with a known oversized body and caps reads
// for streaming requests before they reach downstream handlers.
func MaxBytesMiddleware(next http.Handler, maxBytes int64) http.Handler {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxRequestBodyBytes
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			if r.ContentLength > maxBytes {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}

		next.ServeHTTP(w, r)
	})
}

// IsMaxBytesError reports whether an error came from http.MaxBytesReader.
func IsMaxBytesError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
