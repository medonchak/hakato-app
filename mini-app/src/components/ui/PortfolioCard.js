import React from 'react';
import { motion } from 'framer-motion';
import { ArrowUpRightIcon, ArrowDownRightIcon } from 'lucide-react';
import { useI18n } from '../../i18n';

function formatUsd(value) {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2
  }).format(value);
}

export function PortfolioCard({ name, totalValue, invested, pnl, pnlPercent, onClick }) {
  const { t } = useI18n();
  const isProfit = pnl >= 0;
  return (
    <motion.div
      whileTap={{ scale: 0.98 }}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
      className="bg-white rounded-2xl border border-slate-100/60 shadow-sm p-5 cursor-pointer"
    >
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-slate-600">{name}</h3>
        <div
          className={`flex items-center gap-1 rounded-lg px-2.5 py-1 text-xs font-semibold ${
            isProfit ? 'bg-emerald-50 text-emerald-600' : 'bg-rose-50 text-rose-600'
          }`}
        >
          {isProfit ? <ArrowUpRightIcon className="w-3 h-3" /> : <ArrowDownRightIcon className="w-3 h-3" />}
          <span className="tabular-nums">
            {isProfit ? '+' : ''}
            {pnlPercent.toFixed(1)}%
          </span>
        </div>
      </div>

      <p className={`text-2xl font-bold tabular-nums mb-1 ${isProfit ? 'text-emerald-500' : 'text-rose-500'}`}>{formatUsd(totalValue)}</p>

      <div className="flex items-center gap-4 text-xs">
        <span className="text-slate-400">
          {t('invested')}: <span className="text-slate-600 font-medium tabular-nums">{formatUsd(invested)}</span>
        </span>
        <span className="font-medium tabular-nums text-slate-800">
          {isProfit ? '+' : ''}
          {formatUsd(pnl)}
        </span>
      </div>
    </motion.div>
  );
}
