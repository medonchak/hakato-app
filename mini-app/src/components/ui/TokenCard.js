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

export function TokenCard({ name, symbol, price, amount, buyPrice, unrealized, color }) {
  const { t } = useI18n();
  const isProfit = unrealized >= 0;
  const initials = symbol.slice(0, 2).toUpperCase();
  return (
    <motion.div whileTap={{ scale: 0.98 }} className="bg-white rounded-2xl border border-slate-100/60 shadow-sm p-4">
      <div className="flex items-center gap-3">
        <div
          className="w-11 h-11 rounded-full flex items-center justify-center text-white text-sm font-bold flex-shrink-0"
          style={{ backgroundColor: color }}
          aria-hidden="true"
        >
          {initials}
        </div>

        <div className="flex-1 min-w-0">
          <p className="text-sm font-semibold text-slate-800 truncate">{name}</p>
          <p className="text-xs text-slate-400">{symbol}</p>
        </div>

        <div className="text-right flex-shrink-0">
          <p className="text-sm font-semibold text-slate-800 tabular-nums">{formatUsd(price)}</p>
          <div
            className={`flex items-center justify-end gap-0.5 text-xs font-medium ${
              isProfit ? 'text-emerald-500' : 'text-rose-500'
            }`}
          >
            {isProfit ? <ArrowUpRightIcon className="w-3 h-3" /> : <ArrowDownRightIcon className="w-3 h-3" />}
            <span className="tabular-nums">{formatUsd(Math.abs(unrealized))}</span>
          </div>
        </div>
      </div>

      <div className="mt-3 pt-3 border-t border-slate-50 flex items-center gap-4 text-xs text-slate-400">
        <span>
          {t('amount')}: <span className="text-slate-600 font-medium tabular-nums">{amount}</span>
        </span>
        <span>
          {t('buy_price')}: <span className="text-slate-600 font-medium tabular-nums">{formatUsd(buyPrice)}</span>
        </span>
      </div>
    </motion.div>
  );
}
