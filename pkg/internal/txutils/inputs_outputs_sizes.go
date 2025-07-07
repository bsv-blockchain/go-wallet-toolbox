package txutils

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

const P2PKHUnlockingScriptLength = 107

var P2PKHOutputSize = TransactionOutputSize(25)
var P2PKHEstimatedInputSize = TransactionInputSize(P2PKHUnlockingScriptLength)

// EstimatedInputSizeByType returns the estimated size of a transaction output based on its type.
func EstimatedInputSizeByType(txType wdk.OutputType) uint64 {
	switch txType {
	case wdk.OutputTypeP2PKH:
		return P2PKHEstimatedInputSize

	case wdk.OutputTypeCustom:
		fallthrough
	default:
		panic("unsupported tx type: " + string(txType))
	}
}
