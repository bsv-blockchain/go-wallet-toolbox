//go:build !pkcs11

// Command infra is the storage server with the Securosys PKCS#11 root key.
// This file is the build without the pkcs11 tag.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "hsm server requires: cd hsm/cmd/infra && go build -tags pkcs11 -o infra-hsm .")
	os.Exit(1)
}
