package bitails

import (
	"net/http"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// URLs for Bitails API
const (
	ProductionURL = "https://api.bitails.io/"
	TestnetURL    = "https://test-api.bitails.io/"
)

const (
	ServiceName = defs.BitailsServiceName
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
