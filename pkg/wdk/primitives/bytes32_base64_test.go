package primitives_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TS-ecosystem short cert type: base64("CommonSource identity") = 21 bytes decoded.
const commonSourceIdentityB64 = "Q29tbW9uU291cmNlIGlkZW50aXR5"

func TestEncodeBytes32Base64_TrimsTrailingZeros(t *testing.T) {
	t.Parallel()

	// given: 21-byte type zero-padded into [32]byte (as CertificateType does)
	decoded, err := base64.StdEncoding.DecodeString(commonSourceIdentityB64)
	require.NoError(t, err)
	require.Len(t, decoded, 21)

	var padded [32]byte
	copy(padded[:], decoded)

	// when:
	got := primitives.EncodeBytes32Base64(padded)

	// then: must match original short base64, not the zero-padded re-encode
	require.Equal(t, commonSourceIdentityB64, got)

	paddedB64 := base64.StdEncoding.EncodeToString(padded[:])
	require.NotEqual(t, paddedB64, got, "naive EncodeToString of [32]byte must not equal trimmed form")
	require.Equal(t, "Q29tbW9uU291cmNlIGlkZW50aXR5AAAAAAAAAAAAAAA=", paddedB64)
	require.Equal(t, 0, len(got)%4, "EncodeBytes32Base64 output must be Base64String.Validate-safe")
}

func TestDecodeBytes32Base64_AcceptsShortBase64(t *testing.T) {
	t.Parallel()

	// when:
	got, err := primitives.DecodeBytes32Base64(commonSourceIdentityB64)

	// then:
	require.NoError(t, err)
	require.Equal(t, []byte("CommonSource identity"), bytes.TrimRight(got[:], "\x00"))
	require.Equal(t, byte(0), got[21])
	require.Equal(t, byte(0), got[31])
}

func TestBytes32Base64_ShortRoundTrip(t *testing.T) {
	t.Parallel()

	// when: short base64 → [32]byte → short base64
	arr, err := primitives.DecodeBytes32Base64(commonSourceIdentityB64)
	require.NoError(t, err)
	got := primitives.EncodeBytes32Base64(arr)

	// then: no zero-pad re-encode corruption
	require.Equal(t, commonSourceIdentityB64, got)
}

func TestBytes32Base64_Full32RoundTrip(t *testing.T) {
	t.Parallel()

	var full [32]byte
	for i := range full {
		full[i] = byte(i + 1) // no trailing zeros
	}
	encoded := primitives.EncodeBytes32Base64(full)
	decoded, err := primitives.DecodeBytes32Base64(encoded)
	require.NoError(t, err)
	require.Equal(t, full, decoded)
	require.Equal(t, base64.StdEncoding.EncodeToString(full[:]), encoded)
}

func TestDecodeBytes32Base64_RejectsTooLong(t *testing.T) {
	t.Parallel()

	tooLong := base64.StdEncoding.EncodeToString(make([]byte, 33))
	_, err := primitives.DecodeBytes32Base64(tooLong)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds 32")
}

func TestDecodeBytes32Base64_RejectsInvalidBase64(t *testing.T) {
	t.Parallel()

	_, err := primitives.DecodeBytes32Base64("not!valid")
	require.Error(t, err)
}

func TestDecodeBytes32Base64_EmptyStringYieldsZeroArray(t *testing.T) {
	t.Parallel()

	got, err := primitives.DecodeBytes32Base64("")
	require.NoError(t, err)
	require.Equal(t, [32]byte{}, got)
}

func TestEncodeBytes32Base64_AllZerosEncodesFullArray(t *testing.T) {
	t.Parallel()

	var zeros [32]byte
	got := primitives.EncodeBytes32Base64(zeros)
	require.Equal(t, base64.StdEncoding.EncodeToString(zeros[:]), got)
	require.NotEmpty(t, got)
}

func TestWalletCertificate_ToSDKCertificate_AcceptsShortTypeAndSerial(t *testing.T) {
	t.Parallel()

	// Storage layer may hold TS-ecosystem short base64 (fixtures use short forms too).
	shortSerial := "c2VyaWFsMTIz" // "serial123"
	cert := wdk.WalletCertificate{
		Type:               primitives.Base64String(commonSourceIdentityB64),
		SerialNumber:       primitives.Base64String(shortSerial),
		Subject:            "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
		Certifier:          "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
		RevocationOutpoint: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890.0",
		// 64-byte signature as 128 hex chars
		Signature: primitives.HexString(
			"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890" +
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		),
	}

	sdkCert, err := cert.ToSDKCertificate()
	require.NoError(t, err)
	require.Equal(t, commonSourceIdentityB64, primitives.EncodeBytes32Base64([32]byte(sdkCert.Type)))
	require.Equal(t, shortSerial, primitives.EncodeBytes32Base64([32]byte(sdkCert.SerialNumber)))
}

func TestListCertificatesArgs_JSON_AcceptsShortTypesAsBase64String(t *testing.T) {
	t.Parallel()

	// wdk storage args use opaque Base64String (not Bytes32Base64), so short types must work.
	raw := []byte(`{"certifiers":[],"types":["Q29tbW9uU291cmNlIGlkZW50aXR5"],"limit":100}`)
	var args wdk.ListCertificatesArgs
	require.NoError(t, json.Unmarshal(raw, &args))
	require.Len(t, args.Types, 1)
	require.Equal(t, primitives.Base64String(commonSourceIdentityB64), args.Types[0])
	require.NoError(t, args.Types[0].Validate())
}
