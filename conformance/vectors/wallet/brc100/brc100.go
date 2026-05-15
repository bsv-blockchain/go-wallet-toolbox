// Package brc100 embeds vendored BRC-100 wallet method conformance vectors.
//
// Origin: bsv-blockchain/ts-stack (conformance/vectors/wallet/brc100/*.json).
// Each file contains request/response vectors for one (or a small group of)
// BRC-100 wallet interface methods (root_key + args → expected result).
// These are logical (not wire) vectors for the sdk.Interface / Wallet methods.
//
// We vendor only the core high-value files initially. Additional files from
// the upstream directory can be added as coverage expands.
//
// Refresh: ./conformance/scripts/refresh-vectors.sh
package brc100

import _ "embed"

// CreateActionVectors contains the 90+ vectors for wallet.brc100.createaction.
//
//go:embed createaction.json
var CreateActionVectors []byte

// SignActionVectors contains the vectors for wallet.brc100.signaction.
//
//go:embed signaction.json
var SignActionVectors []byte

// InternalizeActionVectors contains the vectors for wallet.brc100.internalizeaction.
//
//go:embed internalizeaction.json
var InternalizeActionVectors []byte

// ListOutputsVectors contains the vectors for wallet.brc100.listoutputs.
//
//go:embed listoutputs.json
var ListOutputsVectors []byte

// ListActionsVectors contains the vectors for wallet.brc100.listactions.
//
//go:embed listactions.json
var ListActionsVectors []byte

// ProveCertificateVectors contains the vectors for wallet.brc100.provecertificate.
//
//go:embed provecertificate.json
var ProveCertificateVectors []byte

// RelinquishOutputVectors contains the vectors for wallet.brc100.relinquishoutput.
//
//go:embed relinquishoutput.json
var RelinquishOutputVectors []byte

// GetPublicKeyVectors contains the vectors for wallet.brc100.getpublickey.
//
//go:embed getpublickey.json
var GetPublicKeyVectors []byte

// GetNetworkVectors contains the vectors for wallet.brc100.getnetwork.
//
//go:embed getnetwork.json
var GetNetworkVectors []byte
