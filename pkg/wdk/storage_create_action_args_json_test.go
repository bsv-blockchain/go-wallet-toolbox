package wdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// Regression for #776: TS storage servers expect inputBEEF as a number array
// (not base64) and outpoint fields as lowercase "txid"/"vout".
func TestValidCreateActionArgsWireFormat(t *testing.T) {
	args := ValidCreateActionArgs{
		Description: "spend existing output",
		InputBEEF:   primitives.BEEF{0, 1, 255},
		Inputs: []ValidCreateActionInput{
			{
				Outpoint: OutPoint{
					TxID: "abcd1234",
					Vout: 1,
				},
				InputDescription: "input 0",
			},
		},
		Outputs: []ValidCreateActionOutput{},
		Labels:  []primitives.StringUnder300{},
		Options: ValidCreateActionOptions{
			SendWith:     []primitives.TXIDHexString{},
			KnownTxids:   []primitives.TXIDHexString{},
			NoSendChange: []OutPoint{},
		},
	}

	data, err := json.Marshal(args)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	// inputBEEF must be a JSON number array, never a base64 string.
	assert.Equal(t, "[0,1,255]", string(raw["inputBEEF"]))

	// Nested outpoint must use lowercase JSON keys expected by TS.
	var inputs []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["inputs"], &inputs))
	require.Len(t, inputs, 1)

	var outpoint map[string]any
	require.NoError(t, json.Unmarshal(inputs[0]["outpoint"], &outpoint))
	assert.Equal(t, "abcd1234", outpoint["txid"])
	assert.EqualValues(t, 1, outpoint["vout"])
	_, hasTxID := outpoint["TxID"]
	_, hasVoutPascal := outpoint["Vout"]
	assert.False(t, hasTxID, "must not emit PascalCase TxID")
	assert.False(t, hasVoutPascal, "must not emit PascalCase Vout")
}

func TestOutPointJSONTags(t *testing.T) {
	data, err := json.Marshal(OutPoint{TxID: "deadbeef", Vout: 7})
	require.NoError(t, err)
	assert.JSONEq(t, `{"txid":"deadbeef","vout":7}`, string(data))

	var decoded OutPoint
	require.NoError(t, json.Unmarshal([]byte(`{"txid":"cafebabe","vout":3}`), &decoded))
	assert.Equal(t, OutPoint{TxID: "cafebabe", Vout: 3}, decoded)
}
