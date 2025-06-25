package bitails

import (
	"net/http"
	"time"
)

const (
	// Retry Config
	Retries         = 2
	RetriesWaitTime = 2 * time.Second

	// URLs
	ProductionURL = "https://api.bitails.io/"
	TestnetURL    = "https://test-api.bitails.io/"

	// Service
	ServiceName             = "Bitails"
	BroadcastEndpoint       = "tx/broadcast/multi"
	FetchInfoEndpointFormat = "tx/%s/status"

	// Bitails Error Codes
	ErrorCodeAlreadyInMempool = -27
	ErrorCodeMissingInputs    = -25

	// HTTP Status Codes
	HTTPStatusCreated = http.StatusCreated
	HTTPStatusOK      = http.StatusOK
)
