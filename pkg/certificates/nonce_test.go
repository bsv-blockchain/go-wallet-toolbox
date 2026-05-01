package certificates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNonceKeyIDUsesLosslessEncoding(t *testing.T) {
	first := []byte{0xff}
	second := []byte{0xfe}

	require.Equal(t, BytesToUTF8(first), BytesToUTF8(second))
	require.NotEqual(t, NonceKeyID(first), NonceKeyID(second))
}
