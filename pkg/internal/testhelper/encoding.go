package testhelper

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
)

func BytesFromBase64(s string) []byte {
	result, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return result
}

func DerivationByNumber(n int64) (asBytes []byte, base64Str string) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, n)
	if err != nil {
		panic(err)
	}

	return buf.Bytes(), base64.StdEncoding.EncodeToString(buf.Bytes())
}
