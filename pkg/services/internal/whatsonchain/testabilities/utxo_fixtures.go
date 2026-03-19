package testabilities

import "fmt"

const (
	TestScriptHash       = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	TestTxIDHex          = "abc123def4567890abc123def4567890abc123def4567890abc123def4567890"
	TestTxIndex          = uint32(1)
	TestUtxoHeight       = int64(700000)
	TestUtxoSatoshis     = uint64(123456)
	TestUtxoMempoolSpent = false
)

// UtxoSuccessJSON returns a valid JSON response for a confirmed UTXO.
func UtxoSuccessJSON(scriptHash, txid string, index uint32, height int64, value uint64) string {
	return fmt.Sprintf(`{
		"script": "%s",
		"result": [{
			"height": %d,
			"tx_pos": %d,
			"tx_hash": "%s",
			"value": %d,
			"isSpentInMempoolTx": false,
			"status": "confirmed"
		}],
		"error": ""
	}`, scriptHash, height, index, txid, value)
}

// UtxoAPIErrorJSON returns a WoC-style error payload.
func UtxoAPIErrorJSON(errMsg string) string {
	return fmt.Sprintf(`{
		"result": [],
		"error": "%s"
	}`, errMsg)
}
