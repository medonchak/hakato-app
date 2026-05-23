import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { SearchIcon, ArrowUpRightIcon } from 'lucide-react';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { SkeletonCard, SkeletonTokenCard } from '../components/ui/Skeleton';
import { fetchAddressAnalytics } from '../api';
import { useI18n } from '../i18n';

const container = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { staggerChildren: 0.06 } }
};

const item = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.4, ease: 'easeOut' } }
};

export function AddressAnalytics() {
  const { t } = useI18n();
  const [address, setAddress] = useState('');
  const [state, setState] = useState('idle');
  const [analytics, setAnalytics] = useState(null);

  const handleAnalyze = async () => {
    if (!address.trim()) return;
    setState('loading');
    const data = await fetchAddressAnalytics(address.trim());
    setAnalytics(data);
    setState(data ? 'results' : 'idle');
  };

  const dynamicRows = analytics
    ? Object.entries(analytics).filter(([key]) => !['total_tx', 'total_gas'].includes(key))
    : [];

  return (
    <motion.div variants={container} initial="hidden" animate="show" className="space-y-5">
      <motion.div variants={item} className="pt-2">
        <h1 className="text-xl font-bold text-slate-800">{t('address_analytics')}</h1>
        <p className="text-sm text-slate-400 mt-0.5">{t('analyze_wallet_chain')}</p>
      </motion.div>

      <motion.div variants={item} className="flex justify-center py-4">
        <div className="relative">
          <div className="w-20 h-20 rounded-2xl bg-gradient-to-br from-cyan-100 to-teal-100 flex items-center justify-center">
            <SearchIcon className="w-8 h-8 text-cyan-500" />
          </div>
          <div className="absolute -top-1 -right-1 w-6 h-6 rounded-lg bg-gradient-to-br from-violet-400 to-fuchsia-400 flex items-center justify-center">
            <span className="text-white text-[8px] font-bold">0x</span>
          </div>
        </div>
      </motion.div>

      <motion.div variants={item} className="space-y-3">
        <Input
          label={t('wallet_address')}
          placeholder="0x742d35Cc6634C0532925a3b844Bc9e7595f..."
          value={address}
          onChange={(e) => setAddress(e.target.value)}
        />
        <Button fullWidth onClick={handleAnalyze} disabled={!address.trim() || state === 'loading'}>
          <SearchIcon className="w-4 h-4" />
          {state === 'loading' ? t('analyzing') : t('analyze')}
        </Button>
      </motion.div>

      <AnimatePresence mode="wait">
        {state === 'idle' && (
          <motion.div key="idle" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="text-center py-8">
            <p className="text-sm text-slate-400">{t('enter_wallet_prompt')}</p>
          </motion.div>
        )}

        {state === 'loading' && (
          <motion.div key="loading" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} className="space-y-3">
            <SkeletonCard />
            <SkeletonTokenCard />
            <SkeletonTokenCard />
            <SkeletonCard />
          </motion.div>
        )}

        {state === 'results' && (
          <motion.div key="results" variants={container} initial="hidden" animate="show" className="space-y-4">
            <motion.div variants={item}>
              <Card variant="highlighted" className="p-5">
                <h3 className="text-xs font-medium text-slate-400 uppercase tracking-wide mb-3">{t('address_summary')}</h3>
                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <p className="text-lg font-bold text-slate-800 tabular-nums">{analytics?.total_tx ?? '--'}</p>
                    <p className="text-[10px] text-slate-400 mt-0.5">{t('transactions')}</p>
                  </div>
                  <div>
                    <p className="text-lg font-bold text-slate-800 tabular-nums">{analytics?.total_gas ?? '--'}</p>
                    <p className="text-[10px] text-slate-400 mt-0.5">{t('total_gas')}</p>
                  </div>
                  <div>
                    <p className="text-lg font-bold text-slate-800 tabular-nums">{address.slice(0, 6)}...{address.slice(-4)}</p>
                    <p className="text-[10px] text-slate-400 mt-0.5">{t('address')}</p>
                  </div>
                </div>
              </Card>
            </motion.div>

            <motion.div variants={item}>
              <h3 className="text-sm font-bold text-slate-700 mb-3">{t('additional_metrics')}</h3>
              <Card className="p-4 space-y-3">
                {dynamicRows.length === 0 ? (
                  <div className="text-sm text-slate-500">{t('no_additional_fields')}</div>
                ) : (
                  dynamicRows.map(([key, value]) => (
                    <div className="flex items-center gap-3" key={key}>
                      <div className="w-7 h-7 rounded-lg bg-cyan-50 flex items-center justify-center flex-shrink-0">
                        <ArrowUpRightIcon className="w-3.5 h-3.5 text-cyan-500" />
                      </div>
                      <div className="flex-1">
                        <p className="text-xs font-medium text-slate-700">{key}</p>
                      </div>
                      <p className="text-xs font-semibold text-slate-700 tabular-nums">{String(value)}</p>
                    </div>
                  ))
                )}
              </Card>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="h-20" />
    </motion.div>
  );
}
