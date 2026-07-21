// Throughput demo dashboard — control plane client + WalletClient funding.

const MAX_POINTS = 60;
const MAX_TOPUPS = 50;
const POLL_MS = 5000;
const SDK_URLS = [
  "https://esm.sh/@bsv/sdk@1.4.20",
  "https://esm.sh/@bsv/sdk",
];

const els = {
  networkBadge: document.getElementById("network-badge"),
  mainnetBanner: document.getElementById("mainnet-banner"),
  connStatus: document.getElementById("conn-status"),
  connLabel: document.getElementById("conn-label"),
  streamState: document.getElementById("stream-state"),
  streamStatus: document.getElementById("stream-status"),
  succeeded: document.getElementById("stat-succeeded"),
  failed: document.getElementById("stat-failed"),
  iteration: document.getElementById("stat-iteration"),
  tps: document.getElementById("tps"),
  workers: document.getElementById("workers"),
  btnStart: document.getElementById("btn-start"),
  btnStop: document.getElementById("btn-stop"),
  balDefault: document.getElementById("bal-default"),
  balFuel: document.getElementById("bal-fuel"),
  balReserve: document.getElementById("bal-reserve"),
  balRunway: document.getElementById("bal-runway"),
  metaDenom: document.getElementById("meta-denom"),
  metaWater: document.getElementById("meta-water"),
  metaServer: document.getElementById("meta-server"),
  topupLog: document.getElementById("topup-log"),
  topupEmpty: document.getElementById("topup-empty"),
  fundAddress: document.getElementById("fund-address"),
  fundAmount: document.getElementById("fund-amount"),
  btnFund: document.getElementById("btn-fund"),
  btnRefreshFunding: document.getElementById("btn-refresh-funding"),
  btnCopyAddress: document.getElementById("btn-copy-address"),
  fundStatus: document.getElementById("fund-status"),
  manualTxid: document.getElementById("manual-txid"),
  manualHex: document.getElementById("manual-hex"),
  manualVout: document.getElementById("manual-vout"),
  btnManualInternalize: document.getElementById("btn-manual-internalize"),
  tpsChartFallback: document.getElementById("tps-chart-fallback"),
  fuelChartFallback: document.getElementById("fuel-chart-fallback"),
};

let fundingInfo = null;
let lastTickKey = "";
let chartsEnabled = typeof window.Chart === "function";
const seenTopups = new Set();
let streamBusy = false;

// ---------------------------------------------------------------------------
// Charts (optional — page remains usable if Chart.js CDN is blocked)
// ---------------------------------------------------------------------------

const chartDefaults = {
  responsive: true,
  maintainAspectRatio: true,
  animation: false,
  interaction: { mode: "index", intersect: false },
  scales: {
    x: {
      ticks: { color: "#93a0bf", maxTicksLimit: 8 },
      grid: { color: "rgba(36,48,80,0.6)" },
    },
    y: {
      beginAtZero: true,
      ticks: { color: "#93a0bf", precision: 0 },
      grid: { color: "rgba(36,48,80,0.6)" },
    },
  },
  plugins: {
    legend: {
      labels: { color: "#c9d6f5", boxWidth: 12, boxHeight: 12, usePointStyle: true },
    },
    tooltip: {
      backgroundColor: "rgba(12, 18, 36, 0.95)",
      titleColor: "#e8eefc",
      bodyColor: "#c9d6f5",
      borderColor: "#243050",
      borderWidth: 1,
    },
  },
};

let tpsChart = null;
let fuelChart = null;

if (chartsEnabled) {
  try {
    tpsChart = new Chart(document.getElementById("tps-chart"), {
      type: "line",
      data: {
        labels: [],
        datasets: [
          {
            label: "succeeded/s",
            data: [],
            borderColor: "#3dd6c6",
            backgroundColor: "rgba(61, 214, 198, 0.12)",
            fill: true,
            tension: 0.3,
            pointRadius: 0,
            borderWidth: 2,
          },
          {
            label: "failed/s",
            data: [],
            borderColor: "#ff5c7a",
            backgroundColor: "rgba(255, 92, 122, 0.08)",
            fill: true,
            tension: 0.3,
            pointRadius: 0,
            borderWidth: 2,
          },
        ],
      },
      options: chartDefaults,
    });

    fuelChart = new Chart(document.getElementById("fuel-chart"), {
      type: "line",
      data: {
        labels: [],
        datasets: [
          {
            label: "fuel UTXOs",
            data: [],
            borderColor: "#ffb020",
            backgroundColor: "rgba(255, 176, 32, 0.1)",
            fill: true,
            tension: 0.3,
            pointRadius: 0,
            borderWidth: 2,
          },
          {
            label: "reserve UTXOs",
            data: [],
            borderColor: "#8b9cff",
            backgroundColor: "rgba(139, 156, 255, 0.08)",
            fill: true,
            tension: 0.3,
            pointRadius: 0,
            borderWidth: 2,
          },
        ],
      },
      options: chartDefaults,
    });
  } catch (err) {
    console.warn("Chart.js init failed", err);
    chartsEnabled = false;
    tpsChart = null;
    fuelChart = null;
  }
}

