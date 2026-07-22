# Task 4 Report — Web UI polish

## Status
**done**

## Commits
- `feat(dashboard): polish throughput demo web UI` (web-only under `cmd/throughput_dashboard/web/**`)

## Summary of UI changes

### Controls (start/stop)
- TPS / workers inputs with bounds, numeric `inputmode`, disabled while stream is running
- Busy labels (“Starting…”, “Stopping…”) and inline stream status (ok/err/info) instead of only `alert`
- Stream state shown as a running/stopped pill; counters keep tabular mono formatting

### Charts
- Chart.js line charts for TPS succeeded/failed and fuel/reserve inventory
- Soft fills, tooltips, reduced motion-friendly static updates
- If Chart.js CDN fails: fallback messages; gauges still update via status/SSE

### Balances & network
- Balances / runway / water marks from `/api/status` and SSE `tick`
- Mainnet warning banner + badge; non-mainnet hides banner and shows network badge as OK
- Live SSE connection chip (Connecting / Live / Reconnecting / offline) with poll backup every 5s
- Deduped tick application so poll + SSE do not double-plot chart points

### Top-ups
- SSE `topup` event log with basket tags, timestamps, empty state, capped list, dedupe keys

### Funding (WalletClient)
- Load `/api/funding`, show deposit address, copy button, suggested satoshis
- Pay via dynamic `import` of `@bsv/sdk` WalletClient (multi-CDN fallback URLs)
- Robust parsing of createAction results: hex / base64 / Uint8Array / nested beef fields / txid variants; optional output-index match on locking script
- `POST /api/funding/internalize` with `atomic_tx_hex` and/or `txid` + `output_index`
- Manual internalize panel (txid / atomic hex / vout) when WalletClient or CDN is unavailable
- Graceful error copy when CDN, wallet extension, or API fails

### Aesthetic / a11y
- Dark demo theme refined (gradients, focus rings, hover states, empty log panel)
- IBM Plex Sans/Mono loaded; responsive single-column under 960px
- Skip link, `aria-live` status regions, labeled controls, `prefers-reduced-motion`

## Concerns
- `@bsv/sdk` version pins: tried `1.9.25` (not published on esm.sh); UI tries `1.4.20` then unversioned `@bsv/sdk` — MetaNet host wallet shapes still vary; manual internalize is the safety net
- Chart.js and Google Fonts still need network; offline demo loses charts/fonts but keeps control plane
- No automated browser tests in this task (static assets only)
- Did not modify any Go packages (task boundary respected)
