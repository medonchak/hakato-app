import React, { useEffect, useState } from "react";
import {
  getTokenActivity,
  getTokenHourly,
  getTokenAnomalies,
} from "../api";

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

export default function TokenDashboard({ token }) {
  const [hourly, setHourly] = useState([]);
  const [activity, setActivity] = useState([]);
  const [anomalies, setAnomalies] = useState([]);
  const [loading, setLoading] = useState(true);
    
  useEffect(() => {
    setLoading(true);
    Promise.all([
      getTokenHourly(token.id),
      getTokenActivity(token.id),
      getTokenAnomalies(token.id),
    ])
    .then(([h, a, an]) => {
  setHourly(
    (h.data || [])
      .map(x => ({
        ...x,
        time: new Date(x.hour_ts * 1000).toLocaleTimeString(),
      }))
      .reverse()
  );
  setActivity(a.data || []);
  setAnomalies(an.data || []);
})
      .finally(() => setLoading(false));
  }, [token]);
console.log(activity,hourly,anomalies,loading)
  if (loading) return <div>Loading token…</div>;

  return (
    <div style={{ padding: 16 }}>
  

      <h2 style={{ marginTop: 8 }}>Token Analytics</h2>
      <div style={{ opacity: 0.6 }}>{token.symbol}</div>

      {/* ===== CHART ===== */}
      <h3 style={{ marginTop: 24 }}>📈 Volume (USD)</h3>

      <ResponsiveContainer width="100%" height={260}>
        <LineChart data={hourly}>
          <XAxis dataKey="time" />
          <YAxis />
          <Tooltip />
          <Line
            type="monotone"
            dataKey="volume_usd"
            stroke="#4caf50"
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>

      {/* ===== ANOMALIES ===== */}
      <h3 style={{ marginTop: 24 }}>🚨 Anomalies</h3>

      {anomalies.length === 0 && <div>No anomalies</div>}

      {anomalies.map((a) => (
        <div
          key={a.tx_hash}
          style={{
            border: "1px solid #f44336",
            borderRadius: 8,
            padding: 12,
            marginBottom: 8,
          }}
        >
          <b>{a.reason}</b>
          <div>Severity: {a.severity}</div>
          <div>USD: {a.amount_usd?.toFixed(2) ?? "-"}</div>
          <div>Direction: {a.direction}</div>
        </div>
      ))}

      {/* ===== TRANSFERS ===== */}
      <h3 style={{ marginTop: 24 }}>🔁 Transfers</h3>

      <table width="100%">
        <thead>
          <tr>
            <th align="left">Time</th>
            <th align="center">Dir</th>
            <th align="right">USD</th>
            <th align="left">Exchange</th>
          </tr>
        </thead>
        <tbody>
          {activity.map((t) => (
            <tr key={t.tx_hash}>
              <td>{new Date(t.block_time).toLocaleTimeString()}</td>
              <td align="center">{t.direction}</td>
              <td align="right">
                {t.amount_usd?.toFixed(2) ?? "-"}
              </td>
              <td>{t.exchange_name ?? "-"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
