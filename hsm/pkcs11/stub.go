//go:build !pkcs11

// Package pkcs11 is the Securosys PKCS#11 backend for hsm.Deriver.
// The implementation is built with -tags pkcs11 so the default module
// tests do not need cgo or a provider library.
package pkcs11
