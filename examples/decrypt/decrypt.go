package main

import (
	"encoding/base64"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/methods"
)

func main() {
	ciphertext := "+QwtscldxX4DRD0qChriIRBE6imUBL/aRkRVJk1n7a75ScyN/fpqn1szH7ztJRSRko9c4K/8jGXM1H+bzPJ38JZ1Ggw7lw4PFLvpfv/Q3jCuihEuGdCgzZjsfe/clUZDhN3pKg=="

	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		fmt.Printf("Error decoding base64: %v\n", err)
		return
	}

	identityKey := "020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03"
	decryptResult := methods.DecryptHandler(ciphertextBytes, identityKey)

	if decryptResult.Plaintext != nil {
		fmt.Printf("Decrypted Message: %s\n", string(decryptResult.Plaintext))
	}
}
