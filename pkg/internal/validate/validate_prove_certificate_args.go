package validate

import (
	"encoding/base64"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func ProveCertificateArgs(args sdk.ProveCertificateArgs) error {
	cert := args.Certificate
	if cert.Signature == nil {
		return fmt.Errorf("invalid certificate signature: value cannot be a nil value")
	}

	if len(cert.Type) != 0 {
		str := primitives.Base64String(cert.Type.Base64())
		if err := str.Validate(); err != nil {
			return fmt.Errorf("invalid certificate type: base64 encoded validation check: %w", err)
		}
	}

	if len(cert.SerialNumber) != 0 {
		str := primitives.Base64String(base64.StdEncoding.EncodeToString(cert.SerialNumber[:]))
		if err := str.Validate(); err != nil {
			return fmt.Errorf("invalid certificate serial number: base64 encoded validation check: %w", err)
		}
	}

	if cert.Certifier != nil && !cert.Certifier.Validate() {
		return fmt.Errorf("invalid certificate certifier: failed validation check")
	}

	if cert.RevocationOutpoint != nil && len(cert.RevocationOutpoint.String()) != 0 {
		outpoint := primitives.OutpointString(cert.RevocationOutpoint.String())
		if err := outpoint.Validate(); err != nil {
			return fmt.Errorf("invalid certificate certifier revocation outpoint: failed validation check")
		}
	}

	if cert.Signature != nil {
		rHex := fmt.Sprintf("%064x", cert.Signature.R)
		sHex := fmt.Sprintf("%064x", cert.Signature.S)

		hex := primitives.HexString(rHex + sHex)
		if err := hex.Validate(); err != nil {
			return fmt.Errorf("invalid certificate signature: failed validation check")
		}
	}

	if args.Verifier != nil {
		hex := primitives.PubKeyHex(args.Verifier.ToDERHex())
		if err := hex.Validate(); err != nil {
			return fmt.Errorf("invalid verifier: failed validation check")
		}
	}

	const (
		minPrivilegedReasonLength = 5
		maxPrivilegedReasonLength = 50
	)

	if len(args.PrivilegedReason) != 0 && (len(args.PrivilegedReason) < minPrivilegedReasonLength || len(args.PrivilegedReason) > maxPrivilegedReasonLength) {
		return fmt.Errorf("invalid privileged reason length: reason string length must be at least 5 characters, maximum 50")
	}

	return nil
}
