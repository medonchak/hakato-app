import React, { useState } from 'react';
import { WalletIcon, RefreshCwIcon, XIcon, CheckIcon, AlertCircleIcon } from 'lucide-react';
import { useWallet } from '../hooks/useWallet';

const API = process.env.REACT_APP_API_URL || 'http://localhost:8080';

const TOKENS = ['MNT', 'mETH', 'USDY', 'USDC', 'USDT'];
const PCT_PRESETS = [25, 50, 75, 100];

function shortAddr(addr) {
  return addr ? `${addr.slice(0, 6)}…${addr.slice(-4)}` : '';
}

export function WalletPanel({ onTradeConfigSaved }) {
  const { address, isMantle, balances, connecting, error, connect, disconnect, refreshBalances } = useWallet();

  const [tradeToken, setTradeToken]   = useState('USDC');
  const [tradeAmount, setTradeAmount] = useState('');
  const [saving, setSaving]           = useState(false);
  const [saved, setSaved]             = useState(false);
  const [saveError, setSaveError]     = useState(null);

  const selectedBalance = parseFloat(balances[tradeToken] ?? '0');

  function applyPct(pct) {
    if (!selectedBalance) return;
    const val = (selectedBalance * pct) / 100;
    setTradeAmount(val.toFixed(4));
  }

  async function saveTradeConfig() {
    if (!tradeAmount || parseFloat(tradeAmount) <= 0) return;
    setSaving(true);
    setSaveError(null);
    try {
      const res = await fetch(`${API}/api/agent/trade-config`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          chain_id:    5000,
          token:       tradeToken,
          amount_usd:  parseFloat(tradeAmount),
          wallet_addr: address,
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
      onTradeConfigSaved?.({ token: tradeToken, amount: parseFloat(tradeAmount) });
    } catch (e) {
      setSaveError(e.message);
    } finally {
      setSaving(false);
    }
  }

  // ── Not connected ─────────────────────────────────────────────────
  if (!address) {
    return (
      <div className="bg-white rounded-2xl border border-slate-100 shadow-sm p-4">
        <div className="flex items-center gap-2 mb-3">
          <WalletIcon className="w-4 h-4 text-cyan-500" />
          <span className="text-sm font-semibold text-slate-700">Гаманець</span>
        </div>

        {error && (
          <div className="flex items-start gap-2 bg-red-50 rounded-xl p-3 mb-3">
            <AlertCircleIcon className="w-4 h-4 text-red-500 mt-0.5 shrink-0" />
            <p className="text-xs text-red-600">{error}</p>
          </div>
        )}

        <button
          onClick={connect}
          disabled={connecting}
          className="w-full py-3 rounded-xl bg-cyan-500 text-white text-sm font-semibold
                     hover:bg-cyan-600 active:scale-95 transition-all disabled:opacity-50"
        >
          {connecting ? 'Підключення…' : 'Підключити MetaMask'}
        </button>
        <p className="text-[10px] text-slate-400 text-center mt-2">
          Mantle Network (chainId 5000)
        </p>
      </div>
    );
  }

  // ── Wrong network ─────────────────────────────────────────────────
  if (!isMantle) {
    return (
      <div className="bg-amber-50 rounded-2xl border border-amber-200 p-4">
        <div className="flex items-center gap-2 mb-2">
          <AlertCircleIcon className="w-4 h-4 text-amber-500" />
          <span className="text-sm font-semibold text-amber-700">Невірна мережа</span>
        </div>
        <p className="text-xs text-amber-600 mb-3">Переключись на Mantle (chainId 5000)</p>
        <button
          onClick={connect}
          className="w-full py-2.5 rounded-xl bg-amber-400 text-white text-sm font-semibold hover:bg-amber-500 transition-colors"
        >
          Переключитись на Mantle
        </button>
      </div>
    );
  }

  // ── Connected ─────────────────────────────────────────────────────
  return (
    <div className="bg-white rounded-2xl border border-slate-100 shadow-sm p-4 space-y-4">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-green-400" />
          <span className="text-xs font-semibold text-slate-700">{shortAddr(address)}</span>
          <span className="text-[10px] text-slate-400 bg-cyan-50 px-1.5 py-0.5 rounded-full">Mantle</span>
        </div>
        <div className="flex gap-2">
          <button onClick={refreshBalances} className="text-slate-400 hover:text-cyan-500 transition-colors">
            <RefreshCwIcon className="w-3.5 h-3.5" />
          </button>
          <button onClick={disconnect} className="text-slate-400 hover:text-red-400 transition-colors">
            <XIcon className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Balances */}
      <div className="grid grid-cols-3 gap-2">
        {TOKENS.map((tok) => (
          <div
            key={tok}
            onClick={() => setTradeToken(tok)}
            className={`rounded-xl p-2 text-center cursor-pointer transition-all border ${
              tradeToken === tok
                ? 'border-cyan-400 bg-cyan-50'
                : 'border-transparent bg-slate-50 hover:bg-slate-100'
            }`}
          >
            <p className="text-[10px] text-slate-400">{tok}</p>
            <p className="text-xs font-bold text-slate-800 truncate">
              {balances[tok] != null ? Number(balances[tok]).toFixed(3) : '—'}
            </p>
          </div>
        ))}
      </div>

      {/* Amount selector */}
      <div>
        <p className="text-xs text-slate-500 mb-1.5">
          Сума угоди — <span className="font-semibold text-slate-700">{tradeToken}</span>
          {selectedBalance > 0 && (
            <span className="text-slate-400"> (баланс: {selectedBalance.toFixed(4)})</span>
          )}
        </p>

        {/* Percent presets */}
        <div className="flex gap-1.5 mb-2">
          {PCT_PRESETS.map((pct) => (
            <button
              key={pct}
              onClick={() => applyPct(pct)}
              disabled={!selectedBalance}
              className="flex-1 py-1 rounded-lg bg-slate-100 text-xs font-semibold text-slate-600
                         hover:bg-cyan-50 hover:text-cyan-600 active:scale-95 transition-all disabled:opacity-40"
            >
              {pct}%
            </button>
          ))}
        </div>

        {/* Manual input */}
        <div className="relative">
          <input
            type="number"
            value={tradeAmount}
            onChange={(e) => setTradeAmount(e.target.value)}
            placeholder="0.00"
            min="0"
            step="0.01"
            className="w-full rounded-xl border border-slate-200 bg-slate-50 px-3 py-2.5 text-sm
                       text-slate-800 placeholder-slate-400 focus:outline-none focus:border-cyan-400
                       focus:bg-white transition-colors pr-16"
          />
          <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-slate-400 font-medium">
            {tradeToken}
          </span>
        </div>

        {tradeAmount && selectedBalance > 0 && parseFloat(tradeAmount) > selectedBalance && (
          <p className="text-[10px] text-red-500 mt-1">Перевищує баланс</p>
        )}
      </div>

      {/* Save button */}
      {saveError && (
        <p className="text-[10px] text-red-500">{saveError}</p>
      )}

      <button
        onClick={saveTradeConfig}
        disabled={
          saving ||
          !tradeAmount ||
          parseFloat(tradeAmount) <= 0 ||
          (selectedBalance > 0 && parseFloat(tradeAmount) > selectedBalance)
        }
        className={`w-full py-3 rounded-xl text-sm font-semibold transition-all active:scale-95
          ${saved
            ? 'bg-green-500 text-white'
            : 'bg-cyan-500 text-white hover:bg-cyan-600 disabled:opacity-50'
          }`}
      >
        {saved ? (
          <span className="flex items-center justify-center gap-1.5">
            <CheckIcon className="w-4 h-4" /> Збережено
          </span>
        ) : saving ? 'Збереження…' : 'Застосувати суму'}
      </button>
    </div>
  );
}
