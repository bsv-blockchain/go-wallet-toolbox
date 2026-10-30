// Package metrics defines the wallet-side OpenTelemetry instruments: the
// storage-side instruments of the throughput UTXO-management strategy
// (proposal §5.4) and the chaintracks chain-state instruments. All instruments
// are registered against telemetry.MeterProvider: they are no-ops until a host
// supplies one or the process calls tracing.EnableMetrics, so the privacy strategy and
// unconfigured deployments pay nothing. Thresholds and paging are an external
// concern — the wallet only emits telemetry.
package metrics
