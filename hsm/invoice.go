package hsm

import (
	"fmt"
	"regexp"
	"strings"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// invoicePattern matches go-sdk KeyDeriver.computeInvoiceNumber. The function
// is unexported there, so this copy has to stay in lockstep: the equivalence
// tests fail if a derived public key diverges.
var invoicePattern = regexp.MustCompile(`^[a-z0-9 ]+$`)

func computeInvoiceNumber(protocol sdk.Protocol, keyID string) (string, error) {
	if protocol.SecurityLevel < 0 || protocol.SecurityLevel > 2 {
		return "", fmt.Errorf("protocol security level must be 0, 1, or 2")
	}
	if len(keyID) > 800 {
		return "", fmt.Errorf("key IDs must be 800 characters or less")
	}
	if len(keyID) < 1 {
		return "", fmt.Errorf("key IDs must be 1 character or more")
	}

	protocolName := strings.ToLower(strings.TrimSpace(protocol.Protocol))
	if len(protocolName) > 400 {
		if strings.HasPrefix(protocolName, "specific linkage revelation ") {
			if len(protocolName) > 430 {
				return "", fmt.Errorf("specific linkage revelation protocol names must be 430 characters or less")
			}
		} else {
			return "", fmt.Errorf("protocol names must be 400 characters or less")
		}
	}
	if len(protocolName) < 5 {
		return "", fmt.Errorf("protocol names must be 5 characters or more")
	}
	if strings.Contains(protocolName, "  ") {
		return "", fmt.Errorf("protocol names cannot contain multiple consecutive spaces (\"  \")")
	}
	if !invoicePattern.MatchString(protocolName) {
		return "", fmt.Errorf("protocol names can only contain letters, numbers and spaces")
	}
	if strings.HasSuffix(protocolName, " protocol") {
		return "", fmt.Errorf("no need to end your protocol name with \" protocol\"")
	}

	return fmt.Sprintf("%d-%s-%s", protocol.SecurityLevel, protocolName, keyID), nil
}
