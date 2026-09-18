// Package signaturebackend selects go-sdk's secp256k1 implementation from the
// build. A binary built with cgo on a platform GoBDK supports (darwin or linux,
// amd64 or arm64) signs and verifies through GoBDK (libsecp256k1), ~7x faster
// than go-sdk's built-in pure-Go implementation. Any other build, or one with
// -tags nobdk, keeps the built-in implementation.
//
// The switch is process-wide and has to precede any signing or verification, so
// it happens when the package loads. pkg/storage imports it, so every wallet and
// storage built from this module gets it.
package signaturebackend
