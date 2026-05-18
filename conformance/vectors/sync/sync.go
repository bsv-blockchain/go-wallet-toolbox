// Package sync embeds the vendored BRC sync conformance vectors so Go test
// runners can consume them without filesystem path resolution.
//
// Origin: bsv-blockchain/ts-stack. Pinned via conformance/SOURCE.
// Refresh: ./conformance/scripts/refresh-vectors.sh
package sync

import _ "embed"

// BRC40UserState is the raw JSON corpus for sync.brc40 (BRC-40 User Wallet
// Data Synchronization). See ts-stack conformance/runner/ts/dispatchers/sync.ts
// for the reference dispatcher contract.
//
//go:embed brc40-user-state.json
var BRC40UserState []byte
