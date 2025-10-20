package storage

import "net/http"

// ServerOptions represents configurable options for the storage server
type ServerOptions struct {
	Port uint
	// Monetize - should the payment middleware be applied to the server
	Monetize bool

	// CalculateRequestPrice optional custom implementation of function that calculates the price of the request.
	// Used only if the Monetize option is set to true.
	CalculateRequestPrice func(r *http.Request) (int, error)
}
