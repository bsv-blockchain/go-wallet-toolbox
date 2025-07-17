package testhelper

import ec "github.com/bsv-blockchain/go-sdk/primitives/ec"

func IdentityKeyFromHex(hex string) *ec.PublicKey {
	result, err := ec.PublicKeyFromString(hex)
	if err != nil {
		panic(err)
	}
	return result
}
