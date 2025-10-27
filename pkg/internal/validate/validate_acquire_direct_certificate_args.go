package validate

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

func ValidateAcquireDirectCertificateArgs(args *sdk.AcquireCertificateArgs) error {
	if args.AcquisitionProtocol != sdk.AcquisitionProtocolDirect {
		return fmt.Errorf("only acquire direct certificate requests allowed here")
	}
	if args.Certifier == nil {
		return fmt.Errorf("certifier: invalid when acquisitionProtocol is 'direct'")
	}
	if args.SerialNumber == nil {
		return fmt.Errorf("serialNumber: invalid when acquisitionProtocol is 'direct'")
	}
	if args.Signature == nil {
		return fmt.Errorf("signature: invalid when acquisitionProtocol is 'direct'")
	}
	if args.RevocationOutpoint == nil {
		return fmt.Errorf("revocationOutpoint: invalid when acquisitionProtocol is 'direct'")
	}
	if args.KeyringRevealer == nil {
		return fmt.Errorf("keyringRevealer: invalid when acquisitionProtocol is 'direct'")
	}
	if args.KeyringForSubject == nil {
		return fmt.Errorf("keyringForSubject: invalid when acquisitionProtocol is 'direct'")
	}
	if to.Value(args.Privileged) && args.PrivilegedReason == "" {
		return fmt.Errorf("privilegedReason: invalid when 'privileged' is true")
	}

	return nil
}
