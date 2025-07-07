package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TableCertificateX(args *wdk.TableCertificateX) error {
	err := args.Type.Validate()
	if err != nil {
		return fmt.Errorf("invalid type argument: %w", err)
	}

	err = args.SerialNumber.Validate()
	if err != nil {
		return fmt.Errorf("invalid serialNumber argument: %w", err)
	}

	err = args.Certifier.Validate()
	if err != nil {
		return fmt.Errorf("invalid certifier argument: %w", err)
	}

	err = args.Subject.Validate()
	if err != nil {
		return fmt.Errorf("invalid subject argument: %w", err)
	}

	if args.Verifier != nil && *args.Verifier != "" {
		err = args.Verifier.Validate()
		if err != nil {
			return fmt.Errorf("invalid verifier argument: %w", err)
		}
	}

	err = args.RevocationOutpoint.Validate()
	if err != nil {
		return fmt.Errorf("invalid revocationOutpoint argument: %w", err)
	}

	err = args.Signature.Validate()
	if err != nil {
		return fmt.Errorf("invalid signature argument: %w", err)
	}

	for _, f := range args.Fields {
		err = f.MasterKey.Validate()
		if err != nil {
			return fmt.Errorf("invalid %s field masterKey argument: %w", f.FieldName, err)
		}
	}

	return nil
}
