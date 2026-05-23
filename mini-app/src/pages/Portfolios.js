import React, { useState } from 'react';
import { motion } from 'framer-motion';
import { PlusIcon, Trash2Icon } from 'lucide-react';
import { PortfolioCard } from '../components/ui/PortfolioCard';
import { Card } from '../components/ui/Card';
import { Modal } from '../components/ui/Modal';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';
import { createPortfolio } from '../api';
import { useI18n } from '../i18n';

const container = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { staggerChildren: 0.07 } }
};

const item = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.4, ease: 'easeOut' } }
};

export function Portfolios({ userId, portfolios, onSelectPortfolio, onChanged }) {
  const { t } = useI18n();
  const [createOpen, setCreateOpen] = useState(false);
  const [portfolioName, setPortfolioName] = useState('');
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState(null);
  const [confirmId, setConfirmId] = useState(null);
  const { deletePortfolio } = require('../api');

  const handleDelete = async (id) => {
    setDeletingId(id);
    try {
      await deletePortfolio(id);
      onChanged?.();
    } finally {
      setDeletingId(null);
      setConfirmId(null);
    }
  };

  const handleCreate = async () => {
    const name = portfolioName.trim();
    if (!name || !userId) return;

    setCreating(true);
    try {
      await createPortfolio(userId, name);
      setPortfolioName('');
      setCreateOpen(false);
      onChanged?.();
    } finally {
      setCreating(false);
    }
  };

  return (
    <>
      <motion.div variants={container} initial="hidden" animate="show" className="space-y-4">
        <motion.div variants={item} className="pt-2">
          <h1 className="text-xl font-bold text-slate-800">{t('my_portfolios')}</h1>
          <p className="text-sm text-slate-400 mt-0.5">{t('track_crypto_investments')}</p>
        </motion.div>

        {portfolios?.map((portfolio) => (
          <motion.div variants={item} key={portfolio.id} className="relative group">
            <PortfolioCard
              name={portfolio.name}
              totalValue={portfolio.totalValue}
              invested={portfolio.totalInvested}
              pnl={portfolio.totalPnL}
              pnlPercent={portfolio.pnlPercent}
              onClick={() => onSelectPortfolio(portfolio)}
            />
            <button
              className="absolute bottom-3 right-4 p-2 rounded-xl hover:bg-rose-50 transition-colors group flex-shrink-0"
              title={t('delete')}
              disabled={deletingId === portfolio.id}
              onClick={(e) => {
                e.stopPropagation();
                setConfirmId(portfolio.id);
              }}
              style={{ zIndex: 2 }}
              aria-label={`${t('delete')} ${portfolio.name}`}
            >
              <Trash2Icon
                className={`w-4 h-4 text-slate-300 group-hover:text-rose-500 transition-colors ${
                  deletingId === portfolio.id ? 'animate-spin text-rose-300' : ''
                }`}
              />
            </button>
            <Modal isOpen={!!confirmId} onClose={() => setConfirmId(null)} title={t('delete_confirm_title')}>
              <div className="space-y-4">
                <div className="text-center text-slate-700 text-base font-semibold">{t('delete_portfolio_confirm')}</div>
                <div className="flex gap-2 mt-4">
                  <Button variant="secondary" fullWidth onClick={() => setConfirmId(null)}>
                    {t('cancel')}
                  </Button>
                  <Button fullWidth variant="danger" onClick={() => handleDelete(confirmId)} disabled={deletingId === confirmId}>
                    {deletingId === confirmId ? t('deleting') : t('delete')}
                  </Button>
                </div>
              </div>
            </Modal>
          </motion.div>
        ))}

        <motion.div variants={item}>
          <Card variant="dashed" onClick={() => setCreateOpen(true)} className="flex items-center justify-center gap-2 py-6">
            <div className="w-8 h-8 rounded-full bg-slate-100 flex items-center justify-center">
              <PlusIcon className="w-4 h-4 text-slate-400" />
            </div>
            <span className="text-sm font-medium text-slate-400">{t('add_portfolio')}</span>
          </Card>
        </motion.div>

        <div className="h-20" />
      </motion.div>

      <Modal isOpen={createOpen} onClose={() => setCreateOpen(false)} title={t('new_portfolio')}>
        <div className="space-y-4">
          <Input
            label={t('portfolio_name')}
            placeholder={t('my_portfolios')}
            value={portfolioName}
            onChange={(event) => setPortfolioName(event.target.value)}
          />
          <div className="flex gap-2">
            <Button variant="secondary" fullWidth onClick={() => setCreateOpen(false)}>
              {t('cancel')}
            </Button>
            <Button fullWidth onClick={handleCreate} disabled={!portfolioName.trim() || !userId || creating}>
              {creating ? t('creating') : t('create')}
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
