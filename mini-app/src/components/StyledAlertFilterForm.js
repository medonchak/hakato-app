import React, { useState } from 'react';
import { Input } from './ui/Input';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { sendAlerts } from '../api';
import { useI18n } from '../i18n';

function isPositive(value) {
  return value !== '' && !Number.isNaN(Number(value)) && Number(value) > 0;
}

export function StyledAlertFilterForm({ ruleId, userId, onSaved }) {
  const { t } = useI18n();
  const telegramId = window.Telegram?.WebApp?.initDataUnsafe?.user?.id || 574595536;

  const getCreatorForServer = () => {
    const tg = Number(telegramId);
    if (Number.isInteger(tg) && tg > 0 && tg <= 2147483647) return tg;
    const uid = Number(userId);
    if (Number.isInteger(uid) && uid > 0) return uid;
    return null;
  };

  const [address, setAddress] = useState('');
  const [alertChaine, setAlertChaine] = useState('ETH');
  const [swapEnabled, setSwapEnabled] = useState(false);
  const [swapMin, setSwapMin] = useState('');
  const [swapCurr, setSwapCurr] = useState('USDT');
  const [swapTokens, setSwapTokens] = useState([]);
  const [swapTokenInput, setSwapTokenInput] = useState('');
  const [financeEnabled, setFinanceEnabled] = useState(false);
  const [minUSD, setMinUSD] = useState('');
  const [sellNative, setSellNative] = useState(false);
  const [buyNative, setBuyNative] = useState(false);
  const [buyAnyNative, setBuyAnyNative] = useState(false);
  const [buyAnyStable, setBuyAnyStable] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const addSwapToken = () => {
    const token = swapTokenInput.trim().toLowerCase();
    if (!token) return;
    if (!swapTokens.includes(token)) setSwapTokens((prev) => [...prev, token]);
    setSwapTokenInput('');
  };

  const removeSwapToken = (token) => {
    setSwapTokens((prev) => prev.filter((item) => item !== token));
  };

  const resetForm = () => {
    setAddress('');
    setAlertChaine('ETH');
    setSwapEnabled(false);
    setSwapMin('');
    setSwapCurr('USDT');
    setSwapTokens([]);
    setSwapTokenInput('');
    setFinanceEnabled(false);
    setMinUSD('');
    setSellNative(false);
    setBuyNative(false);
    setBuyAnyNative(false);
    setBuyAnyStable(false);
    setError('');
  };

  const validate = () => {
    if (!address.trim()) return t('error_address_required');
    if (!swapEnabled && !financeEnabled) return t('error_enable_condition');
    if (swapEnabled && !isPositive(swapMin)) return t('error_swap_min');
    if (financeEnabled && !isPositive(minUSD)) return t('error_finance_min');
    if (financeEnabled && !sellNative && !buyNative && !buyAnyNative && !buyAnyStable) return t('error_select_finance_op');
    return '';
  };

  const onSubmit = async () => {
    const ruleIdNum = Number(ruleId);
    if (!ruleIdNum) {
      setError('Invalid rule ID');
      return;
    }

    const creatorForServer = getCreatorForServer();
    if (!creatorForServer) {
      setError('Invalid user ID');
      return;
    }

    const validationError = validate();
    setError(validationError);
    if (validationError) return;

    const payload = {
      creator: String(creatorForServer),
      ruleId: ruleIdNum,
      address: address.trim(),
      alertChaine
    };

    if (swapEnabled) {
      payload.swap = {
        enabled: true,
        minAmount: swapMin || '',
        currency: swapCurr,
        tokens: swapTokens
      };
    }

    if (financeEnabled) {
      payload.swapFinance = {
        enabled: true,
        minUsd: minUSD,
        allow: {
          sellNative,
          buyNative,
          buyAnyNative,
          buyAnyStable
        }
      };
    }

    setSaving(true);
    try {
      const res = await sendAlerts(payload);
      if (!res?.ok) {
        setError(res?.error || t('save_failed'));
        return;
      }
      resetForm();
      onSaved?.();
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-3">
      <div className="text-[11px] text-slate-400">{t('rule_id')}: {Number(ruleId) || '--'}</div>
      <Input label={t('address')} placeholder="0x..." value={address} onChange={(event) => setAddress(event.target.value)} />

      <Card className="p-3 space-y-2">
        <p className="text-xs font-semibold text-slate-600">{t('network')}</p>
        <div className="grid grid-cols-2 gap-2">
          <Button variant={alertChaine === 'ETH' ? 'primary' : 'secondary'} onClick={() => setAlertChaine('ETH')}>Ethereum</Button>
          <Button variant={alertChaine === 'BSC' ? 'primary' : 'secondary'} onClick={() => setAlertChaine('BSC')}>BSC</Button>
        </div>
      </Card>

      <Card className="p-3 space-y-2">
        <label className="flex items-center gap-2 text-sm font-medium text-slate-700">
          <input type="checkbox" checked={swapEnabled} onChange={(event) => setSwapEnabled(event.target.checked)} />
          {t('token_swap')}
        </label>

        {swapEnabled && (
          <>
            <Input label={t('swap_minimum')} type="number" value={swapMin} onChange={(event) => setSwapMin(event.target.value)} />
            <div className="grid grid-cols-2 gap-2">
              <Button variant={swapCurr === 'USDT' ? 'primary' : 'secondary'} onClick={() => setSwapCurr('USDT')}>USDT</Button>
              <Button variant={swapCurr === (alertChaine === 'ETH' ? 'ETH' : 'BNB') ? 'primary' : 'secondary'} onClick={() => setSwapCurr(alertChaine === 'ETH' ? 'ETH' : 'BNB')}>
                {alertChaine === 'ETH' ? 'ETH' : 'BNB'}
              </Button>
            </div>
            <Input label={t('swap_token_contract')} placeholder="0x..." value={swapTokenInput} onChange={(event) => setSwapTokenInput(event.target.value)} />
            <Button variant="secondary" onClick={addSwapToken}>{t('add_token')}</Button>
            {swapTokens.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {swapTokens.map((token) => (
                  <button type="button" key={token} onClick={() => removeSwapToken(token)} className="text-xs px-2 py-1 rounded-lg border border-slate-200 bg-slate-50 text-slate-600">
                    {token}
                  </button>
                ))}
              </div>
            )}
          </>
        )}
      </Card>

      <Card className="p-3 space-y-2">
        <label className="flex items-center gap-2 text-sm font-medium text-slate-700">
          <input type="checkbox" checked={financeEnabled} onChange={(event) => setFinanceEnabled(event.target.checked)} />
          {t('financial_operations')}
        </label>

        {financeEnabled && (
          <>
            <Input label={t('minimum_usd')} type="number" value={minUSD} onChange={(event) => setMinUSD(event.target.value)} />
            <div className="grid grid-cols-2 gap-2 text-xs text-slate-700">
              <label><input type="checkbox" checked={sellNative} onChange={(event) => setSellNative(event.target.checked)} /> {t('sell_native')}</label>
              <label><input type="checkbox" checked={buyNative} onChange={(event) => setBuyNative(event.target.checked)} /> {t('buy_native')}</label>
              <label><input type="checkbox" checked={buyAnyNative} onChange={(event) => setBuyAnyNative(event.target.checked)} /> {t('buy_any_native')}</label>
              <label><input type="checkbox" checked={buyAnyStable} onChange={(event) => setBuyAnyStable(event.target.checked)} /> {t('buy_any_stable')}</label>
            </div>
          </>
        )}
      </Card>

      {error && <p className="text-xs text-rose-500">{error}</p>}

      <Button fullWidth onClick={onSubmit} disabled={saving}>{saving ? t('saving') : t('save_filter')}</Button>
    </div>
  );
}
