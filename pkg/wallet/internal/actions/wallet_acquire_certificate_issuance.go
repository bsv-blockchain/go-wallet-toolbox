package actions

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bsv-blockchain/go-sdk/auth/brc104"
	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	walletcerts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/certificates"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// PrepareIssuanceActionDataParams contains parameters for preparing the certificate issuance request
type PrepareIssuanceActionDataParams struct {
	Wallet      sdk.Interface
	Args        sdk.AcquireCertificateArgs
	Nonce       string
	IdentityKey *ec.PublicKey
}

// PrepareIssuanceActionDataResult contains the prepared payload and related data
type PrepareIssuanceActionDataResult struct {
	Body          []byte
	Fields        map[string]string
	MasterKeyring map[string]string
	CertTypeB64   string
	CounterParty  sdk.Counterparty
}

// ParseCertificateResponseParams contains parameters for parsing the certificate response
type ParseCertificateResponseParams struct {
	Response    *http.Response
	Args        sdk.AcquireCertificateArgs
	IdentityKey *ec.PublicKey
}

// ParseCertificateResponseResult contains the parsed and validated certificate
type ParseCertificateResponseResult struct {
	Certificate        *certificates.Certificate
	ParsedCertifier    *ec.PublicKey
	RevocationOutpoint *transaction.Outpoint
	ParsedSignature    *ec.Signature
	SerialNumber       []byte
	ServerNonce        string
	CertFields         map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64
}

// StoreCertificateParams contains parameters for storing the certificate
type StoreCertificateParams struct {
	Storage            AcquireCertificateIssuanceStorage
	Auth               wdk.AuthID
	Certificate        *certificates.Certificate
	Certifier          *ec.PublicKey
	RevocationOutpoint *transaction.Outpoint
	Signature          *ec.Signature
	IdentityKey        *ec.PublicKey
	CertTypeB64        string
	Fields             map[string]string
	MasterKeyring      map[string]string
}

// AcquireCertificateIssuanceStorage defines the storage methods needed for certificate issuance
type AcquireCertificateIssuanceStorage interface {
	GetAuth(ctx context.Context) (wdk.AuthID, error)
	InsertCertificateAuth(ctx context.Context, cert *wdk.TableCertificateX) (uint, error)
}

// PrepareIssuanceActionData creates the certificate signing request payload along with certificate data
func PrepareIssuanceActionData(ctx context.Context, p PrepareIssuanceActionDataParams) (*PrepareIssuanceActionDataResult, error) {
	// Prepare counterparty for encryption
	counterParty := sdk.Counterparty{
		Counterparty: p.Args.Certifier,
		Type:         sdk.CounterpartyTypeOther,
	}

	// Create encrypted certificate fields
	fieldsForEncryption, err := mapping.MapToFieldsForEncryption(p.Args.Fields)
	if err != nil {
		return nil, fmt.Errorf("failed to map fields to fields for encryption: %w", err)
	}

	certificateFieldsResult, err := certificates.CreateCertificateFields(ctx, p.Wallet, counterParty, fieldsForEncryption, to.Value(p.Args.Privileged), p.Args.PrivilegedReason)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate fields: %w", err)
	}

	fields := make(map[string]string, len(certificateFieldsResult.CertificateFields))
	for k, v := range certificateFieldsResult.CertificateFields {
		fields[to.String(k)] = to.String(v)
	}

	masterKeyring := make(map[string]string, len(certificateFieldsResult.MasterKeyring))
	for k, v := range certificateFieldsResult.MasterKeyring {
		masterKeyring[to.String(k)] = to.String(v)
	}

	// Build certificate signing request
	certTypeB64 := base64.StdEncoding.EncodeToString(p.Args.Type[:])
	body, err := json.Marshal(&walletcerts.ProtocolIssuanceRequest{
		Type:          certTypeB64,
		Nonce:         p.Nonce,
		Fields:        fields,
		MasterKeyring: masterKeyring,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal HTTP request payload: %w", err)
	}

	return &PrepareIssuanceActionDataResult{
		Body:          body,
		Fields:        fields,
		MasterKeyring: masterKeyring,
		CertTypeB64:   certTypeB64,
		CounterParty:  counterParty,
	}, nil
}