if (!chartsEnabled) {
  els.tpsChartFallback.hidden = false;
  els.fuelChartFallback.hidden = false;
  document.getElementById("tps-chart")?.setAttribute("hidden", "");
  document.getElementById("fuel-chart")?.setAttribute("hidden", "");
}

function pushPoint(chart, label, values) {
  if (!chart) return;
  chart.data.labels.push(label);
  values.forEach((v, i) => chart.data.datasets[i].data.push(v));
  while (chart.data.labels.length > MAX_POINTS) {
    chart.data.labels.shift();
    chart.data.datasets.forEach((d) => d.data.shift());
  }
  chart.update("none");
}

// ---------------------------------------------------------------------------
// Formatting & status helpers
// ---------------------------------------------------------------------------

function fmt(n) {
  if (n == null || n === "—") return "—";
  return Number(n).toLocaleString();
}

function setStatus(el, kind, message) {
  el.className = kind ? `status ${kind}` : "status";
  el.textContent = message || "";
}

function setConn(state, label) {
  els.connStatus.className = `conn conn-${state}`;
  els.connLabel.textContent = label;
}

function setBusy(btn, busy, idleLabel, busyLabel) {
  btn.disabled = busy || btn.dataset.forceDisabled === "1";
  btn.classList.toggle("busy", busy);
  if (busyLabel || idleLabel) {
    btn.textContent = busy ? (busyLabel || idleLabel) : idleLabel;
  }
}

function tickKey(tick) {
  if (!tick) return "";
  return [
    tick.timestamp,
    tick.tps_succeeded,
    tick.tps_failed,
    tick.fuel_count,
    tick.reserve_count,
    tick.stream?.succeeded,
    tick.stream?.failed,
    tick.stream?.iteration,
    tick.stream?.running,
  ].join("|");
}

// ---------------------------------------------------------------------------
// Tick / top-up application
// ---------------------------------------------------------------------------

function applyTick(tick, { chart = true } = {}) {
  if (!tick) return;

  const key = tickKey(tick);
  const isDup = key && key === lastTickKey;
  lastTickKey = key || lastTickKey;

  const stream = tick.stream || {};
  const running = !!stream.running;

  els.streamState.textContent = running ? "running" : "stopped";
  els.streamState.className = `value state-pill ${running ? "state-running" : "state-stopped"}`;
  els.succeeded.textContent = fmt(stream.succeeded ?? 0);
  els.failed.textContent = fmt(stream.failed ?? 0);
  els.iteration.textContent = fmt(stream.iteration ?? 0);

  if (!streamBusy) {
    els.btnStart.disabled = running;
    els.btnStart.dataset.forceDisabled = running ? "1" : "0";
    els.btnStop.disabled = !running;
    els.btnStop.dataset.forceDisabled = !running ? "1" : "0";
    els.tps.disabled = running;
    els.workers.disabled = running;
  }

  // Reflect live configured rate only when stream reports values.
  if (stream.tps) els.tps.value = stream.tps;
  if (stream.workers) els.workers.value = stream.workers;

  els.balDefault.textContent = fmt(tick.default_sats);
  els.balFuel.textContent = fmt(tick.fuel_count);
  els.balReserve.textContent = fmt(tick.reserve_count);
  els.balRunway.textContent =
    tick.fuel_runway_seconds != null ? Number(tick.fuel_runway_seconds).toFixed(1) : "—";
  els.metaDenom.textContent = tick.denomination ?? "—";
  els.metaWater.textContent = `${tick.low_water ?? "—"} / ${tick.high_water ?? "—"} (target ${tick.target_pool_size ?? "—"})`;

  if (chart && !isDup) {
    const label =
      (tick.timestamp || "").slice(11, 19) || new Date().toISOString().slice(11, 19);
    pushPoint(tpsChart, label, [tick.tps_succeeded || 0, tick.tps_failed || 0]);
    pushPoint(fuelChart, label, [tick.fuel_count || 0, tick.reserve_count || 0]);
  }
}

