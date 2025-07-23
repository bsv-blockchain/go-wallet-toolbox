package history

import (
	"encoding/hex"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/to"
)

type TxData struct {
	hex *string
}

func BeefObj(beef *transaction.Beef) TxData {
	bytes, err := beef.Bytes()
	var content string
	if err != nil {
		content = fmt.Sprintf("<couldn't convert beef object to beef bytes: %v>", err)
	} else {
		content = hex.EncodeToString(bytes)
	}

	return TxData{
		hex: &content,
	}
}

func Hex(content string) TxData {
	return TxData{
		hex: &content,
	}
}

func Bytes(bytes []byte) TxData {
	return TxData{
		hex: to.Ptr(hex.EncodeToString(bytes)),
	}
}

func (b *TxData) toHex() string {
	if b.hex == nil {
		return "<empty hex>"
	}
	return *b.hex
}
