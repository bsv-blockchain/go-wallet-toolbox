package txutils

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/is"
)

// HydrateBEEF hydrates the source transactions for each input using the transactions stored in the BEEF.
// This is necessary for operations like script verification that need access to the source transactions'
// outputs.
func HydrateBEEF(beef *transaction.Beef) error {
	for txIDHash, beefTx := range beef.Transactions {
		if beefTx.Transaction == nil {
			continue
		}
		// skip already mined txs - their inputs don't need hydration
		if beefTx.Transaction.MerklePath != nil {
			continue
		}

		for _, input := range beefTx.Transaction.Inputs {
			if input.SourceTXID == nil {
				continue
			}
			err := hydrateInput(input, beef, 0)
			if err != nil {
				return fmt.Errorf("failed to hydrate input %s of tx %s: %w", input.SourceTXID.String(), txIDHash.String(), err)
			}
		}
	}
	return nil
}

// HydrateBEEFSubjects attaches SourceTransaction to each input of the listed
// transactions only, without recursing into the parents' own inputs. Use with
// direct-sources-only BEEFs (entity.WithDirectSourcesOnly) where grandparents
// are intentionally absent: script verification and EF construction need only
// the direct source outputs.
func HydrateBEEFSubjects(beef *transaction.Beef, txIDs []string) error {
	for _, txID := range txIDs {
		tx := beef.FindTransaction(txID)
		if tx == nil {
			return fmt.Errorf("could not find subject transaction %s in beef", txID)
		}
		for _, input := range tx.Inputs {
			if input.SourceTXID == nil || input.SourceTransaction != nil {
				continue
			}
			src, ok := beef.Transactions[*input.SourceTXID]
			if !ok || src.Transaction == nil {
				return fmt.Errorf("failed to hydrate input %s of tx %s: could not find transaction %s in beef",
					input.SourceTXID.String(), txID, input.SourceTXID.String())
			}
			input.SourceTransaction = src.Transaction
		}
	}
	return nil
}

func hydrateInput(input *transaction.TransactionInput, beef *transaction.Beef, depth int) error {
	txID := input.SourceTXID.String()
	if depth > 100 {
		return fmt.Errorf("could not hydrate the input %s: too many recursions", txID)
	}
	if input.SourceTransaction != nil {
		return nil
	}

	tx, ok := beef.Transactions[*input.SourceTXID]
	if !ok {
		return fmt.Errorf("could not find transaction %s in beef", txID)
	}
	if tx.Transaction == nil {
		// A TxIDOnly entry carries no raw transaction. Verification accepts one
		// (allowTxidOnly) whenever a BUMP covers it, so this is reachable, and
		// walking into its inputs would dereference nil. There is nothing to
		// hydrate from a bare txid: leave the input alone. Script verification
		// skips inputs without a source transaction.
		return nil
	}
	input.SourceTransaction = tx.Transaction
	if tx.DataFormat == transaction.RawTxAndBumpIndex {
		if !is.Between(tx.BumpIndex, 0, len(beef.BUMPs)-1) {
			return fmt.Errorf("cannot find bump with index %d for tx %s", tx.BumpIndex, txID)
		}
		input.SourceTransaction.MerklePath = beef.BUMPs[tx.BumpIndex]
		return nil
	}
	for _, source := range input.SourceTransaction.Inputs {
		err := hydrateInput(source, beef, depth+1)
		if err != nil {
			return err
		}
	}
	return nil
}