function topupKey(ev) {
  const p = ev?.payload || {};
  return `${ev?.timestamp || ""}|${p.basket || ""}|${p.before ?? ""}|${p.after ?? ""}|${p.kind || ""}`;
}

function prependTopup(ev) {
  if (!ev) return;
  const key = topupKey(ev);
  if (key && seenTopups.has(key)) return;
  if (key) {
    seenTopups.add(key);
    // Bound memory for long-running demos.
    if (seenTopups.size > MAX_TOPUPS * 4) {
      const drop = [...seenTopups].slice(0, seenTopups.size - MAX_TOPUPS * 2);
      drop.forEach((k) => seenTopups.delete(k));
    }
  }

  const p = ev.payload || {};
  const li = document.createElement("li");

  const time = document.createElement("time");
  time.dateTime = ev.timestamp || "";
  time.textContent = (ev.timestamp || "").slice(11, 19) || "—";
  li.appendChild(time);

  const basket = String(p.basket || "").toLowerCase();
  if (basket) {
    const tag = document.createElement("span");
    tag.className = `tag${basket === "reserve" ? " reserve" : ""}`;
    tag.textContent = basket;
    li.appendChild(tag);
  }

  const text = document.createElement("span");
  text.textContent =
    p.message ||
    `${p.kind || "topup"} ${p.basket || ""} +${p.delta ?? "?"} (now ${p.after ?? "?"})`;
  li.appendChild(text);

  els.topupLog.prepend(li);
  while (els.topupLog.children.length > MAX_TOPUPS) {
    els.topupLog.removeChild(els.topupLog.lastChild);
  }
  els.topupEmpty.classList.add("hidden");
}

// ---------------------------------------------------------------------------
// API helpers
// ---------------------------------------------------------------------------

async function readJSON(res) {
  const text = await res.text();
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    throw new Error(`Non-JSON response (${res.status}): ${text.slice(0, 120)}`);
  }
}

async function refreshStatus({ chart = false } = {}) {
  const res = await fetch("/api/status");
  const data = await readJSON(res);
  if (!res.ok) throw new Error(data.error || `status ${res.status}`);

  els.metaServer.textContent = data.server_url || "—";
  if (data.mainnet) {
    els.networkBadge.textContent = "MAINNET";
    els.networkBadge.className = "badge badge-warn";
    els.mainnetBanner.style.display = "";
  } else {
    els.networkBadge.textContent = String(data.network || "test").toUpperCase();
    els.networkBadge.className = "badge badge-ok";
    els.mainnetBanner.style.display = "none";
  }

  if (data.tick) applyTick(data.tick, { chart });

  // Seed top-up log once from status history (deduped).
  const topups = (data.events || []).filter((e) => e.type === "topup");
  topups.slice(-20).forEach(prependTopup);
  return data;
}

async function refreshFunding() {
  els.btnRefreshFunding.disabled = true;
  try {
    const res = await fetch("/api/funding");
    fundingInfo = await readJSON(res);
    if (!res.ok || fundingInfo.error) {
      els.fundAddress.textContent = fundingInfo.error || `funding ${res.status}`;
      els.btnCopyAddress.disabled = true;
      setStatus(els.fundStatus, "err", fundingInfo.error || "Failed to load deposit address");
      return;
    }
    els.fundAddress.textContent = fundingInfo.address || "—";
    els.btnCopyAddress.disabled = !fundingInfo.address;
    if (fundingInfo.suggested_satoshis) {
      els.fundAmount.value = fundingInfo.suggested_satoshis;
    }
    setStatus(els.fundStatus, "", "");
  } catch (err) {
    els.fundAddress.textContent = "unavailable";
    els.btnCopyAddress.disabled = true;
    setStatus(els.fundStatus, "err", err?.message || String(err));
  } finally {
    els.btnRefreshFunding.disabled = false;
  }
}

// ---------------------------------------------------------------------------
// Funding result parsing — WalletClient shapes vary across sdk / wallet hosts
// ---------------------------------------------------------------------------

