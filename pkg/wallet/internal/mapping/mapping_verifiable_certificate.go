package mapping

import (
	"encoding/base64"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

func MapVerifiableCertificateToCertificate(cert certificates.VerifiableCertificate) (wallet.Certificate, error) {
	serialBytes, err := base64.StdEncoding.DecodeString(string(cert.SerialNumber))
	if err != nil {
		return wallet.Certificate{}, fmt.Errorf("failed to decode certificate serial number: %w", err)
	}

	var serial wallet.SerialNumber
	if len(serialBytes) != len(serial) {
		return wallet.Certificate{}, fmt.Errorf("serial bytes length: %d is not equal to wallet.SerialNumber bytes length: %d", len(serialBytes), len(serial))
	}

	copy(serial[:], serialBytes)

	certTypeBytes, err := base64.StdEncoding.DecodeString(string(cert.Type))
	if err != nil {
		return wallet.Certificate{}, fmt.Errorf("failed to decode certificate type: %w", err)
	}

	var certType wallet.CertificateType
	if len(certType) != len(certTypeBytes) {
		return wallet.Certificate{}, fmt.Errorf("certificate type bytes length: %d is not equal to wallet.CertificateType bytes length: %d", len(certTypeBytes), len(certType))
	}

	copy(certType[:], certTypeBytes)

	fields := make(map[string]string, len(cert.Fields))
	for k, v := range cert.Fields {
		fields[to.String(k)] = to.String(v)
	}

	signature, err := ec.ParseSignature(cert.Signature)
	if err != nil {
		return wallet.Certificate{}, fmt.Errorf("failed to parse signature: %w", err)
	}

	return wallet.Certificate{
		Type:               certType,
		SerialNumber:       serial,
		Subject:            to.Ptr(cert.Subject),
		Certifier:          to.Ptr(cert.Certifier),
		RevocationOutpoint: cert.RevocationOutpoint,
		Fields:             fields,
		Signature:          signature,
	}, nil
}
