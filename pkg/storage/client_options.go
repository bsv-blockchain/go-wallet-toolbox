package storage

import (
	"log/slog"
	"net/http"
)

// ClientOptions is a function that can be used to override internal dependencies.
// This is meant to be used for testing purposes.
type ClientOptions = func(*clientOptions)

type clientOptions struct {
	httpClient *http.Client
	logger     *slog.Logger
}

func defaultClientOptions() clientOptions {
	return clientOptions{}
}

// WithClientLogger is a function that can be used to set the logger for a client.
func WithClientLogger(logger *slog.Logger) ClientOptions {
	return func(o *clientOptions) {
		o.logger = logger
	}
}

// WithHttpClient is a function that can be used to override the http.Client used by the client.
// This is meant to be used for testing purposes.
func WithHttpClient(httpClient *http.Client) ClientOptions {
	return func(o *clientOptions) {
		o.httpClient = httpClient
	}
}