function bytesToHex(bytes) {
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

function looksLikeHex(s) {
  return typeof s === "string" && s.length >= 2 && /^[0-9a-fA-F]+$/.test(s.replace(/^0x/i, ""));
}

function normalizeHex(s) {
  if (typeof s !== "string") return "";
  const t = s.trim().replace(/^0x/i, "");
  return looksLikeHex(t) ? t.toLowerCase() : "";
}

function tryBase64ToHex(s) {
  if (typeof s !== "string" || !s.length) return "";
  // Avoid treating pure hex as base64.
  if (looksLikeHex(s)) return "";
  try {
    const bin = atob(s);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return bytesToHex(bytes);
  } catch {
    return "";
  }
}

function coerceToHex(value) {
  if (value == null || value === "") return "";
  if (typeof value === "string") {
    const asHex = normalizeHex(value);
    if (asHex) return asHex;
    return tryBase64ToHex(value);
  }
  if (value instanceof Uint8Array || ArrayBuffer.isView(value)) {
    return bytesToHex(new Uint8Array(value.buffer, value.byteOffset, value.byteLength));
  }
  if (value instanceof ArrayBuffer) {
    return bytesToHex(new Uint8Array(value));
  }
  if (Array.isArray(value) && value.every((n) => Number.isInteger(n) && n >= 0 && n <= 255)) {
    return bytesToHex(value);
  }
  if (typeof value === "object") {
    if (typeof value.toHex === "function") {
      try {
        return normalizeHex(value.toHex()) || coerceToHex(value.toHex());
      } catch { /* ignore */ }
    }
    if (typeof value.toString === "function" && value.toString !== Object.prototype.toString) {
      const s = value.toString();
      if (looksLikeHex(s)) return normalizeHex(s);
    }
    // Nested common fields.
    for (const k of ["hex", "atomicBEEF", "atomicBeef", "beef", "rawTx", "tx", "bytes"]) {
      if (k in value) {
        const nested = coerceToHex(value[k]);
        if (nested) return nested;
      }
    }
  }
  return "";
}

function extractTxid(result) {
  if (!result || typeof result !== "object") return "";
  const candidates = [
    result.txid,
    result.txId,
    result.txID,
    result.TxID,
    Array.isArray(result.txids) ? result.txids[0] : null,
    Array.isArray(result.Txids) ? result.Txids[0] : null,
  ];
  for (const c of candidates) {
    if (typeof c === "string") {
      const h = normalizeHex(c);
      if (h.length === 64) return h;
    }
  }
  return "";
}

function extractAtomicHex(result) {
  if (!result || typeof result !== "object") return "";
  const candidates = [
    result.tx,
    result.Tx,
    result.rawTx,
    result.rawtx,
    result.atomicBeef,
    result.atomicBEEF,
    result.AtomicBEEF,
    result.beef,
    result.BEEF,
    result.signableTransaction?.tx,
    result.signableTransaction?.Tx,
  ];
  for (const c of candidates) {
    const hex = coerceToHex(c);
    if (hex) return hex;
  }
  return "";
}

function extractOutputIndex(result, lockingScriptHex) {
  // Prefer explicit single-output payment (vout 0) unless wallet reports outputs.
  const outs =
    result?.outputs ||
    result?.Outputs ||
    result?.tx?.outputs ||
    null;
  if (Array.isArray(outs) && lockingScriptHex) {
    const want = normalizeHex(lockingScriptHex);
    for (let i = 0; i < outs.length; i++) {
      const ls =
        outs[i]?.lockingScript ||
        outs[i]?.locking_script ||
        outs[i]?.script ||
        "";
      const hex = coerceToHex(ls);
      if (hex && want && hex === want) return i;
    }
  }
  return 0;
}

async function loadWalletClient() {
  let lastErr;
  for (const url of SDK_URLS) {
    try {
      const mod = await import(/* @vite-ignore */ url);
      if (mod?.WalletClient) return mod.WalletClient;
      if (mod?.default?.WalletClient) return mod.default.WalletClient;
      lastErr = new Error(`WalletClient export missing from ${url}`);
    } catch (err) {
      lastErr = err;
    }
  }
  throw new Error(
    `Could not load @bsv/sdk WalletClient (CDN blocked or offline). ${lastErr?.message || lastErr || ""}`.trim()
  );
}

async function postInternalize(body) {
  const res = await fetch("/api/funding/internalize", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await readJSON(res);
  if (!res.ok) throw new Error(data.error || "internalize failed");
  return data;
}

// ---------------------------------------------------------------------------
// Stream controls
// ---------------------------------------------------------------------------

function parsePositiveInt(input, fallback, { min = 1, max = 1e9 } = {}) {
  const n = Number(input);
  if (!Number.isFinite(n) || n < min) return fallback;
  return Math.min(max, Math.floor(n));
}

els.btnStart.addEventListener("click", async () => {
  const tps = parsePositiveInt(els.tps.value, 10, { min: 1, max: 10000 });
  const workers = parsePositiveInt(els.workers.value, 8, { min: 1, max: 256 });
  els.tps.value = tps;
  els.workers.value = workers;

  streamBusy = true;
  setBusy(els.btnStart, true, "Start stream", "Starting…");
  els.btnStop.disabled = true;
  setStatus(els.streamStatus, "info", `Starting stream at ${tps} TPS · ${workers} workers…`);

  try {
    const res = await fetch("/api/stream/start", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tps, workers }),
    });
    const data = await readJSON(res);
    if (!res.ok) throw new Error(data.error || "failed to start");
    setStatus(els.streamStatus, "ok", "Stream running.");
    await refreshStatus({ chart: false });
  } catch (err) {
    setStatus(els.streamStatus, "err", err?.message || String(err));
  } finally {
    streamBusy = false;
    setBusy(els.btnStart, false, "Start stream");
    // applyTick from refreshStatus re-enables buttons from stream state.
    await refreshStatus({ chart: false }).catch(() => {});
  }
});

