import React, { useEffect, useState } from "react";

export default function NetworkHudPanel({ chain }) {
  const [d, setD] = useState(null);

  useEffect(() => {
    let ok = true;
    fetch(`/api/stats?chain=${chain}`)
      .then(r => r.json())
      .then(j => ok && setD(j))
      .catch(() => ok && setD(null));
    return () => { ok = false };
  }, [chain]);

  if (!d) return null;

  const tx = Number(d.tx_1h || 0);
  const tx24 = Number(d.tx_24h || 0);
  const gas = Number(d.gas_1h || 0) / 1e18;

  const base = Number(d.tx_baseline_1h || tx || 1);
  const prev = Number(d.tx_prev_1h || base);

  const load = tx / base;
  const delta = ((tx - prev) / Math.max(prev, 1)) * 100;

  const status =
    load < 1.1 ? "OK" :
    load < 1.5 ? "WATCH" :
    "ALERT";

  const color =
    status === "OK" ? "#22c55e" :
    status === "WATCH" ? "#f59e0b" :
    "#ef4444";

  const pct = Math.min(Math.round(load * 100), 200);

  return (
    <div style={{
      width: 320,
      padding: 16,
      borderRadius: 18,
      background: "linear-gradient(180deg,#0b1020,#050814)",
      boxShadow: "0 20px 40px rgba(0,0,0,.45)",
      color: "#e5e7eb",
      display: "grid",
      gridTemplateColumns: "1fr 120px",
      gap: 16
    }}>
      {/* LEFT – DATA */}
      <div style={{ display: "grid", gap: 10 }}>
        <div style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center"
        }}>
          <div style={{ fontSize: 13, letterSpacing: 1 }}>
            {chain.toUpperCase()}
          </div>
          <div style={{
            fontSize: 12,
            color,
            fontWeight: 600
          }}>
            {status}
          </div>
        </div>

        <Metric label="Tx / 1h" value={tx.toLocaleString()} />
        <Metric label="Tx / 24h" value={tx24.toLocaleString()} />
        <Metric label="Gas / 1h" value={gas.toFixed(2)} />
        <Metric
          label="Δ activity"
          value={`${delta > 0 ? "+" : ""}${delta.toFixed(1)}%`}
          accent
        />

        <div>
          <div style={{ fontSize: 11, opacity: .6 }}>Load vs baseline</div>
          <div style={{
            height: 8,
            borderRadius: 6,
            background: "#271b11",
            overflow: "hidden",
            marginTop: 4
          }}>
            <div style={{
              width: `${pct}%`,
              height: "100%",
              background: color,
              transition: "width .4s ease"
            }} />
          </div>
        </div>
      </div>

      {/* RIGHT – RADIAL GAUGE */}
      <div style={{
        position: "relative",
        width: 120,
        height: 120
      }}>
        <svg viewBox="0 0 120 120" width="120" height="120">
          <circle
            cx="60" cy="60" r="48"
            fill="none"
            stroke="#111827"
            strokeWidth="8"
          />
          <circle
            cx="60" cy="60" r="48"
            fill="none"
            stroke={color}
            strokeWidth="8"
            strokeDasharray={`${(pct / 100) * 302} 302`}
            transform="rotate(-90 60 60)"
          />
          <circle cx="60" cy="60" r="4" fill={color} />
        </svg>

        <div style={{
          position: "absolute",
          inset: 0,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: 14,
          fontWeight: 600
        }}>
          {pct}%
        </div>
      </div>
    </div>
  );
}

function Metric({ label, value, accent }) {
  return (
    <div style={{
      display: "flex",
      justifyContent: "space-between",
      fontSize: 12,
      color: accent ? "#93c5fd" : "#e5e7eb"
    }}>
      <span style={{ opacity: .6 }}>{label}</span>
      <span>{value}</span>
    </div>
  );
}
