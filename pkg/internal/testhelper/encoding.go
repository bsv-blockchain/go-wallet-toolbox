package testhelper

import "encoding/base64"

func BytesFromBase64(s string) []byte {
	result, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return result
}
