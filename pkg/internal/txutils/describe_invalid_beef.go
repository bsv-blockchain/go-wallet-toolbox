package txutils

import (
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// maxReportedTxIDs keeps a rejection reason readable when a large graph fails
// wholesale. The count is always reported, so nothing is hidden by truncating
// the list.
const maxReportedTxIDs = 5

// DescribeInvalidBEEF explains why a BEEF failed verification.
//
// Verification answers only true or false, which leaves "provided beef is not
// valid" as a dead end in a log: the same message covers a missing ancestor, an
// unprovable transaction, and a proof whose root does not match. The validation
// pass already distinguishes those, so this recovers the distinction rather than
// making the next occurrence another investigation.
//
// Note that a transaction reported as notValid is usually not itself broken. It
// most often means one of its ancestors could not be proven — a txid-only entry
// for a transaction that is not yet mined, for instance, since only a merkle
// proof makes such an entry valid.
func DescribeInvalidBEEF(beef *transaction.Beef) string {
	if beef == nil {
		return "beef is nil"
	}

	result := beef.ValidateTransactions()

	var parts []string
	if len(result.NotValid) > 0 {
		parts = append(parts, fmt.Sprintf("notValid=%s", summariseTxIDs(result.NotValid)))
	}
	if len(result.MissingInputs) > 0 {
		parts = append(parts, fmt.Sprintf("missingInputs=%s", summariseTxIDs(result.MissingInputs)))
	}
	if len(result.WithMissingInputs) > 0 {
		parts = append(parts, fmt.Sprintf("withMissingInputs=%s", summariseTxIDs(result.WithMissingInputs)))
	}
	if len(result.TxidOnly) > 0 {
		parts = append(parts, fmt.Sprintf("txidOnly=%s", summariseTxIDs(result.TxidOnly)))
	}

	parts = append(parts, fmt.Sprintf("txs=%d bumps=%d valid=%d",
		len(beef.Transactions), len(beef.BUMPs), len(result.Valid)))

	if len(result.NotValid) == 0 && len(result.MissingInputs) == 0 && len(result.WithMissingInputs) == 0 {
		// Nothing structural, so the rejection came from the proofs themselves:
		// a BUMP whose leaves disagree on the root, or a bump index that does
		// not contain the transaction claiming it.
		parts = append(parts, "structure is complete, so a merkle proof did not check out")
	}

	return strings.Join(parts, " ")
}

func summariseTxIDs(txIDs []string) string {
	if len(txIDs) <= maxReportedTxIDs {
		return fmt.Sprintf("%d%v", len(txIDs), txIDs)
	}
	return fmt.Sprintf("%d%v...", len(txIDs), txIDs[:maxReportedTxIDs])
}
