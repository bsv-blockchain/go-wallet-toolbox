// Throughput demo dashboard — control plane client + WalletClient funding.

const MAX_POINTS = 60;

const els = {
  networkBadge: document.getElementById("network-badge"),
  mainnetBanner: document.getElementById("mainnet-banner"),
  streamState: document.getElementById("stream-state"),
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
  fundAddress: document.getElementById("fund-address"),
  fundAmount: document.getElementById("fund-amount"),
  btnFund: document.getElementById("btn-fund"),
  btnRefreshFunding: document.getElementById("btn-refresh-funding"),
  fundStatus: document.getElementById("fund-status"),
};

let fundingInfo = null;

const tpsChart = new Chart(document.getElementById("tps-chart"), {
  type: "line",
  data: {
    labels: [],
    datasets: [
      { label: "succeeded/s", data: [], borderColor: "#3dd6c6", tension: 0.25, pointRadius: 0 },
      { label: "failed/s", data: [], borderColor: "#ff5c7a", tension: 0.25, pointRadius: 0 },
    ],
  },
  options: {
    responsive: true,
    animation: false,
    scales: {
      x: { ticks: { color: "#93a0bf", maxTicksLimit: 8 }, grid: { color: "rgba(36,48,80,0.6)" } },
      y: { beginAtZero: true, ticks: { color: "#93a0bf" }, grid: { color: "rgba(36,48,80,0.6)" } },
    },
    plugins: { legend: { labels: { color: "#c9d6f5" } } },
  },
});

const fuelChart = new Chart(document.getElementById("fuel-chart"), {
  type: "line",
  data: {
    labels: [],
    datasets: [
      { label: "fuel UTXOs", data: [], borderColor: "#ffb020", tension: 0.25, pointRadius: 0 },
      { label: "reserve UTXOs", data: [], borderColor: "#8b9cff", tension: 0.25, pointRadius: 0 },
    ],
  },
  options: {
    responsive: true,
    animation: false,
    scales: {
      x: { ticks: { color: "#93a0bf", maxTicksLimit: 8 }, grid: { color: "rgba(36,48,80,0.6)" } },
      y: { beginAtZero: true, ticks: { color: "#93a0bf" }, grid: { color: "rgba(36,48,80,0.6)" } },
    },
    plugins: { legend: { labels: { color: "#c9d6f5" } } },
  },
});

function pushPoint(chart, label, values) {
  chart.data.labels.push(label);
  values.forEach((v, i) => chart.data.datasets[i].data.push(v));
  while (chart.data.labels.length > MAX_POINTS) {
    chart.data.labels.shift();
    chart.data.datasets.forEach((d) => d.data.shift());
  }
  chart.update("none");
}

function applyTick(tick) {
  if (!tick) return;
  const stream = tick.stream || {};
  els.streamState.textContent = stream.running ? "running" : "stopped";
  els.streamState.style.color = stream.running ? "#4ade80" : "#93a0bf";
  els.succeeded.textContent = stream.succeeded ?? 0;
  els.failed.textContent = stream.failed ?? 0;
  els.iteration.textContent = stream.iteration ?? 0;
  els.btnStart.disabled = !!stream.running;
  els.btnStop.disabled = !stream.running;
  if (stream.tps) els.tps.value = stream.tps;
  if (stream.workers) els.workers.value = stream.workers;

  els.balDefault.textContent = fmt(tick.default_sats);
  els.balFuel.textContent = fmt(tick.fuel_count);
  els.balReserve.textContent = fmt(tick.reserve_count);
  els.balRunway.textContent = tick.fuel_runway_seconds != null
    ? Number(tick.fuel_runway_seconds).toFixed(1)
    : "—";
  els.metaDenom.textContent = tick.denomination ?? "—";
  els.metaWater.textContent = `${tick.low_water ?? "—"} / ${tick.high_water ?? "—"} (target ${tick.target_pool_size ?? "—"})`;

  const label = (tick.timestamp || "").slice(11, 19) || new Date().toISOString().slice(11, 19);
  pushPoint(tpsChart, label, [tick.tps_succeeded || 0, tick.tps_failed || 0]);
  pushPoint(fuelChart, label, [tick.fuel_count || 0, tick.reserve_count || 0]);
}

function fmt(n) {
  if (n == null || n === "—") return "—";
  return Number(n).toLocaleString();
}

