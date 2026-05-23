import React, { useEffect, useState } from 'react';
import { PencilIcon, RefreshCwIcon, Trash2Icon } from 'lucide-react';
import { getRuleFilters, deleteAlertFilter, updateAlertFilter } from '../api';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { useI18n } from '../i18n';

function getCategories(filter = {}, t) {
  const categories = [];
  if (filter?.swap?.enabled) categories.push(t('swap'));
  if (filter?.swapFinance?.enabled) categories.push(t('finance'));
  return categories;
}

function toNumberOrEmpty(value) {
  if (value === '') return '';
  const number = Number(value);
  return Number.isNaN(number) ? '' : number;
}

export default function UserRulesModal({ idrule, refreshKey = 0 }) {
  const { t } = useI18n();
  const [filters, setFilters] = useState([]);
  const [loading, setLoading] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [editFilter, setEditFilter] = useState(null);
  const [saving, setSaving] = useState(false);

  const normalizeRows = (data) => {
    if (!Array.isArray(data)) return [];
    return data.map((row) => ({ id: row?.id ?? row?.ID, filter: row?.filter ?? row?.Filter ?? {} }));
  };

  const loadFilters = async () => {
    if (!idrule) {
      setFilters([]);
      return;
    }
    setLoading(true);
    try {
      const data = await getRuleFilters(idrule);
      setFilters(normalizeRows(data));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadFilters();
  }, [idrule, refreshKey]);

  const handleDelete = async (id) => {
    await deleteAlertFilter(id);
    await loadFilters();
  };

  const startEdit = (row) => {
    setEditingId(row.id);
    setEditFilter({ ...(row.filter || {}) });
    setEditOpen(true);
  };

  const closeEdit = () => {
    setEditingId(null);
    setEditFilter(null);
    setEditOpen(false);
  };

  const saveEdit = async () => {
    if (!editingId || !editFilter) return;
    setSaving(true);
    try {
      await updateAlertFilter(editingId, editFilter);
      closeEdit();
      await loadFilters();
    } finally {
      setSaving(false);
    }
  };

  const setSwapEnabled = (enabled) => {
    setEditFilter((prev) => ({
      ...prev,
      swap: {
        ...(prev?.swap || {}),
        enabled,
        minAmount: prev?.swap?.minAmount || '',
        currency: prev?.swap?.currency || 'USDT',
        tokens: Array.isArray(prev?.swap?.tokens) ? prev.swap.tokens : []
      }
    }));
  };

  const setFinanceEnabled = (enabled) => {
    setEditFilter((prev) => ({
      ...prev,
      swapFinance: {
        ...(prev?.swapFinance || {}),
        enabled,
        minUsd: prev?.swapFinance?.minUsd || '',
        allow: {
          sellNative: prev?.swapFinance?.allow?.sellNative || false,
          buyNative: prev?.swapFinance?.allow?.buyNative || false,
          buyAnyNative: prev?.swapFinance?.allow?.buyAnyNative || false,
          buyAnyStable: prev?.swapFinance?.allow?.buyAnyStable || false
        }
      }
    }));
  };

  if (loading) return <div className="p-4 text-sm text-slate-500">{t('loading_filters')}</div>;

  return (
    <div className="space-y-2.5 max-h-[70vh] overflow-y-auto">
      <Card className="!p-3 bg-slate-50/70 border border-slate-200">
        <div className="flex items-center justify-between">
          <p className="text-sm font-semibold text-slate-700">{t('my_rules')} ({filters.length})</p>
          <button type="button" onClick={loadFilters} className="inline-flex items-center gap-1.5 text-xs font-medium text-cyan-700 hover:text-cyan-800">
            <RefreshCwIcon className="w-3.5 h-3.5" />
            {t('refresh')}
          </button>
        </div>
      </Card>

      {filters.length === 0 && (
        <Card className="!p-4">
          <p className="text-sm text-slate-500">{t('no_filters_rule')}</p>
        </Card>
      )}

      {filters.map((row) => {
        const categories = getCategories(row.filter, t);
        return (
          <Card key={row.id} className="!p-3 space-y-2 border border-slate-200">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="text-[11px] uppercase tracking-wide text-slate-400">{t('address')}</p>
                <p className="text-sm font-semibold text-slate-700 break-all">{row.filter?.address || '--'}</p>
              </div>
              <div className="flex items-center gap-1">
                <button type="button" onClick={() => startEdit(row)} className="p-1.5 rounded-lg border border-slate-200 text-slate-500 hover:text-slate-700" aria-label={t('edit_filter')}>
                  <PencilIcon className="w-3.5 h-3.5" />
                </button>
                <button type="button" onClick={() => handleDelete(row.id)} className="p-1.5 rounded-lg border border-slate-200 text-rose-500 hover:text-rose-600" aria-label={t('delete')}>
                  <Trash2Icon className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div className="flex flex-wrap gap-1.5">
              {categories.length > 0 ? (
                categories.map((category) => (
                  <span key={`${row.id}-${category}`} className="text-[11px] px-2 py-0.5 rounded-full bg-cyan-50 text-cyan-700 border border-cyan-100">{category}</span>
                ))
              ) : (
                <span className="text-[11px] px-2 py-0.5 rounded-full bg-slate-100 text-slate-500 border border-slate-200">{t('inactive')}</span>
              )}
            </div>

            <div className="text-xs text-slate-500 space-y-0.5">
              {row.filter?.swap?.enabled && <p>{t('swap_min')}: {row.filter?.swap?.minAmount || '--'} {row.filter?.swap?.currency || ''}</p>}
              {row.filter?.swapFinance?.enabled && <p>{t('finance_min')}: ${row.filter?.swapFinance?.minUsd || '--'}</p>}
            </div>
          </Card>
        );
      })}

      <Modal isOpen={editOpen} onClose={closeEdit} title={t('edit_filter')}>
        {editFilter && (
          <div className="space-y-3">
            <Input label={t('address')} value={editFilter?.address || ''} onChange={(event) => setEditFilter((prev) => ({ ...prev, address: event.target.value }))} />

            <Card className="!p-3 space-y-2">
              <label className="text-sm font-medium text-slate-700 flex items-center gap-2">
                <input type="checkbox" checked={!!editFilter?.swap?.enabled} onChange={(event) => setSwapEnabled(event.target.checked)} />
                {t('swap')}
              </label>
              {!!editFilter?.swap?.enabled && (
                <Input
                  label={t('swap_minimum')}
                  type="number"
                  value={editFilter?.swap?.minAmount || ''}
                  onChange={(event) =>
                    setEditFilter((prev) => ({
                      ...prev,
                      swap: {
                        ...(prev?.swap || {}),
                        enabled: true,
                        minAmount: toNumberOrEmpty(event.target.value),
                        currency: prev?.swap?.currency || 'USDT',
                        tokens: Array.isArray(prev?.swap?.tokens) ? prev.swap.tokens : []
                      }
                    }))
                  }
                />
              )}
            </Card>

            <Card className="!p-3 space-y-2">
              <label className="text-sm font-medium text-slate-700 flex items-center gap-2">
                <input type="checkbox" checked={!!editFilter?.swapFinance?.enabled} onChange={(event) => setFinanceEnabled(event.target.checked)} />
                {t('financial_operations')}
              </label>
              {!!editFilter?.swapFinance?.enabled && (
                <Input
                  label={t('min_usd')}
                  type="number"
                  value={editFilter?.swapFinance?.minUsd || ''}
                  onChange={(event) =>
                    setEditFilter((prev) => ({
                      ...prev,
                      swapFinance: {
                        ...(prev?.swapFinance || {}),
                        enabled: true,
                        minUsd: toNumberOrEmpty(event.target.value),
                        allow: {
                          sellNative: prev?.swapFinance?.allow?.sellNative || false,
                          buyNative: prev?.swapFinance?.allow?.buyNative || false,
                          buyAnyNative: prev?.swapFinance?.allow?.buyAnyNative || false,
                          buyAnyStable: prev?.swapFinance?.allow?.buyAnyStable || false
                        }
                      }
                    }))
                  }
                />
              )}
            </Card>

            <div className="flex gap-2 pt-1">
              <Button variant="secondary" fullWidth onClick={closeEdit}>{t('cancel')}</Button>
              <Button fullWidth onClick={saveEdit} disabled={saving}>{saving ? t('saving') : t('save')}</Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
