import React, { useEffect, useState, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, Tooltip, ReferenceLine, ResponsiveContainer } from 'recharts';
import { TrendingUpIcon, TrendingDownIcon, ActivityIcon, ExternalLinkIcon } from 'lucide-react';
import { WalletPanel } from './WalletPanel';

const API = process.env.REACT_APP_API_URL || 'http://localhost:8080';

export function AgentDashboard() {
  const [signals, setSignals] = useState([]);
  const [strategy, setStrategy] = useState(null);
  const [position, setPosition] = useState(null);
  const [loading, setLoading] = useState(true);
  const [activeToken, setActiveToken] = useState('mETH');
  const [tradeConfig, setTradeConfig] = useState(null);

  const MANTLE_TOKENS = ['mETH', 'MNT', 'USDY'];

  const fetchAgentData = useCallback(async () => {
    try {
      const [sigRes, stratRes, posRes] = await Promise.allSettled([
        fetch(`${API}/api/agent/signals?token=${activeToken}&chain_id=5000`),
        fetch(`${API}/api/agent/strategy?token=${activeToken}&chain_id=5000`),
        fetch(`${API}/api/agent/position?chain_id=5000`),
      ]);

      if (sigRes.status === 'fulfilled' && sigRes.value.ok) {
        const data = await sigRes.value.json();
        setSignals(Array.isArray(data) ? data : []);
      }
      if (stratRes.status === 'fulfilled' && stratRes.value.ok) {
        const data = await stratRes.value.json();
        setStrategy(data);
      }
      if (posRes.status === 'fulfilled' && posRes.value.ok) {
        const data = await posRes.value.json();
        setPosition(data);
      }
    } catch {
      // silently continue with empty state
    } finally {
      setLoading(false);
    }
  }, [activeToken]);

  useEffect(() => {
    fetchAgentData();
    const interval = setInterval(fetchAgentData, 30000);
    return () => clearInterval(interval);
  }, [fetchAgentData]);

  const chartData = signals.slice(-48).map((s) => ({
    time: new Date(s.created_at).toLocaleTimeString('uk-UA', { hour: '2-digit', minute: '2-digit' }),
    price: s.price_usd,
    vwap: s.vwap,
    signal: s.signal_type,
  }));

  const trades = signals.filter((s) => s.signal_type === 'BUY' || s.signal_type === 'SELL');

  return (
    <div className="space-y-4 pb-24">
      {/* Header */}
      <div className="flex items-center justify-between pt-2">
        <h1 className="text-lg font-bold text-slate-800">Agent</h1>
        <div className="flex gap-1">
          {MANTLE_TOKENS.map((tok) => (
            <button
              key={tok}
              onClick={() => setActiveToken(tok)}
              className={`px-3 py-1 rounded-full text-xs font-semibold transition-colors ${
                activeToken === tok ? 'bg-cyan-500 text-white' : 'bg-slate-100 text-slate-500'
              }`}
            >
              {tok}
            </button>
          ))}
        </div>
      </div>

      {/* Wallet connection + trade amount */}
      <WalletPanel onTradeConfigSaved={setTradeConfig} />

      {/* Active trade config badge */}
      {tradeConfig && (
        <div className="flex items-center gap-2 bg-cyan-50 border border-cyan-200 rounded-xl px-3 py-2">
          <div className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse" />
          <span className="text-xs text-cyan-700">
            Агент торгує: <strong>{tradeConfig.amount} {tradeConfig.token}</strong> за угоду
          </span>
        </div>
      )}

      {/* Position & PnL */}
      {position && (
        <div className="bg-gradient-to-br from-slate-800 to-slate-900 rounded-2xl p-4 text-white">
          <p className="text-xs text-slate-400 mb-1">Поточна позиція</p>
          <div className="flex justify-between items-end">
            <div>
              <p className="text-2xl font-bold">
                {position.size_usd != null ? `$${Number(position.size_usd).toFixed(2)}` : '—'}
              </p>
              <p className="text-xs text-slate-400 mt-0.5">{position.token || '—'}</p>
            </div>
            <div className="text-right">
              <p
                className={`text-lg font-semibold ${
                  (position.pnl_usd ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'
                }`}
              >
                {position.pnl_usd != null
                  ? `${position.pnl_usd >= 0 ? '+' : ''}$${Number(position.pnl_usd).toFixed(2)}`
                  : '—'}
              </p>
              <p className="text-xs text-slate-400">PnL</p>
            </div>
          </div>
        </div>
      )}

      {/* Strategy card */}
      {strategy && (
        <div className="bg-white rounded-2xl p-4 border border-slate-100 shadow-sm">
          <div className="flex items-center gap-2 mb-3">
            <ActivityIcon className="w-4 h-4 text-cyan-500" />
            <span className="text-sm font-semibold text-slate-700">Оптимальна стратегія — {activeToken}</span>
          </div>
          <div className="grid grid-cols-3 gap-2 text-center">
            <div className="bg-slate-50 rounded-xl p-2">
              <p className="text-xs text-slate-400">VWAP period</p>
              <p className="text-sm font-bold text-slate-800">{strategy.vwap_period ?? '—'}h</p>
            </div>
            <div className="bg-slate-50 rounded-xl p-2">
              <p className="text-xs text-slate-400">Buy threshold</p>
              <p className="text-sm font-bold text-green-600">
                {strategy.buy_threshold != null ? `${strategy.buy_threshold}%` : '—'}
              </p>
            </div>
            <div className="bg-slate-50 rounded-xl p-2">
              <p className="text-xs text-slate-400">Sell threshold</p>
              <p className="text-sm font-bold text-red-500">
                {strategy.sell_threshold != null ? `+${strategy.sell_threshold}%` : '—'}
              </p>
            </div>
          </div>
          {strategy.sharpe != null && (
            <p className="text-xs text-slate-400 mt-2 text-center">
              Sharpe ratio: <span className="font-semibold text-slate-700">{Number(strategy.sharpe).toFixed(2)}</span>
            </p>
          )}
        </div>
      )}

      {/* VWAP chart */}
      {chartData.length > 0 && (
        <div className="bg-white rounded-2xl p-4 border border-slate-100 shadow-sm">
          <p className="text-sm font-semibold text-slate-700 mb-3">VWAP · Price · Signals</p>
          <ResponsiveContainer width="100%" height={180}>
            <LineChart data={chartData}>
              <XAxis dataKey="time" tick={{ fontSize: 10 }} interval="preserveStartEnd" />
              <YAxis tick={{ fontSize: 10 }} width={55} tickFormatter={(v) => `$${v.toFixed(2)}`} />
              <Tooltip
                formatter={(val, name) => [`$${Number(val).toFixed(4)}`, name === 'price' ? 'Price' : 'VWAP']}
                labelStyle={{ fontSize: 11 }}
                contentStyle={{ fontSize: 11, borderRadius: 8 }}
              />
              <Line type="monotone" dataKey="vwap" stroke="#06b6d4" dot={false} strokeWidth={1.5} name="VWAP" />
              <Line type="monotone" dataKey="price" stroke="#6366f1" dot={false} strokeWidth={1.5} name="price" />
              {chartData
                .filter((d) => d.signal === 'BUY')
                .map((d, i) => (
                  <ReferenceLine key={`b${i}`} x={d.time} stroke="#22c55e" strokeDasharray="3 3" />
                ))}
              {chartData
                .filter((d) => d.signal === 'SELL')
                .map((d, i) => (
                  <ReferenceLine key={`s${i}`} x={d.time} stroke="#ef4444" strokeDasharray="3 3" />
                ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Trades table */}
      {trades.length > 0 && (
        <div className="bg-white rounded-2xl border border-slate-100 shadow-sm overflow-hidden">
          <p className="text-sm font-semibold text-slate-700 px-4 pt-4 pb-2">Угоди</p>
          <div className="divide-y divide-slate-50">
            {trades.slice(-10).reverse().map((t, i) => (
              <div key={i} className="flex items-center justify-between px-4 py-3">
                <div className="flex items-center gap-2">
                  {t.signal_type === 'BUY' ? (
                    <TrendingUpIcon className="w-4 h-4 text-green-500" />
                  ) : (
                    <TrendingDownIcon className="w-4 h-4 text-red-500" />
                  )}
                  <div>
                    <p className="text-xs font-semibold text-slate-800">
                      {t.signal_type} {t.token_symbol || activeToken}
                    </p>
                    <p className="text-[10px] text-slate-400">
                      {new Date(t.created_at).toLocaleString('uk-UA', {
                        day: '2-digit',
                        month: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <p className="text-xs text-slate-600">
                    ${t.size_usd != null ? Number(t.size_usd).toFixed(2) : '—'}
                  </p>
                  {t.tx_hash && (
                    <a
                      href={`https://explorer.mantle.xyz/tx/${t.tx_hash}`}
                      target="_blank"
                      rel="noreferrer"
                      className="text-cyan-500"
                    >
                      <ExternalLinkIcon className="w-3.5 h-3.5" />
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {loading && (
        <div className="text-center py-8 text-slate-400 text-sm">Завантаження даних агента…</div>
      )}

      {!loading && signals.length === 0 && (
        <div className="text-center py-8 text-slate-400 text-sm">
          Агент ще не генерував сигналів для {activeToken}
        </div>
      )}
    </div>
  );
}
