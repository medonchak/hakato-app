import { useEffect, useState, useRef } from 'react';
import { fetchTopTokens } from '../api';

export function ActiveTokensDashboard({ chainId }) {
  const [tokens, setTokens] = useState([]);

  useEffect(() => {
    fetchTopTokens(chainId).then(setTokens);
  }, [chainId]);
 
  return (
    <div className="grid">
      {tokens && tokens.map(t => (
        <div key={t.token} className="card">
          <h3>{t.symbol}</h3>
          <p>Volume 24h: ${t.volume_usd_24h}</p>
          <p>Net Flow: {t.net_exchange_flow}</p>
          {t.whale_events > 0 && <span>🐋 Whale activity</span>}
        </div>
      ))}
    </div>
  );
}

export function TokenInfoModal({ open, onClose, data, onToggleReport }) {
  if (!open) return null;

  return (
    <div className="modal">
      <h2>{data.symbol}</h2>

      <p>Price: ${data.price_usd}</p>
      <p>Market cap: ${data.market_cap}</p>
      <p>Holders: {data.holders}</p>

      <hr />

      <p>24h Volume: ${data.volume_24h}</p>
      <p>Net Flow: {data.net_flow}</p>
      <p>Whale events: {data.whale_events}</p>

      <label>
        <input
          type="checkbox"
          checked={data.daily_report_enabled}
          onChange={e => onToggleReport(e.target.checked)}
        />
        Daily token report
      </label>

      <button onClick={onClose}>Close</button>
    </div>
  );
}
