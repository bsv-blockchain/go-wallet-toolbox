// Package keyderiver is the toolbox seam for BRC-42 and BRC-43 operations
// that must not return a root private key.
//
// go-sdk's wallet.KeyDeriver is a concrete struct. wallet.ProtoWallet stores
// that struct and reads its unexported root scalar, so an HSM implementation
// cannot be passed to NewProtoWallet. Callers that have a raw key keep using
// ProtoWallet. Callers that do not (hsm.enabled) use a Deriver: the HSM
// module implements this interface without importing this package.
package keyderiver
