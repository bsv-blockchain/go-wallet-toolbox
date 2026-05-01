package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var _ http.Handler = (*corsMiddleware)(nil)

// CORSConfig controls cross-origin browser access to an HTTP handler.
type CORSConfig struct {
	Enabled             bool     `mapstructure:"enabled"`
	AllowedOrigins      []string `mapstructure:"allowed_origins"`
	AllowedMethods      []string `mapstructure:"allowed_methods"`
	AllowedHeaders      []string `mapstructure:"allowed_headers"`
	ExposedHeaders      []string `mapstructure:"exposed_headers"`
	AllowPrivateNetwork bool     `mapstructure:"allow_private_network"`
}

// Validate checks that enabled CORS config uses explicit allowlists.
func (c CORSConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if len(c.AllowedOrigins) == 0 {
		return fmt.Errorf("allowed origins must not be empty when CORS is enabled")
	}
	for _, origin := range c.AllowedOrigins {
		if _, err := normalizeOrigin(origin); err != nil {
			return fmt.Errorf("invalid allowed origin %q: %w", origin, err)
		}
	}

	if err := validateCORSTokens("allowed methods", c.AllowedMethods, true); err != nil {
		return err
	}
	if err := validateCORSTokens("allowed headers", c.AllowedHeaders, false); err != nil {
		return err
	}
	if err := validateCORSTokens("exposed headers", c.ExposedHeaders, false); err != nil {
		return err
	}

	return nil
}

type corsMiddleware struct {
	next                http.Handler
	allowedOrigins      map[string]struct{}
	allowedMethods      string
	allowedHeaders      string
	exposedHeaders      string
	allowPrivateNetwork bool
}

// NewCORSMiddleware creates a CORS middleware with explicit allowed origins.
func NewCORSMiddleware(next http.Handler, config CORSConfig) http.Handler {
	if !config.Enabled {
		return next
	}

	allowedOrigins := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			continue
		}
		allowedOrigins[normalized] = struct{}{}
	}

	return &corsMiddleware{
		next:                next,
		allowedOrigins:      allowedOrigins,
		allowedMethods:      strings.Join(config.AllowedMethods, ", "),
		allowedHeaders:      strings.Join(config.AllowedHeaders, ", "),
		exposedHeaders:      strings.Join(config.ExposedHeaders, ", "),
		allowPrivateNetwork: config.AllowPrivateNetwork,
	}
}

func (m *corsMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		m.next.ServeHTTP(w, r)
		return
	}

	normalizedOrigin, err := normalizeOrigin(origin)
	if err != nil {
		http.Error(w, "CORS origin denied", http.StatusForbidden)
		return
	}
	if _, ok := m.allowedOrigins[normalizedOrigin]; !ok {
		http.Error(w, "CORS origin denied", http.StatusForbidden)
		return
	}

	header := w.Header()
	addVary(header, "Origin")
	addVary(header, "Access-Control-Request-Method")
	addVary(header, "Access-Control-Request-Headers")

	header.Set("Access-Control-Allow-Origin", normalizedOrigin)
	header.Set("Access-Control-Allow-Methods", m.allowedMethods)
	if m.allowedHeaders != "" {
		header.Set("Access-Control-Allow-Headers", m.allowedHeaders)
	}
	if m.exposedHeaders != "" {
		header.Set("Access-Control-Expose-Headers", m.exposedHeaders)
	}
	if m.allowPrivateNetwork && strings.EqualFold(r.Header.Get("Access-Control-Request-Private-Network"), "true") {
		header.Set("Access-Control-Allow-Private-Network", "true")
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	m.next.ServeHTTP(w, r)
}

func validateCORSTokens(name string, values []string, required bool) error {
	if required && len(values) == 0 {
		return fmt.Errorf("%s must not be empty when CORS is enabled", name)
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if value == "*" {
			return fmt.Errorf("%s must not contain wildcard values", name)
		}
	}
	return nil
}

func normalizeOrigin(origin string) (string, error) {
	if origin == "*" {
		return "", fmt.Errorf("wildcard origins are not allowed")
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("origin host is required")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must not include user info, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("origin must not include a path")
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}

func addVary(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for _, part := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(part), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}