els.btnStop.addEventListener("click", async () => {
  streamBusy = true;
  setBusy(els.btnStop, true, "Stop stream", "Stopping…");
  els.btnStart.disabled = true;
  setStatus(els.streamStatus, "info", "Stopping stream…");
  try {
    const res = await fetch("/api/stream/stop", { method: "POST" });
    const data = await readJSON(res);
    if (!res.ok) throw new Error(data.error || "failed to stop");
    setStatus(els.streamStatus, "ok", "Stream stopped. FuelKeeper continues running.");
  } catch (err) {
    setStatus(els.streamStatus, "err", err?.message || String(err));
  } finally {
    streamBusy = false;
    setBusy(els.btnStop, false, "Stop stream");
    await refreshStatus({ chart: false }).catch(() => {});
  }
});

// ---------------------------------------------------------------------------
// Funding controls
// ---------------------------------------------------------------------------

els.btnRefreshFunding.addEventListener("click", () => {
  refreshFunding();
});

els.btnCopyAddress.addEventListener("click", async () => {
  const addr = fundingInfo?.address || els.fundAddress.textContent;
  if (!addr || addr === "loading…" || addr === "unavailable") return;
  try {
    await navigator.clipboard.writeText(addr);
    setStatus(els.fundStatus, "ok", "Address copied to clipboard.");
  } catch {
    // Fallback for restricted clipboard.
    const range = document.createRange();
    range.selectNodeContents(els.fundAddress);
    const sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
    setStatus(els.fundStatus, "info", "Select the address and copy manually (clipboard blocked).");
  }
});