function prependTopup(ev) {
  const li = document.createElement("li");
  const p = ev.payload || {};
  const time = document.createElement("time");
  time.textContent = (ev.timestamp || "").slice(11, 19);
  li.appendChild(time);
  li.appendChild(document.createTextNode(
    p.message || `${p.kind || "topup"} ${p.basket || ""} +${p.delta ?? "?"} (now ${p.after ?? "?"})`
  ));
  els.topupLog.prepend(li);
  while (els.topupLog.children.length > 50) {
    els.topupLog.removeChild(els.topupLog.lastChild);
  }
}

async function refreshStatus() {
  const res = await fetch("/api/status");
  const data = await res.json();
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
  if (data.tick) applyTick(data.tick);
  (data.events || []).filter((e) => e.type === "topup").slice(-20).forEach(prependTopup);
}

async function refreshFunding() {
  const res = await fetch("/api/funding");
  fundingInfo = await res.json();
  if (fundingInfo.error) {
    els.fundAddress.textContent = fundingInfo.error;
    return;
  }
  els.fundAddress.textContent = fundingInfo.address;
  if (fundingInfo.suggested_satoshis) {
    els.fundAmount.value = fundingInfo.suggested_satoshis;
  }
}

els.btnStart.addEventListener("click", async () => {
  const body = {
    tps: Number(els.tps.value) || 10,
    workers: Number(els.workers.value) || 8,
  };
  const res = await fetch("/api/stream/start", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    alert(data.error || "failed to start");
    return;
  }
  await refreshStatus();
});

els.btnStop.addEventListener("click", async () => {
  await fetch("/api/stream/stop", { method: "POST" });
  await refreshStatus();
});

els.btnRefreshFunding.addEventListener("click", () => refreshFunding());

els.btnFund.addEventListener("click", async () => {
  els.fundStatus.className = "status";
  els.fundStatus.textContent = "Connecting WalletClient…";
  try {
    if (!fundingInfo?.locking_script_hex) {
      await refreshFunding();
    }
    const amount = Number(els.fundAmount.value);
    if (!amount || amount <= 0) throw new Error("Enter a positive satoshi amount");

    // Dynamic import so the page still works if CDN is blocked (status UI remains usable).
    const { WalletClient } = await import("https://esm.sh/@bsv/sdk@1.9.25");
    const client = new WalletClient();

    els.fundStatus.textContent = "Creating payment action…";
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

    const atomic =
      result?.tx ||
      result?.rawTx ||
      result?.atomicBeef ||
      result?.beef ||
      "";
    const txid = result?.txid || result?.txids?.[0] || "";

    const body = {
      output_index: 0,
      txid: typeof txid === "string" ? txid : String(txid || ""),
    };
    if (typeof atomic === "string" && atomic.length > 0) {
      // WalletClient may return hex already; strip 0x if present.
      body.atomic_tx_hex = atomic.startsWith("0x") ? atomic.slice(2) : atomic;
    } else if (atomic && typeof atomic === "object" && atomic.toHex) {
      body.atomic_tx_hex = atomic.toHex();
    } else if (atomic instanceof Uint8Array) {
      body.atomic_tx_hex = [...atomic].map((b) => b.toString(16).padStart(2, "0")).join("");
    }

    if (!body.atomic_tx_hex && !body.txid) {
      throw new Error("WalletClient returned neither tx bytes nor txid");
    }

    els.fundStatus.textContent = "Internalizing into operator wallet…";
    const res = await fetch("/api/funding/internalize", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "internalize failed");

    els.fundStatus.className = "status ok";
    els.fundStatus.textContent = `Funded OK${body.txid ? ` · ${body.txid}` : ""}. FuelKeeper will fan out into reserve/fuel.`;
    await refreshStatus();
  } catch (err) {
    console.error(err);
    els.fundStatus.className = "status err";
    els.fundStatus.textContent = err?.message || String(err);
  }
});

function connectSSE() {
  const es = new EventSource("/api/events");
  es.addEventListener("tick", (msg) => {
    try {
      const ev = JSON.parse(msg.data);
      applyTick(ev.payload?.tick);
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
    // Browser will reconnect; also poll as backup.
  };
}

await refreshStatus();
await refreshFunding();
connectSSE();
// Backup poll in case SSE is flaky.
setInterval(() => {
  refreshStatus().catch(() => {});
}, 5000);