// ParseCertificateResponse parses and validates the certificate response from the certifier
func ParseCertificateResponse(p ParseCertificateResponseParams) (*ParseCertificateResponseResult, error) {
	// Read response body
	responseBytes, err := io.ReadAll(p.Response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if p.Response.StatusCode < 200 || p.Response.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status code %d: %s", p.Response.StatusCode, string(responseBytes))
	}

	var response walletcerts.ProtocolIssuanceResponse
	if err = json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Validate response headers and required fields
	responseAuthHeader := p.Response.Header.Get(brc104.HeaderIdentityKey)
	if responseAuthHeader != p.Args.Certifier.ToDERHex() {
		return nil, fmt.Errorf("invalid certifier! Expected: %s, Received: %s", p.Args.Certifier.ToDERHex(), responseAuthHeader)
	}

	if response.ServerNonce == "" {
		return nil, fmt.Errorf("no serverNonce received from certifier")
	}

	if response.Certificate == nil {
		return nil, fmt.Errorf("no certificate received from certifier")
	}

	// Parse certificate components
	subject, err := ec.PublicKeyFromString(response.Certificate.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject field to public key: %w", err)
	}

	certifier, err := ec.PublicKeyFromString(response.Certificate.Certifier)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certifier field to public key: %w", err)
	}

	revocationOutpoint, err := transaction.OutpointFromString(response.Certificate.RevocationOutpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse revocation outpoint: %w", err)
	}

	signatureBytes, err := hex.DecodeString(response.Certificate.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature from hex: %w", err)
	}

	parsedSignature, err := ec.ParseSignature(signatureBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signature: %w", err)
	}

	certFields, err := walletcerts.MapToCertificateFields(response.Certificate.Fields)
	if err != nil {
		return nil, fmt.Errorf("failed to map certificate fields: %w", err)
	}

	signedCert := certificates.NewCertificate(
		sdk.StringBase64(response.Certificate.Type),
		sdk.StringBase64(response.Certificate.SerialNumber),
		to.Value(subject),
		to.Value(certifier),
		revocationOutpoint,
		certFields,
		signatureBytes)

	serialNumber, err := base64.StdEncoding.DecodeString(string(signedCert.SerialNumber))
	if err != nil {
		return nil, fmt.Errorf("failed to decode serialNumber: %w", err)
	}

	return &ParseCertificateResponseResult{
		Certificate:        signedCert,
		ParsedCertifier:    certifier,
		RevocationOutpoint: revocationOutpoint,
		ParsedSignature:    parsedSignature,
		SerialNumber:       serialNumber,
		ServerNonce:        response.ServerNonce,
		CertFields:         certFields,
	}, nil
}

// VerifyCertificateIssuance verifies the certificate against the original request parameters
func VerifyCertificateIssuance(ctx context.Context, wallet sdk.Interface, parsedCert *ParseCertificateResponseResult, nonce string, issuanceActionData *PrepareIssuanceActionDataResult, subject, certifier *ec.PublicKey, originator string) error {
	// Verify if serial number has correct length
	if len(parsedCert.SerialNumber) != walletcerts.NonceHMACSize {
		return fmt.Errorf("invalid serialNumber length: got %d, want %d", len(parsedCert.SerialNumber), walletcerts.NonceHMACSize)
	}

	// Decode both nonces from base64 and concatenate the raw bytes
	// TypeScript does: Utils.toArray(clientNonce + serverNonce, 'base64')
	// which decodes the concatenated base64 strings to bytes
	clientNonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return fmt.Errorf("failed to decode client nonce: %w", err)
	}
	serverNonceBytes, err := base64.StdEncoding.DecodeString(parsedCert.ServerNonce)
	if err != nil {
		return fmt.Errorf("failed to decode server nonce: %w", err)
	}
	dataToVerify := append(clientNonceBytes, serverNonceBytes...)
	var hmacToVerifyArray [32]byte
	copy(hmacToVerifyArray[:], parsedCert.SerialNumber)

	verifyHmacResult, err := wallet.VerifyHMAC(ctx, sdk.VerifyHMACArgs{
		HMAC: hmacToVerifyArray,
		Data: dataToVerify,
		EncryptionArgs: sdk.EncryptionArgs{
			KeyID: parsedCert.ServerNonce + nonce,
			ProtocolID: sdk.Protocol{
				SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
				Protocol:      "certificate issuance",
			},
			Counterparty: issuanceActionData.CounterParty,
		},
	}, originator)
	if err != nil {
		return fmt.Errorf("failed to verify HMAC signature: %w", err)
	}
	if !verifyHmacResult.Valid {
		return fmt.Errorf("invalid serialNumber")
	}

	// Validate certificate type
	if string(parsedCert.Certificate.Type) != issuanceActionData.CertTypeB64 {
		return fmt.Errorf("invalid certificate type! Expected: %s, Received: %s", issuanceActionData.CertTypeB64, parsedCert.Certificate.Type)
	}

	// Validate certificate subject matches our identity key
	if parsedCert.Certificate.Subject.ToDERHex() != subject.ToDERHex() {
		return fmt.Errorf("invalid certificate subject! Expected: %s, Received: %s", subject.ToDERHex(), parsedCert.Certificate.Subject.ToDERHex())
	}

	// Validate certifier
	if parsedCert.Certificate.Certifier.ToDERHex() != certifier.ToDERHex() {
		return fmt.Errorf("invalid certifier! Expected: %s, Received: %s", certifier.ToDERHex(), parsedCert.Certificate.Certifier.ToDERHex())
	}

	// Validate revocation outpoint exists
	if parsedCert.Certificate.RevocationOutpoint == nil {
		return fmt.Errorf("invalid revocationOutpoint")
	}

	// Validate that certificate fields match what we sent
	if len(parsedCert.Certificate.Fields) != len(issuanceActionData.Fields) {
		return fmt.Errorf("fields mismatch! Objects have different number of keys. Expected: %d, Received: %d", len(issuanceActionData.Fields), len(parsedCert.Certificate.Fields))
	}

	for fieldName, fieldValue := range issuanceActionData.Fields {
		signedCertFieldValue, isPresent := parsedCert.Certificate.Fields[sdk.CertificateFieldNameUnder50Bytes(fieldName)]
		if !isPresent {
			return fmt.Errorf("missing field: %s in certificate fields from the certifier", fieldName)
		}

		if string(signedCertFieldValue) != fieldValue {
			return fmt.Errorf("invalid field! Expected: %s, Received: %s", fieldValue, string(signedCertFieldValue))
		}
	}

	// Verify certificate signature
	if err := parsedCert.Certificate.Verify(ctx); err != nil {
		return fmt.Errorf("failed to verify certificate: %w", err)
	}

	return nil
}

