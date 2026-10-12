package metrics

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const servicesMeterName = "github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"

var (
	servicesInitOnce sync.Once

	serviceCalls    metric.Int64Counter
	bumpLeafMissing metric.Int64Counter
)

func ensureServicesInstruments() {
	servicesInitOnce.Do(func() {
		meter := otel.Meter(servicesMeterName)
		serviceCalls, _ = meter.Int64Counter("wallet.services.calls_total",
			metric.WithDescription("Per-service call outcomes by method, service and outcome (ok, not_found, empty, canceled, error)"))
		bumpLeafMissing, _ = meter.Int64Counter("wallet.beef.bump_leaf_missing_total",
			metric.WithDescription("BUMP txid leaves that are not part of the BEEF (expected for trimmed BEEFs)"))
	})
}

// RecordServiceCall counts one service call. Outcome is "ok", "not_found", "empty" and
// "canceled" for expected non-failures, or "error" for real failures.
func RecordServiceCall(ctx context.Context, method, service, outcome string) {
	ensureServicesInstruments()
	if serviceCalls != nil {
		serviceCalls.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("service", service),
			attribute.String("outcome", outcome),
		))
	}
}

// RecordBumpLeafMissing counts one BUMP txid leaf that is absent from the BEEF.
func RecordBumpLeafMissing(ctx context.Context) {
	ensureServicesInstruments()
	if bumpLeafMissing != nil {
		bumpLeafMissing.Add(ctx, 1)
	}
}