els.btnFund.addEventListener("click", async () => {
  setStatus(els.fundStatus, "info", "Connecting WalletClient…");
  setBusy(els.btnFund, true, "Pay with WalletClient", "Paying…");
  els.btnManualInternalize.disabled = true;

  try {
    if (!fundingInfo?.locking_script_hex) {
      await refreshFunding();
    }
    if (!fundingInfo?.locking_script_hex) {
      throw new Error("Deposit locking script unavailable — check /api/funding");
    }

    const amount = parsePositiveInt(els.fundAmount.value, 0, { min: 1 });
    if (!amount) throw new Error("Enter a positive satoshi amount");
    els.fundAmount.value = amount;

    const WalletClient = await loadWalletClient();
    const client = new WalletClient();

    setStatus(els.fundStatus, "info", "Creating payment action in browser wallet…");
    const result = await client.createAction({
      description: "throughput dashboard operator top-up",
      outputs: [
        {
          lockingScript: fundingInfo.locking_script_hex,
          satoshis: amount,
          outputDescription: "operator fuel deposit",
        },
      ],
    });

    const atomicHex = extractAtomicHex(result);
    const txid = extractTxid(result);
    const outputIndex = extractOutputIndex(result, fundingInfo.locking_script_hex);

    const body = { output_index: outputIndex };
    if (atomicHex) body.atomic_tx_hex = atomicHex;
    if (txid) body.txid = txid;

    if (!body.atomic_tx_hex && !body.txid) {
      console.error("WalletClient createAction result:", result);
      throw new Error(
        "WalletClient returned neither tx bytes nor txid. Use Manual internalize with a known txid, or check the browser wallet."
      );
    }

    setStatus(els.fundStatus, "info", "Internalizing into operator wallet…");
    await postInternalize(body);

    const ref = body.txid || (body.atomic_tx_hex ? `${body.atomic_tx_hex.slice(0, 16)}…` : "");
    setStatus(
      els.fundStatus,
      "ok",
      `Funded OK${ref ? ` · ${ref}` : ""}. FuelKeeper will fan out into reserve/fuel.`
    );
    await refreshStatus({ chart: false });
  } catch (err) {
    console.error(err);
    const msg = err?.message || String(err);
    // Friendlier guidance when CDN / extension is missing.
    if (/Failed to fetch|import|CDN|WalletClient|network/i.test(msg)) {
      setStatus(
        els.fundStatus,
        "err",
        `${msg} — open Manual internalize below if you paid the address externally.`
      );
    } else {
      setStatus(els.fundStatus, "err", msg);
    }
  } finally {
    setBusy(els.btnFund, false, "Pay with WalletClient");
    els.btnManualInternalize.disabled = false;
  }
});

els.btnManualInternalize.addEventListener("click", async () => {
  setBusy(els.btnManualInternalize, true, "Internalize", "Internalizing…");
  setStatus(els.fundStatus, "info", "Internalizing…");
  try {
    const txid = normalizeHex(els.manualTxid.value.trim());
    const atomic = normalizeHex(els.manualHex.value.trim()) || tryBase64ToHex(els.manualHex.value.trim());
    const outputIndex = parsePositiveInt(els.manualVout.value, 0, { min: 0, max: 1e6 });

    if (!txid && !atomic) {
      throw new Error("Provide a txid and/or atomic tx hex");
    }
    if (txid && txid.length !== 64) {
      throw new Error("txid must be 64 hex characters");
    }

    const body = { output_index: outputIndex };
    if (atomic) body.atomic_tx_hex = atomic;
    if (txid) body.txid = txid;

    await postInternalize(body);
    setStatus(els.fundStatus, "ok", `Internalized OK${txid ? ` · ${txid}` : ""}.`);
    els.manualTxid.value = "";
    els.manualHex.value = "";
    await refreshStatus({ chart: false });
  } catch (err) {
    setStatus(els.fundStatus, "err", err?.message || String(err));
  } finally {
    setBusy(els.btnManualInternalize, false, "Internalize");
  }
});

// ---------------------------------------------------------------------------
// SSE + poll backup
// ---------------------------------------------------------------------------

function connectSSE() {
  setConn("pending", "Connecting…");
  const es = new EventSource("/api/events");

  es.addEventListener("open", () => {
    setConn("live", "Live");
  });

  // Some browsers only fire open via onopen.
  es.onopen = () => setConn("live", "Live");

  es.addEventListener("tick", (msg) => {
    try {
      const ev = JSON.parse(msg.data);
      const tick = ev.payload?.tick ?? ev.tick ?? null;
      applyTick(tick, { chart: true });
      setConn("live", "Live");
    } catch (e) {
      console.warn("tick parse", e);
    }
  });

  es.addEventListener("topup", (msg) => {
    try {
      const ev = JSON.parse(msg.data);
      prependTopup(ev);
    } catch (e) {
      console.warn("topup parse", e);
    }
  });

  es.onerror = () => {
    setConn("offline", "Reconnecting…");
    // Browser auto-reconnects EventSource; poll backup keeps gauges fresh.
  };
}

// ---------------------------------------------------------------------------
// Boot
// ---------------------------------------------------------------------------

try {
  await refreshStatus({ chart: true });
} catch (err) {
  setStatus(els.streamStatus, "err", `Status unavailable: ${err?.message || err}`);
  setConn("offline", "API offline");
}

await refreshFunding();
connectSSE();

setInterval(() => {
  refreshStatus({ chart: false }).catch(() => {
    setConn("offline", "Poll failed");
  });
}, POLL_MS);
