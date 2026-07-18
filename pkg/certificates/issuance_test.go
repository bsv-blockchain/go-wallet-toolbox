package certificates

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapToCertificateFieldsValidatesBase64Values(t *testing.T) {
	fields, err := MapToCertificateFields(map[string]string{
		"name": base64.StdEncoding.EncodeToString([]byte("Alice")),
	})

	require.NoError(t, err)
	require.Len(t, fields, 1)
}

func TestMapToCertificateFieldsRejectsInvalidBase64Value(t *testing.T) {
	_, err := MapToCertificateFields(map[string]string{
		"name": "not base64",
	})

	require.ErrorContains(t, err, "must be base64 encoded")
}

func TestMapToCertificateFieldsRejectsOversizedDecodedValue(t *testing.T) {
	value := base64.StdEncoding.EncodeToString(make([]byte, MaxCertificateFieldDecodedBytes+1))

	_, err := MapToCertificateFields(map[string]string{
		"name": value,
	})

	require.ErrorContains(t, err, "decoded value must not exceed")
}
