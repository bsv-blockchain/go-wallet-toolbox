package bitails

import (
	"net/http"
	"time"
)

// Retry configuration constants
const (
	Retries         = 2
	RetriesWaitTime = 2 * time.Second
)

// URLs for Bitails API
const (
	ProductionURL = "https://api.bitails.io/"
	TestnetURL    = "https://test-api.bitails.io/"
)

// Service constants for Bitails
const (
	ServiceName             = "Bitails"
	BroadcastEndpoint       = "tx/broadcast/multi"
	FetchInfoEndpointFormat = "tx/%s/status"
)

// Bitails Error Codes
const (
	ErrorCodeAlreadyInMempool = -27
	ErrorCodeMissingInputs    = -25
)

// HTTP Status Codes
const (
	HTTPStatusCreated = http.StatusCreated
	HTTPStatusOK      = http.StatusOK
)
