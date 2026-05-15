// Package storage embeds vendored storage adapter conformance vectors.
//
// Origin: bsv-blockchain/ts-stack (wallet/storage/adapter-conformance.json).
// These vectors define the exact HTTP contract (`/storage/v1/*`, `{ "args": ... }`
// request bodies, status codes, and response shapes) for the remote storage
// adapter / remoting layer. The Go storage server (v1adapter) must implement
// this contract to pass cross-impl conformance.
//
// Refresh: ./conformance/scripts/refresh-vectors.sh
package storage

// blank import required for the //go:embed directive below.
import _ "embed"

// AdapterConformance is the raw JSON corpus for the Wallet Storage Adapter
// HTTP Interface conformance (18 vectors covering settings, migrate, actions,
// list/*, certificates, outputs/relinquish, and sync/* paths).
//
//go:embed adapter-conformance.json
var AdapterConformance []byte
