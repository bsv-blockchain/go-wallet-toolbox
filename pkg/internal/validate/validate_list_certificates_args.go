package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ListCertificatesArgs(args *wdk.ListCertificatesArgs) error {
	for _, c := range args.Certifiers {
		err := c.Validate()
		if err != nil {
			return fmt.Errorf("invalid certifier argument: %w", err)
		}
	}

	for _, t := range args.Types {
		err := t.Validate()
		if err != nil {
			return fmt.Errorf("invalid type argument: %w", err)
		}
	}

	err := args.Limit.Validate()
	if err != nil {
		return fmt.Errorf("invalid type argument: %w", err)
	}

	if err := validateListCertificatesPartialArgs(&args.ListCertificatesArgsPartial); err != nil {
		return fmt.Errorf("invalid partial argument: %w", err)
	}

	return nil
}

func validateListCertificatesPartialArgs(args *wdk.ListCertificatesArgsPartial) error {
	if args.SerialNumber != nil {
		err := args.SerialNumber.Validate()
		if err != nil {
			return fmt.Errorf("invalid partial serialNumber argument: %w", err)
		}
	}

	if args.RevocationOutpoint != nil {
		err := args.RevocationOutpoint.Validate()
		if err != nil {
			return fmt.Errorf("invalid partial revocationOutpoint argument: %w", err)
		}
	}

	if args.Signature != nil {
		err := args.Signature.Validate()
		if err != nil {
			return fmt.Errorf("invalid partial signature argument: %w", err)
		}
	}

	if args.Subject != nil {
		err := args.Subject.Validate()
		if err != nil {
			return fmt.Errorf("invalid partial subject argument: %w", err)
		}
	}

	return nil
}
