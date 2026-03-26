package nosendtest

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type NosendAssertion interface {
	ProcessedSuccessfully(processActionResult *wdk.ProcessActionResult, additionalTxIDs ...string) NosendAssertion
	ProcessedWithServiceError(processActionResult *wdk.ProcessActionResult, additionalTxIDs ...string) NosendAssertion
	Funds() testabilities.FundsAssertion
}

type nosendAssertion struct {
	testing.TB

	act *noSendAct
}

func (a *nosendAssertion) ProcessedSuccessfully(processActionResult *wdk.ProcessActionResult, additionalTxIDs ...string) NosendAssertion {
	txIDs := append(a.act.NoSendTxs(), additionalTxIDs...)

	testabilities.NotDelayedResultsAsserter(processActionResult.NotDelayedResults).
		ContainsTxsWithStatus(a, wdk.ReviewActionResultStatusSuccess, txIDs...)

	testabilities.SendWithResultsAsserter(processActionResult.SendWithResults).
		ContainsTxsWithStatus(a, wdk.SendWithResultStatusUnproven, txIDs...)

	testabilities.
		ThenDBState(a, a.act.activeProvider).
		HasUserTransactionsByTxIDsWithStatus(a.act.user, wdk.TxStatusUnproven, txIDs...)

	return a
}

func (a *nosendAssertion) ProcessedWithServiceError(processActionResult *wdk.ProcessActionResult, additionalTxIDs ...string) NosendAssertion {
	txIDs := append(a.act.NoSendTxs(), additionalTxIDs...)

	testabilities.NotDelayedResultsAsserter(processActionResult.NotDelayedResults).
		ContainsTxsWithStatus(a, wdk.ReviewActionResultStatusServiceError, txIDs...)

	testabilities.SendWithResultsAsserter(processActionResult.SendWithResults).
		ContainsTxsWithStatus(a, wdk.SendWithResultStatusSending, txIDs...)

	testabilities.
		ThenDBState(a, a.act.activeProvider).
		HasUserTransactionsByTxIDsWithStatus(a.act.user, wdk.TxStatusSending, txIDs...)

	return a
}

func (a *nosendAssertion) Funds() testabilities.FundsAssertion {
	return testabilities.ThenFunds(a, a.act.user, a.act.activeProvider)
}
