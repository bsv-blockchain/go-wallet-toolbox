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
	ServiceName = "Bitails"
)

// Bitails Error Codes
const (
	ErrorCodeAlreadyInMempool = "-27"
	ErrorCodeDoubleSpend      = "-26"
	ErrorCodeMissingInputs    = "-25"
)

// Network error tokens found in Bitails error messages
const (
	ErrorTokenECONNRESET   = "ECONNRESET"
	ErrorTokenECONNREFUSED = "ECONNREFUSED"
)

// HTTP Status Codes
const (
	HTTPStatusCreated = http.StatusCreated
	HTTPStatusOK      = http.StatusOK
	BlockHeaderLength = 80
	MerkleRootOffset  = 36
	MerkleRootLength  = 32
)
