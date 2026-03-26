package certifier

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	script "github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	walletcerts "github.com/bsv-blockchain/go-wallet-toolbox/pkg/certificates"
)

// CertificateService handles certificate signing operations.
type CertificateService struct {
	wallet sdk.Interface
	config *ServerConfig
}

// NewCertificateService creates a new certificate service with the given wallet and configuration.
func NewCertificateService(wallet sdk.Interface, cfg *ServerConfig) *CertificateService {
	return &CertificateService{
		wallet: wallet,
		config: cfg,
	}
}

// SignCertificate processes a certificate issuance request and returns a signed certificate.
func (s *CertificateService) SignCertificate(
	ctx context.Context,
	req *walletcerts.ProtocolIssuanceRequest,
	clientPubKey *ec.PublicKey,
) (*walletcerts.ProtocolIssuanceResponse, error) {
	serverNonce, err := walletcerts.CreateNonce(ctx, s.wallet, s.config.Randomizer, clientPubKey, s.config.Originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create server nonce: %w", err)
	}

	decodedClientNonce, err := base64.StdEncoding.DecodeString(req.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode client nonce: %w", err)
	}

	decodedSrvNonce, err := base64.StdEncoding.DecodeString(serverNonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode server nonce: %w", err)
	}

	hmac, err := s.wallet.CreateHMAC(ctx, sdk.CreateHMACArgs{
		EncryptionArgs: sdk.EncryptionArgs{
			ProtocolID: sdk.Protocol{
				SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
				Protocol:      "certificate issuance",
			},
			KeyID: serverNonce + req.Nonce,
			Counterparty: sdk.Counterparty{
				Type:         sdk.CounterpartyTypeOther,
				Counterparty: clientPubKey,
			},
		},
		Data: append(decodedClientNonce, decodedSrvNonce...),
	}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create server hmac: %w", err)
	}

	certifierKey, err := s.wallet.GetPublicKey(ctx, sdk.GetPublicKeyArgs{IdentityKey: true}, s.config.Originator)
	if err != nil {
		return nil, fmt.Errorf("failed to get server wallet public key: %w", err)
	}

	serialNumber := base64.StdEncoding.EncodeToString(hmac.HMAC[:])
	certFields, err := walletcerts.MapToCertificateFields(req.Fields)
	if err != nil {
		return nil, fmt.Errorf("failed to map certificate fields: %w", err)
	}

	hashOfSerialNumber := sha256.Sum256([]byte(serialNumber))
	hashHex := hex.EncodeToString(hashOfSerialNumber[:])

	lockingScript, err := script.NewFromASM("OP_SHA256 " + hashHex + " OP_EQUAL")
	if err != nil {
		return nil, fmt.Errorf("failed to create locking script: %w", err)
	}

	customInstr, err := json.Marshal(map[string]string{
		"serialNumber": serialNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom instructions: %w", err)
	}

	revocationOutpoint, err := s.wallet.CreateAction(ctx,
		sdk.CreateActionArgs{
			Description: "Certificate revocation",
			Outputs: []sdk.CreateActionOutput{{
				OutputDescription:  "Certificate revocation outpoint",
				Satoshis:           1,
				LockingScript:      lockingScript.Bytes(),
				Basket:             "certificate revocation",
				CustomInstructions: string(customInstr), // the unlockingScript is just the serialNumber
			}},
			Options: &sdk.CreateActionOptions{
				RandomizeOutputs: to.Ptr(false), // this ensures the output is always at the same position at outputIndex 0
			},
		},
		s.config.Originator,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create revocation outpoint: %w", err)
	}

	signedCertificate := certificates.NewCertificate(
		sdk.StringBase64(req.Type),
		sdk.StringBase64(serialNumber),
		*clientPubKey,
		*certifierKey.PublicKey,
		&transaction.Outpoint{
			Txid:  revocationOutpoint.Txid,
			Index: 0,
		},
		certFields,
		nil)
	err = signedCertificate.Sign(ctx, s.wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	// Mock response with a certificate
	response := &walletcerts.ProtocolIssuanceResponse{
		ServerNonce: serverNonce,
		Certificate: &walletcerts.Certificate{
			Type:               string(signedCertificate.Type),
			SerialNumber:       string(signedCertificate.SerialNumber),
			Subject:            signedCertificate.Subject.ToDERHex(),
			Certifier:          signedCertificate.Certifier.ToDERHex(),
			RevocationOutpoint: signedCertificate.RevocationOutpoint.String(),
			Fields:             s.convertFieldsToString(signedCertificate.Fields),
			Signature:          hex.EncodeToString(signedCertificate.Signature),
		},
	}

	return response, nil
}

func (s *CertificateService) convertFieldsToString(fields map[sdk.CertificateFieldNameUnder50Bytes]sdk.StringBase64) map[string]string {
	stringFields := make(map[string]string)
	for k, v := range fields {
		stringFields[string(k)] = string(v)
	}
	return stringFields
}