// TestCertificateDecryption verifies that the certificate fields can be decrypted
func TestCertificateDecryption(ctx context.Context, wallet sdk.Interface, certFields map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64, masterKeyring map[string]string, counterParty sdk.Counterparty, args sdk.AcquireCertificateArgs) error {
	certMasterKeyring, err := walletcerts.MapToCertificateFields(masterKeyring)
	if err != nil {
		return fmt.Errorf("failed to map certificate master keyring fields: %w", err)
	}

	_, err = certificates.DecryptFields(ctx, wallet,
		certMasterKeyring,
		certFields,
		counterParty,
		to.Value(args.Privileged),
		args.PrivilegedReason,
	)
	if err != nil {
		return fmt.Errorf("failed to decrypt certificate: %w", err)
	}

	return nil
}

// StoreCertificate saves the certificate to storage
func StoreCertificate(ctx context.Context, p StoreCertificateParams) error {
	// Convert signature to hex string for storage
	rHex := fmt.Sprintf("%064x", p.Signature.R)
	sHex := fmt.Sprintf("%064x", p.Signature.S)
	sigHex := rHex + sHex

	// Parse fields into TableCertificateField slice
	certificateFields, err := wdk.ParseToTableCertificateFieldSlice(to.Value(p.Auth.UserID), p.Fields, p.MasterKeyring)
	if err != nil {
		return fmt.Errorf("failed to parse certificate fields for user %d: %w", to.Value(p.Auth.UserID), err)
	}

	// Insert certificate into storage
	certifierPubKeyHex := primitives.PubKeyHex(p.Certifier.ToDERHex())
	_, err = p.Storage.InsertCertificateAuth(ctx, &wdk.TableCertificateX{
		TableCertificate: wdk.TableCertificate{
			UserID:             to.Value(p.Auth.UserID),
			Type:               primitives.Base64String(p.CertTypeB64),
			SerialNumber:       primitives.Base64String(p.Certificate.SerialNumber),
			Certifier:          certifierPubKeyHex,
			Subject:            primitives.PubKeyHex(p.IdentityKey.ToDERHex()),
			RevocationOutpoint: primitives.OutpointString(p.RevocationOutpoint.String()),
			Signature:          primitives.HexString(sigHex),
			Verifier:           to.Ptr(certifierPubKeyHex), // Certifier is the verifier (KeyringRevealer.Certifier is true)
		},
		Fields: certificateFields,
	})
	if err != nil {
		return fmt.Errorf("failed to insert certificate for user %d: %w", to.Value(p.Auth.UserID), err)
	}

	return nil
}
