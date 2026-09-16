//go:build cgo && !nobdk && !ios && !android && (darwin || linux) && (amd64 || arm64)

package signaturebackend

import "github.com/bsv-blockchain/go-sdk/transaction/bdk"

// Name identifies the backend this build uses.
const Name = "gobdk-libsecp256k1"

func init() {
	bdk.InstallSignatureBackend()
}
