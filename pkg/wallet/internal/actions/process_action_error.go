package actions

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/slices"

	pkgerrors "github.com/bsv-blockchain/go-wallet-toolbox/pkg/errors"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// newProcessActionError builds a ProcessActionError from process results, attaching optional
// recovery fields (Tx AtomicBEEF + noSendChange outpoints) when a new transaction is available.
// Matches TypeScript WERR_REVIEW_ACTIONS which carries txid, tx, sendWithResults,
// reviewActionResults, and noSendChange.
func newProcessActionError(
	processActionResult *wdk.ProcessActionResult,
	txID *chainhash.Hash,
	tx *assembler.AssembledTransaction,
	noSendChangeOutputVouts []int,
) *pkgerrors.ProcessActionError {
	processErr := pkgerrors.NewProcessActionError(
		processActionResult.SendWithResults,
		processActionResult.NotDelayedResults,
	)

	if txID != nil {
		var beef []byte
		if tx != nil {
			// Best-effort: if AtomicBEEF serialization fails, still attach the txid.
			if bytes, err := tx.AtomicBEEF(true); err == nil {
				beef = bytes
			}
		}
		processErr = processErr.WithTx(txID.String(), beef)
	}

	if txID != nil && len(noSendChangeOutputVouts) > 0 {
		if outpoints, err := mapping.MapIndexesToOutpoints(txID, noSendChangeOutputVouts); err == nil {
			processErr = processErr.WithNoSendChange(outpointsToStrings(outpoints))
		}
	}

	return processErr
}

func outpointsToStrings(outpoints []transaction.Outpoint) []string {
	return slices.Map(outpoints, func(op transaction.Outpoint) string {
		return string(primitives.NewOutpointString(op.Txid.String(), op.Index))
	})
}
