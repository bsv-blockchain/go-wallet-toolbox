package history

import (
	"encoding/hex"
	"fmt"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// TxData encapsulates transaction data.
// It works as a union type to handle different representations of transaction data
type TxData struct {
	beefObj   *transaction.Beef
	hex   *string
	bytes []byte
}

func BeefObj(beef *transaction.Beef) TxData {
	return TxData{
		beefObj: beef,
	}
}

func Hex(beefHex string) TxData {
	return TxData{
		hex: &beefHex,
	}
}

func Bytes(beefBytes []byte) TxData {
	return TxData{
		bytes: beefBytes,
	}
}

func (b *TxData) toHex() string {
	if b.hex != nil {
		return *b.hex
	}

	if b.beefObj != nil {
		bytes, err := b.beefObj.Bytes()
		if err != nil {
			return fmt.Sprintf("<couldn't convert beef object to beef bytes: %v>", err)
		}
		return hex.EncodeToString(bytes)
	}

	if len(b.bytes) > 0 {
		return hex.EncodeToString(b.bytes)
	}

	return "<empty tx data container>"
}
