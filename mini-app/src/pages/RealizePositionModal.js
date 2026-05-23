import React, { useState } from 'react';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { useI18n } from '../i18n';

function formatUsd(value) {
  if (value == null || Number.isNaN(Number(value))) return '--';
  return `$${Number(value).toFixed(2)}`;
}

function formatQty(value) {
  if (value == null || Number.isNaN(Number(value))) return '--';
  return Number(value).toFixed(6).replace(/0+$/, '').replace(/\.$/, '');
}

const PRESET_TOKENS = {
  ETH: [
    { label: 'USDT', contract: '0xdAC17F958D2ee523a2206206994597C13D831ec7', symbol: 'USDT' },
    { label: 'USDC', contract: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606e48', symbol: 'USDC' },
    { label: 'DAI', contract: '0x6B175474E89094C44Da98b954EedeAC495271d0F', symbol: 'DAI' }
  ],
  BSC: [
    { label: 'USDT', contract: '0x55d398326f99059fF775485246999027B3197955', symbol: 'USDT' },
    { label: 'USDC', contract: '0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d', symbol: 'USDC' },
    { label: 'BUSD', contract: '0xe9e7cea3dedca5984780bafc599bd69add087d56', symbol: 'BUSD' }
  ]
};

export function RealizePositionModal({ token, tokens = [], chain = 'ETH', onConfirm, onCancel }) {
  const { t } = useI18n();
  const [percent, setPercent] = useState('');
  const [targetType, setTargetType] = useState(null);
  const [selectedTarget, setSelectedTarget] = useState('');
  const [customContract, setCustomContract] = useState('');
  const [error, setError] = useState('');

  const percentNum = Math.max(0, Math.min(100, Number(percent) || 0));
  const amountToRealize = (token.amount * percentNum) / 100;
  const valueToRealize = token.currentPrice * amountToRealize || 0;

  const portfolioTokens = (tokens || []).filter((tkn) => tkn.id !== token.id);
  const stables = PRESET_TOKENS[chain] || [];
  const nativeSymbol = chain === 'ETH' ? 'ETH' : 'BNB';
  const showNativeTarget = String(token.symbol || '').toUpperCase() !== nativeSymbol;

  const handleConfirm = () => {
    if (!percent || percentNum <= 0) return setError(t('error_enter_percentage'));
    if (!targetType) return setError(t('error_select_target'));
    if (targetType === 'custom' && !customContract.trim()) return setError(t('error_enter_contract'));
    if (targetType !== 'native' && targetType !== 'custom' && targetType !== 'cash' && !selectedTarget) return setError(t('error_select_target'));

    setError('');

    let targetTokenId = null;
    let targetContract = null;
    if (targetType === 'cash') targetContract = null;
    else if (targetType === 'portfolio' && selectedTarget) targetTokenId = Number(selectedTarget);
    else if (targetType === 'stable' && selectedTarget) targetContract = selectedTarget;
    else if (targetType === 'native') targetContract = 'native';
    else if (targetType === 'custom') targetContract = customContract.trim().toLowerCase();

    if (targetType === 'portfolio' && !targetTokenId) return setError(t('error_select_target'));
    if (targetType !== 'portfolio' && targetType !== 'cash' && !targetContract) return setError(t('error_select_target'));

    onConfirm({ percentage: percentNum, amountToRealize, valueToRealize, targetType, targetTokenId, targetContract });
  };

  return (
    <div className="space-y-4">
      <div>
        <label className="text-xs font-semibold text-slate-600 block mb-2">{t('realization_percentage')}</label>
        <Input type="number" min="0" max="100" placeholder="0-100" value={percent} onChange={(e) => { setPercent(e.target.value); setError(''); }} />
      </div>

      {percentNum > 0 && (
        <Card className="p-3 space-y-2 bg-emerald-50 border border-emerald-100">
          <p className="text-xs text-emerald-600 font-semibold">{t('will_realize')}</p>
          <p className="text-lg font-bold text-emerald-600">{formatUsd(valueToRealize)}</p>
          <p className="text-xs text-emerald-600">{formatQty(amountToRealize)} {token.symbol} @ {formatUsd(token.currentPrice)}</p>
        </Card>
      )}

      <div>
        <label className="text-xs font-semibold text-slate-600 block mb-2">{t('realize_to')}</label>

        <button
          onClick={() => { setTargetType('cash'); setSelectedTarget(''); setError(''); }}
          className={`w-full p-2 rounded-lg border text-xs font-medium text-left transition-all mb-3 ${
            targetType === 'cash' ? 'border-cyan-500 bg-cyan-50 text-cyan-900' : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300'
          }`}
        >
          {t('cash')}
        </button>

        {portfolioTokens.length > 0 && (
          <div className="mb-3">
            <p className="text-xs text-slate-500 font-semibold mb-2">{t('portfolio_tokens')}</p>
            <div className="grid grid-cols-2 gap-2">
              {portfolioTokens.map((tkn) => (
                <button
                  key={tkn.id}
                  onClick={() => { setTargetType('portfolio'); setSelectedTarget(String(tkn.id)); setError(''); }}
                  className={`p-2 rounded-lg border text-xs font-medium transition-all ${
                    targetType === 'portfolio' && selectedTarget === String(tkn.id)
                      ? 'border-cyan-500 bg-cyan-50 text-cyan-900'
                      : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300'
                  }`}
                >
                  {tkn.symbol}
                </button>
              ))}
            </div>
          </div>
        )}

        {showNativeTarget && (
          <button
            onClick={() => { setTargetType('native'); setSelectedTarget('native'); setError(''); }}
            className={`w-full p-2 rounded-lg border text-xs font-medium text-left transition-all mb-3 ${
              targetType === 'native' ? 'border-cyan-500 bg-cyan-50 text-cyan-900' : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300'
            }`}
          >
            {nativeSymbol}
          </button>
        )}

        {stables.length > 0 && (
          <div className="mb-3">
            <p className="text-xs text-slate-500 font-semibold mb-2">{t('stablecoins')}</p>
            <div className="grid grid-cols-2 gap-2">
              {stables.map((tkn) => (
                <button
                  key={tkn.contract}
                  onClick={() => { setTargetType('stable'); setSelectedTarget(tkn.contract); setError(''); }}
                  className={`p-2 rounded-lg border text-xs font-medium transition-all ${
                    targetType === 'stable' && selectedTarget === tkn.contract
                      ? 'border-cyan-500 bg-cyan-50 text-cyan-900'
                      : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300'
                  }`}
                >
                  {tkn.label}
                </button>
              ))}
            </div>
          </div>
        )}

        <button
          onClick={() => { setTargetType('custom'); setError(''); }}
          className={`w-full p-3 rounded-lg border text-sm font-medium text-left transition-all ${
            targetType === 'custom' ? 'border-cyan-500 bg-cyan-50 text-cyan-900' : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300'
          }`}
        >
          {t('custom_contract')}
        </button>

        {targetType === 'custom' && (
          <div className="mt-2">
            <Input placeholder="0x..." value={customContract} onChange={(e) => { setCustomContract(e.target.value); setError(''); }} />
          </div>
        )}
      </div>

      {error && <div className="text-xs text-rose-500 font-medium">{error}</div>}

      <div className="flex gap-2 pt-2">
        <Button variant="secondary" fullWidth onClick={onCancel}>{t('cancel')}</Button>
        <Button fullWidth onClick={handleConfirm} disabled={percentNum <= 0}>{t('realize_position')}</Button>
      </div>
    </div>
  );
}
