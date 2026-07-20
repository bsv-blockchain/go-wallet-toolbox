package mapping

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func MapVerifiableCertificateToCertificate(cert certificates.VerifiableCertificate) (wallet.Certificate, error) {
	serial, err := primitives.DecodeBytes32Base64(string(cert.SerialNumber))
	if err != nil {
		return wallet.Certificate{}, fmt.Errorf("failed to decode certificate serial number: %w", err)
	}

	certType, err := primitives.DecodeBytes32Base64(string(cert.Type))
	if err != nil {
		return wallet.Certificate{}, fmt.Errorf("failed to decode certificate type: %w", err)
	}

	fields := make(map[string]string, len(cert.Fields))
	for k, v := range cert.Fields {
		fields[to.String(k)] = to.String(v)
	}

	signature, err := ec.ParseSignature(cert.Signature)
	if err != nil {
		return wallet.Certificate{}, fmt.Errorf("failed to parse signature: %w", err)
	}

	return wallet.Certificate{
		Type:               wallet.CertificateType(certType),
		SerialNumber:       wallet.SerialNumber(serial),
		Subject:            to.Ptr(cert.Subject),
		Certifier:          to.Ptr(cert.Certifier),
		RevocationOutpoint: cert.RevocationOutpoint,
		Fields:             fields,
		Signature:          signature,
	}, nil
}
