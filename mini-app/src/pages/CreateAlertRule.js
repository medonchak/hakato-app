import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ChevronLeftIcon, CheckIcon } from 'lucide-react';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { createRuleName } from '../api';
import { useI18n } from '../i18n';

const networks = [
  { id: 'ethereum', name: 'Ethereum', color: '#627EEA' },
  { id: 'polygon', name: 'Polygon', color: '#8247E5' },
  { id: 'bsc', name: 'BSC', color: '#F3BA2F' }
];

export function CreateAlertRule({ onBack, telegramId, onCreated }) {
  const { t } = useI18n();
  const [network, setNetwork] = useState('ethereum');
  const [address, setAddress] = useState('');
  const [tokenSwaps, setTokenSwaps] = useState(false);
  const [financialOps, setFinancialOps] = useState(false);
  const [swapMinValue, setSwapMinValue] = useState('');
  const [opsThreshold, setOpsThreshold] = useState('');
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);

  const canSave = address.trim().length > 0 && (tokenSwaps || financialOps);

  const handleSave = async () => {
    if (!canSave || !telegramId) return;
    setSaving(true);
    try {
      const ruleName = `${network.toUpperCase()} ${address.slice(0, 6)}...`;
      await createRuleName(telegramId, ruleName);
      onCreated?.();
      setSaved(true);
    } finally {
      setSaving(false);
    }
  };

  if (saved) {
    return (
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="flex flex-col items-center justify-center py-20 space-y-4"
      >
        <div className="w-16 h-16 rounded-full bg-gradient-to-br from-cyan-400 to-teal-400 flex items-center justify-center">
          <CheckIcon className="w-8 h-8 text-white" />
        </div>
        <h2 className="text-xl font-bold text-slate-800">{t('rule_created')}</h2>
        <p className="text-sm text-slate-400 text-center max-w-[240px]">
          {t('rule_saved_notify')}
        </p>
        <Button onClick={onBack} className="mt-4">
          {t('back_to_rules')}
        </Button>
      </motion.div>
    );
  }

  return (
    <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="space-y-5">
      <div className="pt-1">
        <button
          onClick={onBack}
          className="flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 transition-colors mb-3 -ml-1"
          aria-label={t('back_to_rules')}
        >
          <ChevronLeftIcon className="w-4 h-4" />
          <span>{t('alert_rules')}</span>
        </button>
        <h1 className="text-xl font-bold text-slate-800">{t('create_alert_rule')}</h1>
        <p className="text-sm text-slate-400 mt-0.5">{t('setup_onchain_monitoring')}</p>
      </div>

      <div className="space-y-2.5">
        <label className="block text-sm font-medium text-slate-600">
          <span className="inline-flex items-center gap-2">
            <span className="w-5 h-5 rounded-full bg-gradient-to-br from-cyan-400 to-teal-400 flex items-center justify-center text-white text-[10px] font-bold">
              1
            </span>
            {t('network')}
          </span>
        </label>
        <div className="grid grid-cols-3 gap-2.5">
          {networks.map((net) => (
            <Card
              key={net.id}
              variant={network === net.id ? 'highlighted' : 'default'}
              onClick={() => setNetwork(net.id)}
              className={`p-3 flex flex-col items-center gap-2 ${network === net.id ? 'ring-2 ring-cyan-300' : ''}`}
            >
              <div
                className="w-8 h-8 rounded-full flex items-center justify-center text-white text-[10px] font-bold"
                style={{ backgroundColor: net.color }}
              >
                {net.name.slice(0, 2).toUpperCase()}
              </div>
              <span className="text-xs font-medium text-slate-700">{net.name}</span>
            </Card>
          ))}
        </div>
      </div>

      <div className="space-y-2.5">
        <label className="block text-sm font-medium text-slate-600">
          <span className="inline-flex items-center gap-2">
            <span className="w-5 h-5 rounded-full bg-gradient-to-br from-cyan-400 to-teal-400 flex items-center justify-center text-white text-[10px] font-bold">
              2
            </span>
            {t('wallet_address')}
          </span>
        </label>
        <Input
          placeholder="0x742d35Cc6634C0532925a3b844Bc..."
          value={address}
          onChange={(e) => setAddress(e.target.value)}
        />
      </div>

      <div className="space-y-2.5">
        <label className="block text-sm font-medium text-slate-600">
          <span className="inline-flex items-center gap-2">
            <span className="w-5 h-5 rounded-full bg-gradient-to-br from-cyan-400 to-teal-400 flex items-center justify-center text-white text-[10px] font-bold">
              3
            </span>
            {t('rule_types')}
          </span>
        </label>

        <div className="space-y-2.5">
          <Card
            onClick={() => setTokenSwaps(!tokenSwaps)}
            className={`p-4 flex items-center gap-3 ${tokenSwaps ? 'ring-2 ring-cyan-300 border-cyan-200' : ''}`}
          >
            <div
              className={`w-5 h-5 rounded-md border-2 flex items-center justify-center flex-shrink-0 transition-colors ${
                tokenSwaps ? 'bg-cyan-400 border-cyan-400' : 'border-slate-300'
              }`}
            >
              {tokenSwaps && <CheckIcon className="w-3 h-3 text-white" />}
            </div>
            <div>
              <p className="text-sm font-medium text-slate-700">{t('token_swap')}</p>
              <p className="text-xs text-slate-400">{t('monitor_dex_swaps')}</p>
            </div>
          </Card>

          <AnimatePresence>
            {tokenSwaps && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                transition={{ duration: 0.25, ease: 'easeInOut' }}
                className="overflow-hidden"
              >
                <div className="pl-8 pb-1">
                  <Input
                    label={t('min_swap_value_usd')}
                    placeholder="e.g. 1000"
                    type="number"
                    value={swapMinValue}
                    onChange={(e) => setSwapMinValue(e.target.value)}
                  />
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          <Card
            onClick={() => setFinancialOps(!financialOps)}
            className={`p-4 flex items-center gap-3 ${financialOps ? 'ring-2 ring-cyan-300 border-cyan-200' : ''}`}
          >
            <div
              className={`w-5 h-5 rounded-md border-2 flex items-center justify-center flex-shrink-0 transition-colors ${
                financialOps ? 'bg-cyan-400 border-cyan-400' : 'border-slate-300'
              }`}
            >
              {financialOps && <CheckIcon className="w-3 h-3 text-white" />}
            </div>
            <div>
              <p className="text-sm font-medium text-slate-700">{t('financial_operations')}</p>
              <p className="text-xs text-slate-400">{t('lending_borrowing_staking')}</p>
            </div>
          </Card>

          <AnimatePresence>
            {financialOps && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                transition={{ duration: 0.25, ease: 'easeInOut' }}
                className="overflow-hidden"
              >
                <div className="pl-8 pb-1">
                  <Input
                    label={t('alert_threshold_usd')}
                    placeholder="e.g. 5000"
                    type="number"
                    value={opsThreshold}
                    onChange={(e) => setOpsThreshold(e.target.value)}
                  />
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>

      <div className="pt-3">
        <Button fullWidth onClick={handleSave} disabled={!canSave || !telegramId || saving}>
          {saving ? t('saving') : t('save_rule')}
        </Button>
      </div>

      <div className="h-24" />
    </motion.div>
  );
}
