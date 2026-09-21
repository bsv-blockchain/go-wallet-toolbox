// Package metrics defines the wallet-side OpenTelemetry instruments: the
// storage-side instruments of the throughput UTXO-management strategy
// (proposal §5.4) and the chaintracks chain-state instruments. All instruments
// are registered against the global meter: they are no-ops until the process
// enables a MeterProvider (tracing.EnableMetrics), so the privacy strategy and
// unconfigured deployments pay nothing. Thresholds and paging are an external
// concern — the wallet only emits telemetry.
package metrics
