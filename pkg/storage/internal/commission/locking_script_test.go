package commission_test

import (
	"testing"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/commission"
)

func TestLockScriptWithKeyOffsetFromPubKey(t *testing.T) {
	// given:
	offsetPrivKey := "L1Yz9NvuvDru3Z7f8Kh8CK34U6UoKa6niynPEvqgHpm3AaNZig6z"
	pubKey := "02f40c35f798e2ece03ae1ebf749545336db8402eb7e620bfe04d50da8ca8b06cc"

	// and:
	generator := commission.NewScriptGenerator(pubKey)

	// and:
	// mocking the offset private key generator
	generator.SetOffsetGenerator(func() (*primitives.PrivateKey, error) {
		return primitives.PrivateKeyFromWif(offsetPrivKey)
	})

	// when:
	lockingScript, keyOffset, err := generator.Generate()

	// then:
	require.NoError(t, err)

	// NOTE: these values are cross-checked with the original/TS code
	assert.Equal(t, "76a914b95556849619ac10419b6a591b6920cb6deef47b88ac", lockingScript.String())
	assert.Equal(t, offsetPrivKey, keyOffset)
}

func TestLockScriptWithKeyOffset_Uniqueness(t *testing.T) {
	// given:
	pubKey := "02f40c35f798e2ece03ae1ebf749545336db8402eb7e620bfe04d50da8ca8b06cc"

	// and:
	generator := commission.NewScriptGenerator(pubKey)

	lockingScripts := make(map[string]struct{})
	keyOffsets := make(map[string]struct{})

	iterations := 100

	// when:
	for range iterations {
		lockingScript, keyOffset, err := generator.Generate()
		require.NoError(t, err)

		scriptHex := lockingScript.String()

		lockingScripts[scriptHex] = struct{}{}
		keyOffsets[keyOffset] = struct{}{}

		_, err = script.DecodeScriptHex(scriptHex)
		require.NoError(t, err)

		_, err = primitives.PrivateKeyFromWif(keyOffset)
		require.NoError(t, err)
	}

	// then:
	assert.Len(t, lockingScripts, iterations, "Locking script should be unique")
	assert.Len(t, keyOffsets, iterations, "Key offset should be unique")
}

func TestLockScriptWithKeyOffset_WrongPubKey(t *testing.T) {
	// given:
	generator := commission.NewScriptGenerator("wrong_pub_key")

	// when:
	_, _, err := generator.Generate()

	// then:
	assert.Error(t, err)
}
