//go:build !(cgo && !nobdk && !ios && !android && (darwin || linux) && (amd64 || arm64))

package signaturebackend

// Name identifies the backend this build uses.
const Name = "go-sdk-builtin"
