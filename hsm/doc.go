// Package hsm derives and uses BSV keys with the root private key kept
// inside a Backend.
//
// The Securosys PKCS#11 backend lives in package pkcs11 and is built with
// -tags pkcs11. Tests use NewSoftwareBackend, which performs the same
// BRC-42 steps with go-sdk arithmetic and never needs hardware.
//
// go-sdk's wallet.KeyDeriver is a concrete struct that returns derived
// private keys, and wallet.ProtoWallet stores that concrete type and reads
// its unexported root scalar. This module therefore does not implement a
// go-sdk type. The toolbox defines the matching seam in pkg/keyderiver and,
// when hsm.enabled is set, calls this package instead of ProtoWallet.
// The operations go-sdk would need to export are documented in README.md.
package hsm
