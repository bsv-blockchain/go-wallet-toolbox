package metrics

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/telemetry"
)

// Results of a manual merkle-path check made by check_for_proofs.
const (
	ProofCheckFound    = "found"
	ProofCheckNotFound = "not_found"
	ProofCheckError    = "error"
)

// Sources of an applied merkle proof.
const (
	ProofSourceSSE    = "sse"
	ProofSourceManual = "manual"
)

var (
	proofsInitOnce sync.Once

	overdueProofTxs   metric.Int64Gauge
	manualProofChecks metric.Int64Counter
	proofsApplied     metric.Int64Counter
)

func ensureProofInstruments() {
	proofsInitOnce.Do(func() {
		meter := telemetry.MeterProvider().Meter(storageMeterName)
		overdueProofTxs, _ = meter.Int64Gauge("wallet.proofs.overdue_txs",
			metric.WithUnit("{transaction}"),
			metric.WithDescription("Unproven transactions old enough (proof_check_after_blocks) for a manual merkle-path check in the last check_for_proofs run"))
		manualProofChecks, _ = meter.Int64Counter("wallet.proofs.manual_checks_total",
			metric.WithDescription("Manual merkle-path checks made by check_for_proofs, by result (found | not_found | error)"))
		proofsApplied, _ = meter.Int64Counter("wallet.proofs.applied_total",
			metric.WithDescription("Merkle proofs applied to transactions, by source (sse | manual)"))
	})
}

// RecordOverdueProofTxs records how many transactions the last check_for_proofs run
// found overdue for a manual merkle-path check.
func RecordOverdueProofTxs(ctx context.Context, count int) {
	ensureProofInstruments()
	if overdueProofTxs != nil {
		overdueProofTxs.Record(ctx, int64(count))
	}
}

// RecordManualProofCheck counts one manual merkle-path check by result.
func RecordManualProofCheck(ctx context.Context, result string) {
	ensureProofInstruments()
	if manualProofChecks != nil {
		manualProofChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	}
}

// RecordProofApplied counts one merkle proof applied to a transaction, by source.
func RecordProofApplied(ctx context.Context, source string) {
	ensureProofInstruments()
	if proofsApplied != nil {
		proofsApplied.Add(ctx, 1, metric.WithAttributes(attribute.String("source", source)))
	}
}
