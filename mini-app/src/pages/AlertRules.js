import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { PlusIcon } from 'lucide-react';
import { RuleCard } from '../components/ui/RuleCard';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { deleteAlertRule } from '../api';
import { createRuleName } from '../api';
import UserRulesModal from '../comp/UserRulesModal';
import { Input } from '../components/ui/Input';
import { StyledAlertFilterForm } from '../components/StyledAlertFilterForm';
import { useI18n } from '../i18n';

const container = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { staggerChildren: 0.07 } }
};

const item = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.4, ease: 'easeOut' } }
};

export function AlertRules({ rules = [], userId, telegramId, onChanged }) {
  const { t } = useI18n();
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [selectedRule, setSelectedRule] = useState(null);
  const [showAddFilter, setShowAddFilter] = useState(false);
  const [filtersRefreshKey, setFiltersRefreshKey] = useState(0);
  const [createOpen, setCreateOpen] = useState(false);
  const [ruleName, setRuleName] = useState('');
  const [creating, setCreating] = useState(false);

  const getRuleId = (rule) => rule?.rule_id ?? rule?.id;

  const handleDelete = (id) => {
    setDeleteTarget(id);
  };

  const confirmDelete = async () => {
    if (deleteTarget) {
      await deleteAlertRule(deleteTarget);
      onChanged?.();
      setDeleteTarget(null);
    }
  };

  const handleCreateRule = async () => {
    const name = ruleName.trim();
    if (!name || !telegramId) return;

    setCreating(true);
    try {
      await createRuleName(telegramId, name);
      setRuleName('');
      setCreateOpen(false);
      onChanged?.();
    } finally {
      setCreating(false);
    }
  };

  return (
    <>
      <motion.div variants={container} initial="hidden" animate="show" className="space-y-4">
        <motion.div variants={item} className="pt-2 flex items-center justify-between">
          <div>
            <h1 className="text-xl font-bold text-slate-800">{t('alert_rules')}</h1>
            <p className="text-sm text-slate-400 mt-0.5">{t('monitor_onchain_activity')}</p>
          </div>
          <span className="text-xs font-medium text-slate-400 bg-slate-100 px-2.5 py-1 rounded-lg">
            {rules.length} {t('active_count')}
          </span>
        </motion.div>

        <AnimatePresence>
          {rules.map((rule) => (
            <motion.div
              key={getRuleId(rule)}
              variants={item}
              exit={{ opacity: 0, x: -20, transition: { duration: 0.2 } }}
              layout
            >
              <div onClick={() => setSelectedRule(rule)}>
                <RuleCard
                  address={rule.address || rule.name || `${t('rule_name')} #${getRuleId(rule)}`}
                  ruleType={rule.name || t('rule_name')}
                  ruleTypeVariant={rule.newCount > 0 ? 'success' : 'neutral'}
                  onDelete={() => handleDelete(getRuleId(rule))}
                />
              </div>
            </motion.div>
          ))}
        </AnimatePresence>

        {rules.length === 0 && (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="text-center py-12">
            <div className="w-14 h-14 rounded-2xl bg-slate-100 flex items-center justify-center mx-auto mb-3">
              <span className="text-2xl">??</span>
            </div>
            <p className="text-sm font-medium text-slate-600">{t('no_alert_rules')}</p>
            <p className="text-xs text-slate-400 mt-1">{t('create_first_rule')}</p>
          </motion.div>
        )}

        <motion.div variants={item} className="pt-2">
          <Button fullWidth onClick={() => setCreateOpen(true)}>
            <PlusIcon className="w-4 h-4" />
            {t('add_rule')}
          </Button>
        </motion.div>

        <div className="h-20" />
      </motion.div>

      <Modal isOpen={deleteTarget !== null} onClose={() => setDeleteTarget(null)} title={t('delete_rule')}>
        <div className="space-y-4">
          <p className="text-sm text-slate-600">{t('delete_rule_confirm')}</p>
          <div className="flex gap-3 pt-2">
            <Button variant="secondary" fullWidth onClick={() => setDeleteTarget(null)}>
              {t('cancel')}
            </Button>
            <Button variant="danger" fullWidth onClick={confirmDelete}>
              {t('delete')}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={createOpen} onClose={() => setCreateOpen(false)} title={t('new_alert_rule')}>
        <div className="space-y-4">
          <Input
            label={t('rule_name')}
            placeholder={t('whale_swaps')}
            value={ruleName}
            onChange={(event) => setRuleName(event.target.value)}
          />

          <div className="flex gap-3 pt-2">
            <Button variant="secondary" fullWidth onClick={() => setCreateOpen(false)}>
              {t('cancel')}
            </Button>
            <Button fullWidth onClick={handleCreateRule} disabled={!ruleName.trim() || !telegramId || creating}>
              {creating ? t('creating') : t('create')}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        isOpen={selectedRule !== null}
        onClose={() => {
          setSelectedRule(null);
          setShowAddFilter(false);
        }}
        title={selectedRule ? selectedRule.name : t('rule_name')}
      >
        {selectedRule && (
          <div className="space-y-3">
            <div className="rounded-xl border border-slate-200 p-2 space-y-2">
              <div className="flex items-center justify-between">
                <p className="text-sm font-semibold text-slate-700">{t('add_filter')}</p>
                <Button variant="secondary" className="!px-3 !py-1.5 !text-xs" onClick={() => setShowAddFilter((value) => !value)}>
                  {showAddFilter ? t('hide') : t('show')}
                </Button>
              </div>

              {showAddFilter && (
                <StyledAlertFilterForm
                  ruleId={getRuleId(selectedRule)}
                  userId={userId}
                  onSaved={() => {
                    setShowAddFilter(false);
                    setFiltersRefreshKey((value) => value + 1);
                    onChanged?.();
                  }}
                />
              )}
            </div>

            <div className="rounded-xl border border-slate-200 p-2">
              <UserRulesModal idrule={getRuleId(selectedRule)} modalAlertFormUpdate={showAddFilter} refreshKey={filtersRefreshKey} />
            </div>
          </div>
        )}
      </Modal>
    </>
  );
}
