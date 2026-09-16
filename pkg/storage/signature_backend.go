package storage

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/signaturebackend"

// SignatureBackend names the secp256k1 implementation go-sdk uses in this binary:
// "gobdk-libsecp256k1" when it was built with cgo on darwin or linux, amd64 or
// arm64, and "go-sdk-builtin" otherwise or with -tags nobdk. The choice is made
// at build time and applies process-wide; log it at startup to confirm a
// deployment got the build it expects.
func SignatureBackend() string {
	return signaturebackend.Name
}
