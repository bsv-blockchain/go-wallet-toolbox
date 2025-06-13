package main

import (
	"encoding/base64"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/methods"
)

func main() {
	text := "Hello, World! This is a test message for encryption."
	identityKey := "020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03"
	encryptResult := methods.EncryptHandler(text, identityKey)

	fmt.Printf("Encrypted Base64 Result:\n%s\n", base64.StdEncoding.EncodeToString(encryptResult.Ciphertext))
}
