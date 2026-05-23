import React from 'react';
import { motion } from 'framer-motion';
import {
  EyeIcon,
  ZapIcon,
  ShieldIcon,
  TrendingUpIcon,
  ArrowUpRightIcon,
  ArrowDownRightIcon
} from 'lucide-react';
import { Card } from '../components/ui/Card';
import MarketDashboard from '../comp/MarketDashboard';
import { useI18n } from '../i18n';

const container = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { staggerChildren: 0.06 } }
};

const item = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.4, ease: 'easeOut' } }
};

export function Dashboard({ displayUser, portfolioCount = 0, alertsCount = 0 }) {
  const { lang, setLang, t } = useI18n();
  const username = displayUser?.username ? `@${displayUser.username}` : displayUser?.first_name || '@user';
  const initials = (displayUser?.first_name || 'U').slice(0, 2).toUpperCase();

  return (
    <motion.div variants={container} initial="hidden" animate="show" className="space-y-5">
      <motion.div variants={item} className="pt-2">
        <div className="flex items-center justify-between mb-1">
          <div className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-cyan-400 to-teal-400 flex items-center justify-center">
            <EyeIcon className="w-4 h-4 text-white" />
          </div>
          <div>
            <h1 className="text-lg font-bold text-slate-800 leading-tight">{t('crypto_dashboard')}</h1>
            <p className="text-[11px] text-slate-400 font-medium tracking-wide uppercase">{t('blockchain_eye')}</p>
          </div>
          </div>
          <div className="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-1">
            <button
              type="button"
              onClick={() => setLang('uk')}
              className={`px-2 py-1 text-[10px] font-semibold rounded ${lang === 'uk' ? 'bg-slate-800 text-white' : 'text-slate-600'}`}
            >
              UA
            </button>
            <button
              type="button"
              onClick={() => setLang('en')}
              className={`px-2 py-1 text-[10px] font-semibold rounded ${lang === 'en' ? 'bg-slate-800 text-white' : 'text-slate-600'}`}
            >
              EN
            </button>
          </div>
        </div>
      </motion.div>

      <motion.div variants={item}>
        <Card className="flex items-center gap-3 p-4">
          <div className="w-11 h-11 rounded-full bg-gradient-to-br from-violet-400 to-fuchsia-400 flex items-center justify-center text-white font-bold text-sm flex-shrink-0">
            {initials}
          </div>
          <div className="flex-1">
            <p className="text-sm font-semibold text-slate-800">{username}</p>
            <p className="text-xs text-slate-400">{t('telegram_connected')}</p>
          </div>
          <div className="w-2 h-2 rounded-full bg-emerald-400 flex-shrink-0" aria-label="Online" />
        </Card>
      </motion.div>

      <motion.div variants={item} className="grid grid-cols-3 gap-3">
        <Card className="p-3 text-center">
          <p className="text-[10px] font-medium text-slate-400 uppercase tracking-wide mb-1">{t('total_value')}</p>
          <p className="text-lg font-bold text-slate-800 tabular-nums">{portfolioCount}</p>
          <div className="flex items-center justify-center gap-0.5 text-emerald-500 text-[10px] font-semibold mt-0.5">
            <ArrowUpRightIcon className="w-3 h-3" />
            <span>{t('portfolios')}</span>
          </div>
        </Card>

        <Card className="p-3 text-center">
          <p className="text-[10px] font-medium text-slate-400 uppercase tracking-wide mb-1">{t('change_24h')}</p>
          <p className="text-lg font-bold text-emerald-500 tabular-nums">{alertsCount}</p>
          <div className="flex items-center justify-center gap-0.5 text-emerald-500 text-[10px] font-semibold mt-0.5">
            <ArrowUpRightIcon className="w-3 h-3" />
            <span>{t('rules')}</span>
          </div>
        </Card>

        <Card className="p-3 text-center">
          <p className="text-[10px] font-medium text-slate-400 uppercase tracking-wide mb-1">{t('alerts')}</p>
          <p className="text-lg font-bold text-slate-800 tabular-nums">{alertsCount}</p>
          <p className="text-[10px] text-slate-400 font-medium mt-0.5">{t('active')}</p>
        </Card>
      </motion.div>

      <motion.div variants={item}>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-bold text-slate-700">{t('token_activity')}</h2>
          <span className="text-xs text-slate-400 font-medium">{t('live')}</span>
        </div>
        <Card className="p-0 overflow-hidden">
          <MarketDashboard chainId={1} />
        </Card>
      </motion.div>

      {/* <motion.div variants={item}>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-bold text-slate-700">Market Overview</h2>
          <span className="text-xs text-slate-400 font-medium">Trending</span>
        </div>
        <div className="space-y-2.5">
          <Card className="flex items-center gap-3 p-3.5">
            <div className="w-9 h-9 rounded-full bg-blue-500 flex items-center justify-center text-white text-xs font-bold flex-shrink-0">ET</div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-semibold text-slate-800">Ethereum</p>
              <p className="text-xs text-slate-400">ETH</p>
            </div>
            <div className="text-right flex-shrink-0">
              <p className="text-sm font-semibold text-slate-800 tabular-nums">$3,245.50</p>
              <div className="flex items-center justify-end gap-0.5 text-emerald-500 text-xs font-medium">
                <TrendingUpIcon className="w-3 h-3" />
                <span className="tabular-nums">+3.2%</span>
              </div>
            </div>
          </Card>

          <Card className="flex items-center gap-3 p-3.5">
            <div className="w-9 h-9 rounded-full bg-orange-500 flex items-center justify-center text-white text-xs font-bold flex-shrink-0">BT</div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-semibold text-slate-800">Bitcoin</p>
              <p className="text-xs text-slate-400">BTC</p>
            </div>
            <div className="text-right flex-shrink-0">
              <p className="text-sm font-semibold text-slate-800 tabular-nums">$67,842.00</p>
              <div className="flex items-center justify-end gap-0.5 text-emerald-500 text-xs font-medium">
                <TrendingUpIcon className="w-3 h-3" />
                <span className="tabular-nums">+1.8%</span>
              </div>
            </div>
          </Card>

          <Card className="flex items-center gap-3 p-3.5">
            <div className="w-9 h-9 rounded-full bg-purple-500 flex items-center justify-center text-white text-xs font-bold flex-shrink-0">SO</div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-semibold text-slate-800">Solana</p>
              <p className="text-xs text-slate-400">SOL</p>
            </div>
            <div className="text-right flex-shrink-0">
              <p className="text-sm font-semibold text-slate-800 tabular-nums">$148.20</p>
              <div className="flex items-center justify-end gap-0.5 text-rose-500 text-xs font-medium">
                <ArrowDownRightIcon className="w-3 h-3" />
                <span className="tabular-nums">-0.8%</span>
              </div>
            </div>
          </Card>
        </div>
      </motion.div> */}

      {/* <motion.div variants={item}>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-bold text-slate-700">Quick Actions</h2>
        </div>
        <div className="grid grid-cols-3 gap-3">
          <Card className="p-3 flex flex-col items-center gap-2 text-center">
            <div className="w-9 h-9 rounded-xl bg-cyan-50 flex items-center justify-center">
              <ZapIcon className="w-4 h-4 text-cyan-500" />
            </div>
            <span className="text-[11px] font-medium text-slate-600">Analyze</span>
          </Card>

          <Card className="p-3 flex flex-col items-center gap-2 text-center">
            <div className="w-9 h-9 rounded-xl bg-violet-50 flex items-center justify-center">
              <ShieldIcon className="w-4 h-4 text-violet-500" />
            </div>
            <span className="text-[11px] font-medium text-slate-600">Alerts</span>
          </Card>

          <Card className="p-3 flex flex-col items-center gap-2 text-center">
            <div className="w-9 h-9 rounded-xl bg-amber-50 flex items-center justify-center">
              <TrendingUpIcon className="w-4 h-4 text-amber-500" />
            </div>
            <span className="text-[11px] font-medium text-slate-600">Trends</span>
          </Card>
        </div>
      </motion.div> */}

      <div className="h-20" />
    </motion.div>
  );
}
