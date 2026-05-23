import React from 'react';
import { motion } from 'framer-motion';
import { Trash2Icon } from 'lucide-react';
import { Badge } from './Badge';
import { useI18n } from '../../i18n';

function shortenAddress(addr) {
  if (addr.length <= 12) return addr;
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

export function RuleCard({ address, ruleType, ruleTypeVariant, onDelete }) {
  const { t } = useI18n();
  return (
    <motion.div
      layout
      className="bg-white rounded-2xl border border-slate-100/60 shadow-sm p-4 flex items-center gap-3"
    >
      <div className="w-9 h-9 rounded-full bg-gradient-to-br from-cyan-100 to-teal-100 flex items-center justify-center flex-shrink-0">
        <span className="text-xs font-bold text-teal-600">0x</span>
      </div>

      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-slate-700 font-mono tabular-nums truncate">
          {shortenAddress(address)}
        </p>
        <div className="mt-1">
          <Badge variant={ruleTypeVariant}>{ruleType}</Badge>
        </div>
      </div>

      <button
        onClick={(e) => {
          e.stopPropagation();
          onDelete();
        }}
        className="p-2 rounded-xl hover:bg-rose-50 transition-colors group flex-shrink-0"
        aria-label={`${t('delete_rule')}: ${shortenAddress(address)}`}
      >
        <Trash2Icon className="w-4 h-4 text-slate-300 group-hover:text-rose-500 transition-colors" />
      </button>
    </motion.div>
  );
}
