package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCORSMiddlewareAllowsConfiguredOrigin(t *testing.T) {
	nextCalled := false
	handler := NewCORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusAccepted)
	}), CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://wallet.example.com"},
		AllowedMethods: []string{http.MethodPost},
		AllowedHeaders: []string{"Content-Type"},
		ExposedHeaders: []string{"X-BSV-Auth-Identity-Key"},
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://wallet.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, nextCalled)
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "https://wallet.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "POST", rec.Header().Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Private-Network"))
}

func TestCORSMiddlewareRejectsUnconfiguredOrigin(t *testing.T) {
	handler := NewCORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://wallet.example.com"},
		AllowedMethods: []string{http.MethodPost},
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddlewareAllowsAnyOriginWhenConfigured(t *testing.T) {
	handler := NewCORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), CORSConfig{
		Enabled:         true,
		AllowAllOrigins: true,
		AllowedMethods:  []string{http.MethodPost},
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://any-wallet.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "https://any-wallet.example", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Origin", rec.Header().Values("Vary")[0])
}

func TestCORSMiddlewareHandlesAllowedPreflight(t *testing.T) {
	handler := NewCORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), CORSConfig{
		Enabled:             true,
		AllowedOrigins:      []string{"http://localhost:3000"},
		AllowedMethods:      []string{http.MethodPost},
		AllowedHeaders:      []string{"Content-Type"},
		AllowPrivateNetwork: true,
	})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Private-Network"))
}

func TestCORSConfigValidateAllowsAllOriginsWithoutList(t *testing.T) {
	err := (CORSConfig{
		Enabled:         true,
		AllowAllOrigins: true,
		AllowedMethods:  []string{http.MethodPost},
	}).Validate()

	require.NoError(t, err)
}

func TestCORSConfigValidateRejectsWildcardOrigin(t *testing.T) {
	err := (CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodPost},
	}).Validate()

	require.ErrorContains(t, err, "wildcard origins are not allowed")
}
