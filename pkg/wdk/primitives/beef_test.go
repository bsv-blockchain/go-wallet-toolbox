package primitives

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBEEFMarshalsToNumberArray(t *testing.T) {
	data, err := json.Marshal(BEEF{0, 1, 255})
	require.NoError(t, err)
	assert.Equal(t, "[0,1,255]", string(data))
}

func TestBEEFMarshalsEmptyAsEmptyArray(t *testing.T) {
	data, err := json.Marshal(BEEF{})
	require.NoError(t, err)
	assert.Equal(t, "[]", string(data))
}

func TestBEEFUnmarshalRoundTrip(t *testing.T) {
	original := BEEF{0, 1, 127, 255}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded BEEF
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestBEEFUnmarshalAcceptsBase64(t *testing.T) {
	var decoded BEEF
	require.NoError(t, json.Unmarshal([]byte(`"AAH/"`), &decoded))
	assert.Equal(t, BEEF{0, 1, 255}, decoded)
}

func TestBEEFUnmarshalRejectsOutOfRange(t *testing.T) {
	var decoded BEEF
	assert.Error(t, json.Unmarshal([]byte("[0,256]"), &decoded))
}

func TestBEEFUnmarshalNull(t *testing.T) {
	decoded := BEEF{1, 2, 3}
	require.NoError(t, json.Unmarshal([]byte("null"), &decoded))
	assert.Nil(t, decoded)
}

func TestBEEFUnmarshalRejectsInvalidBase64(t *testing.T) {
	var decoded BEEF
	assert.Error(t, json.Unmarshal([]byte(`"not!!base64"`), &decoded))
}
