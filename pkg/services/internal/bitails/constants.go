package bitails

import (
	"net/http"
)

// URLs for Bitails API
const (
	ProductionURL = "https://api.bitails.io/"
	TestnetURL    = "https://test-api.bitails.io/"
)

// Service constants for Bitails
const (
	ServiceName       = "Bitails"
	BroadcastEndpoint = "tx/broadcast/multi"
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
	BlockHeaderLength = 80
	MerkleRootOffset  = 36
	MerkleRootLength  = 32
)
