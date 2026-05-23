import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getTokenDashboard, getMarketTokenActivity } from '../api';
import { Card } from '../components/ui/Card';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { useI18n } from '../i18n';

const PAGE_SIZE = 30;

function formatValue(value) {
  if (value == null || Number.isNaN(Number(value))) return '--';
  const number = Number(value);
  if (Math.abs(number) >= 1000000) return `${(number / 1000000).toFixed(2)}M`;
  if (Math.abs(number) >= 1000) return `${(number / 1000).toFixed(2)}K`;
  return number.toFixed(2);
}

function formatSmart(value) {
  if (typeof value === 'string') return value;
  return formatValue(value);
}

function scoreTone(value, inverse = false) {
  const val = Number(value || 0);
  const normalized = inverse ? 100 - val : val;
  if (normalized >= 70) return 'text-emerald-600';
  if (normalized >= 45) return 'text-amber-600';
  return 'text-rose-600';
}

function localizeBackendText(text, t) {
  if (!text) return text;

  const exact = {
    'Not enough USD-enriched transfers yet to estimate transfer-size behavior.': 'backend_not_enough_usd_enriched',
    'No on-chain transfers were indexed for this token in the selected period (1h/24h).': 'backend_no_onchain_transfers_period',
    'No exchange flow data for the selected period.': 'backend_no_exchange_flow_period',
    'Low sample size this hour, so percentile bands are less stable.': 'backend_low_sample_size',
    'Whale-tail profile: median transfers are much smaller than top-tier transfers.': 'backend_whale_tail_profile',
    'Wide dispersion: the market mixes regular flow with periodic large transfers.': 'backend_wide_dispersion',
    'Balanced size profile: transfer amounts are relatively consistent.': 'backend_balanced_size_profile',
    'Exchange-heavy hour with net inflow to exchanges, which can signal potential sell pressure.': 'backend_exchange_inflow_sell_pressure',
    'Exchange-heavy hour with net outflow from exchanges, which can indicate accumulation/withdrawals.': 'backend_exchange_outflow_accumulation',
    'Most activity is outside exchange-tagged wallets, suggesting more organic wallet flow.': 'backend_most_activity_organic',
    'Exchange participation is moderate and does not dominate current token flow.': 'backend_exchange_participation_moderate',
    'High concentration: top holder dominates flow': 'backend_risk_high_concentration',
    'Exchange-driven hour: majority of flow tied to exchange addresses': 'backend_risk_exchange_driven',
    'Transfer size dispersion is extreme (p99 much larger than median)': 'backend_risk_dispersion_extreme',
    'High churn per wallet indicates speculative behavior': 'backend_risk_high_churn',
    'Risk profile is currently moderate': 'backend_risk_moderate'
  };

  if (exact[text]) return t(exact[text]);

  let out = text;
  const fragments = [
    ['Current hour is ', 'backend_flow_current_hour_is'],
    ['accelerating versus baseline', 'backend_flow_accelerating'],
    ['cooling down versus baseline', 'backend_flow_cooling'],
    ['flat versus recent baseline', 'backend_flow_flat'],
    ['with exchange-dominant flow', 'backend_flow_exchange_dominant'],
    ['with mostly organic wallet-to-wallet flow', 'backend_flow_organic'],
    ['with balanced exchange involvement', 'backend_flow_balanced_exchange'],
    ['; active wallets ', 'backend_flow_active_wallets'],
    [' vs 6h average and transaction load ', 'backend_flow_vs_6h_and_tx_load'],
    [' (6h avg ', 'backend_flow_6h_avg'],
    [').', 'backend_flow_end']
  ];

  fragments.forEach(([needle, key]) => {
    out = out.split(needle).join(t(key));
  });
  return out;
}

export default function MarketDashboard({ chainId = 1 }) {
  const { t } = useI18n();
  const [allTokens, setAllTokens] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState('');

  const [selected, setSelected] = useState(null);
  const [details, setDetails] = useState(null);
  const [detailsLoading, setDetailsLoading] = useState(false);

  const [searchQuery, setSearchQuery] = useState('');
  const [period, setPeriod] = useState('1h');
  const [sortBy, setSortBy] = useState('activity');
  const [sortOrder, setSortOrder] = useState('desc');
  const [filtersOpen, setFiltersOpen] = useState(true);
  const [useAllChainsFallback, setUseAllChainsFallback] = useState(false);

  const listRef = useRef(null);
  const requestSeq = useRef(0);

  const fetchPage = useCallback(async (nextOffset, replace = false) => {
    const seq = ++requestSeq.current;

    if (replace) {
      setLoading(true);
      setError('');
    } else {
      setLoadingMore(true);
    }

    try {
      const targetChain = useAllChainsFallback ? 0 : chainId;
      const response = await getMarketTokenActivity({
        chain_id: targetChain,
        q: searchQuery.trim(),
        period,
        sort_by: sortBy,
        order: sortOrder,
        limit: PAGE_SIZE,
        offset: nextOffset
      });

      if (seq !== requestSeq.current) return;

      const list = Array.isArray(response) ? response : response?.data;
      let normalized = Array.isArray(list) ? list : [];

      if (replace && nextOffset === 0 && !useAllChainsFallback && chainId !== 0 && normalized.length === 0) {
        const retry = await getMarketTokenActivity({
          chain_id: 0,
          q: searchQuery.trim(),
          period,
          sort_by: sortBy,
          order: sortOrder,
          limit: PAGE_SIZE,
          offset: 0
        });
        const retryList = Array.isArray(retry) ? retry : retry?.data;
        const fallbackNormalized = Array.isArray(retryList) ? retryList : [];
        if (fallbackNormalized.length > 0) {
          normalized = fallbackNormalized;
          setUseAllChainsFallback(true);
        }
      }

      setAllTokens((prev) => (replace ? normalized : [...prev, ...normalized]));
      setHasMore(normalized.length === PAGE_SIZE);
      setOffset(nextOffset + normalized.length);
    } catch (err) {
      if (seq !== requestSeq.current) return;
      setError(err?.message || t('failed_load_token_activity'));
      if (replace) setAllTokens([]);
      setHasMore(false);
    } finally {
      if (seq !== requestSeq.current) return;
      if (replace) setLoading(false);
      setLoadingMore(false);
    }
  }, [chainId, searchQuery, period, sortBy, sortOrder, t, useAllChainsFallback]);

  useEffect(() => {
    setAllTokens([]);
    setHasMore(true);
    setOffset(0);
    setUseAllChainsFallback(false);

    const timer = setTimeout(() => {
      fetchPage(0, true);
    }, 220);

    return () => clearTimeout(timer);
  }, [fetchPage]);

  const handleScroll = useCallback(() => {
    if (!listRef.current || loading || loadingMore || !hasMore) return;
    const el = listRef.current;
    const nearBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 140;
    if (nearBottom) {
      fetchPage(offset, false);
    }
  }, [fetchPage, hasMore, loading, loadingMore, offset]);

  const openToken = async (token) => {
    setSelected(token);
    setDetailsLoading(true);
    setDetails(null);

    try {
      const payload = await getTokenDashboard(token.chain_id || chainId, token.token);
      setDetails(payload?.data || payload || null);
    } catch (err) {
      setDetails({ error: err?.message || t('failed_load_token_details') });
    } finally {
      setDetailsLoading(false);
    }
  };

  const facts = useMemo(() => details?.facts || {}, [details]);
  const scores = useMemo(() => details?.scores || {}, [details]);
  const reasons = useMemo(() => details?.risk_reasons || [], [details]);
  const interpretation = useMemo(() => details?.interpretation || {}, [details]);
  const localizedTransferSizesText = useMemo(
    () => localizeBackendText(interpretation.transfer_sizes || '--', t),
    [interpretation.transfer_sizes, t]
  );
  const localizedExchangeFlowText = useMemo(
    () => localizeBackendText(interpretation.exchange_flow || '--', t),
    [interpretation.exchange_flow, t]
  );
  const localizedFlowNarrative = useMemo(
    () => localizeBackendText(details?.flow_narrative || '--', t),
    [details?.flow_narrative, t]
  );
  const localizedRiskReasons = useMemo(
    () => reasons.map((reason) => localizeBackendText(reason, t)),
    [reasons, t]
  );
  const localizedDataNote = useMemo(
    () => localizeBackendText(facts.data_note || '', t),
    [facts.data_note, t]
  );
  const noDashboardData = useMemo(() => {
    const state = String(details?.state_label || '').toLowerCase();
    if (state.includes('no data')) return true;
    const tx = Number(facts.tx_count ?? 0);
    const active = Number(facts.active_addresses ?? 0);
    return tx === 0 && active === 0;
  }, [details?.state_label, facts.tx_count, facts.active_addresses]);
  const localizedStateLabel = useMemo(() => {
    const raw = String(details?.state_label || '').trim();
    if (!raw) return '--';

    const normalized = raw.toLowerCase();
    const isEarlyInterest =
      normalized.includes('ранн') ||
      normalized.includes('підтвердж') ||
      normalized.includes('early') ||
      normalized.includes('interest');
    const isActivity =
      normalized.includes('актив') ||
      normalized.includes('activity') ||
      normalized.includes('active');

    if (isEarlyInterest) return t('state_early_confirmed_interest');
    if (isActivity) return t('state_activity');
    if (normalized.includes('no data')) return t('state_no_data');

    return raw;
  }, [details?.state_label, t]);

  if (loading) {
    return <div className="p-4 text-sm text-slate-400">{t('loading_token_activity')}</div>;
  }

  if (error) {
    return <div className="p-4 text-sm text-rose-500">{error}</div>;
  }

  return (
    <>
      <div className="p-3 h-[calc(100vh-210px)] min-h-[560px] flex flex-col gap-3">
        <Card className="p-3 flex-1 min-h-0 flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <p className="text-[11px] uppercase text-slate-400">{t('token_activity')}</p>
            <button
              type="button"
              onClick={() => setFiltersOpen((v) => !v)}
              className="text-[11px] px-2 py-1 rounded-md border border-slate-200 text-slate-600 hover:bg-slate-50"
            >
              {filtersOpen ? t('hide_filters') : t('show_filters')}
            </button>
          </div>

          {filtersOpen && (
            <>
              <input
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder={t('search_placeholder')}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-xs text-slate-700 outline-none focus:border-cyan-400"
              />

              <div className="grid grid-cols-3 gap-2">
                <select value={period} onChange={(e) => setPeriod(e.target.value)} className="rounded-lg border border-slate-200 px-2 py-2 text-xs text-slate-700">
                  <option value="1h">{t('last_hour')}</option>
                  <option value="24h">{t('last_day')}</option>
                </select>
                <select value={sortBy} onChange={(e) => setSortBy(e.target.value)} className="rounded-lg border border-slate-200 px-2 py-2 text-xs text-slate-700">
                  <option value="activity">{t('sort_activity')}</option>
                  <option value="health">{t('sort_health')}</option>
                  <option value="risk">{t('sort_risk')}</option>
                  <option value="tx">{t('sort_tx')}</option>
                  <option value="addresses">{t('sort_addresses')}</option>
                  <option value="symbol">{t('sort_symbol')}</option>
                  <option value="updated_at">{t('sort_updated')}</option>
                </select>
                <select value={sortOrder} onChange={(e) => setSortOrder(e.target.value)} className="rounded-lg border border-slate-200 px-2 py-2 text-xs text-slate-700">
                  <option value="desc">{t('desc')}</option>
                  <option value="asc">{t('asc')}</option>
                </select>
              </div>
            </>
          )}

          <div ref={listRef} onScroll={handleScroll} className="flex-1 min-h-0 overflow-y-auto space-y-2 pr-1">
            {useAllChainsFallback && (
              <div className="text-[11px] text-amber-700 bg-amber-50 border border-amber-200 rounded-md px-2 py-1">
                {t('all_chains_fallback')}
              </div>
            )}
            {allTokens.length ? (
              allTokens.map((token) => (
                <Card key={`all-${token.chain_id}-${token.token}`} className="p-3 flex items-center gap-3" onClick={() => openToken(token)}>
                  <div className="w-8 h-8 rounded-full bg-slate-100 text-slate-700 flex items-center justify-center text-xs font-bold">
                    {(token.symbol || '?').slice(0, 2).toUpperCase()}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-semibold text-slate-800 truncate">{token.symbol || token.name || token.token}</p>
                    <p className="text-[11px] text-slate-400 truncate">{token.name || token.token}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-[11px] text-slate-500">
                      <span title={t('title_tx_count_period')}>{t('tx_short')} {token.tx_count ?? '--'}</span>
                      {' | '}
                      <span title={t('title_addr_unique_period')}>{t('addr_short')} {token.unique_addresses ?? '--'}</span>
                    </p>
                    {(token.tx_count ?? 0) === 0 && (token.unique_addresses ?? 0) === 0 && (
                      <p className="text-[10px] text-amber-600">{t('no_activity_period')}</p>
                    )}
                    <p className="text-[11px] text-slate-500">
                      <span title={t('title_activity_score')}>A {formatValue(token.activity_score)}</span>
                      {' | '}
                      <span title={t('title_health_score')}>H {formatValue(token.health_score)}</span>
                    </p>
                  </div>
                </Card>
              ))
            ) : (
              <div className="text-xs text-slate-400">{t('no_tokens_match')}</div>
            )}

            {loadingMore && <div className="text-xs text-slate-400 py-2">{t('loading_more_tokens')}</div>}
            {!hasMore && allTokens.length > 0 && <div className="text-xs text-slate-400 py-2">{t('end_of_list')}</div>}
          </div>
        </Card>
      </div>

      <Modal isOpen={selected !== null} onClose={() => setSelected(null)} title={selected?.symbol || t('token_label')}>
        {detailsLoading && <div className="text-sm text-slate-400">{t('loading_token_details')}</div>}

        {!detailsLoading && details?.error && <div className="text-sm text-rose-500">{details.error}</div>}

        {!detailsLoading && !details?.error && details && (
          <div className="space-y-3">
            <Card className="p-3">
              <p className="text-[11px] uppercase text-slate-400">{t('state')}</p>
              <p className="text-sm font-semibold text-slate-800">{localizedStateLabel}</p>
              <p className="text-[11px] text-slate-500 mt-1">
                {t('signal')}: <span className="font-semibold">{details.signal_strength || 'n/a'}</span>
              </p>
            </Card>

            {noDashboardData && (
              <Card className="p-3 border border-amber-200 bg-amber-50">
                <p className="text-[11px] uppercase text-amber-700">{t('token_data_unavailable_title')}</p>
                <p className="text-xs text-amber-900 leading-relaxed">
                  {localizedDataNote || t('token_data_unavailable_text')}
                </p>
              </Card>
            )}

            <Card className="p-3 space-y-1">
              <p className="text-[11px] uppercase text-slate-400">{t('how_to_read')}</p>
              <p className="text-xs text-slate-700">{t('how_to_read_tx')}</p>
              <p className="text-xs text-slate-700">{t('how_to_read_addr')}</p>
              <p className="text-xs text-slate-700">{t('how_to_read_a_h')}</p>
              <p className="text-xs text-slate-700">{t('how_to_read_p')}</p>
            </Card>

            <div className="grid grid-cols-2 gap-2">
              <Card className="p-3">
                <p className="text-[11px] text-slate-400" title={t('title_health_score_card')}>{t('health_score')}</p>
                <p className={`text-sm font-semibold ${scoreTone(scores.health)}`}>{scores.health ?? '--'}</p>
              </Card>
              <Card className="p-3">
                <p className="text-[11px] text-slate-400" title={t('title_risk_score_card')}>{t('risk_score')}</p>
                <p className={`text-sm font-semibold ${scoreTone(scores.risk, true)}`}>{scores.risk ?? '--'}</p>
              </Card>
              <Card className="p-3">
                <p className="text-[11px] text-slate-400" title={t('title_addr_unique_period')}>{t('active_addresses')}</p>
                <p className="text-sm font-semibold text-slate-800">{facts.active_addresses ?? '--'}</p>
              </Card>
              <Card className="p-3">
                <p className="text-[11px] text-slate-400" title={t('title_tx_count_period')}>{t('transactions')}</p>
                <p className="text-sm font-semibold text-slate-800">{facts.tx_count ?? '--'}</p>
              </Card>
              <Card className="p-3">
                <p className="text-[11px] text-slate-400" title={t('title_exchange_ratio')}>{t('exchange_ratio')}</p>
                <p className="text-sm font-semibold text-slate-800">{formatValue(facts.exchange_ratio)}%</p>
              </Card>
              <Card className="p-3">
                <p className="text-[11px] text-slate-400" title={t('title_net_exchange_usd')}>{t('net_exchange_usd')}</p>
                <p className="text-sm font-semibold text-slate-800">{formatValue(facts.net_exchange)}</p>
              </Card>
            </div>

            <Card className="p-3 space-y-1">
              <p className="text-[11px] uppercase text-slate-400" title={t('title_transfer_sizes')}>{t('transfer_sizes')}</p>
              <p className="text-xs text-slate-600" title={t('title_p50')}>P50 ({details?.symbol || t('token_label')}): {formatSmart(facts.p50_token_qty)} {t('tokens_unit')}</p>
              <p className="text-xs text-slate-600" title={t('title_p95')}>P95 ({details?.symbol || t('token_label')}): {formatSmart(facts.p95_token_qty)} {t('tokens_unit')}</p>
              <p className="text-xs text-slate-600" title={t('title_p99')}>P99 ({details?.symbol || t('token_label')}): {formatSmart(facts.p99_token_qty)} {t('tokens_unit')}</p>
              {String(facts.p50_token_qty || '--') === '--' &&
                String(facts.p95_token_qty || '--') === '--' &&
                String(facts.p99_token_qty || '--') === '--' && (
                  <p className="text-xs text-slate-500">{t('not_enough_data_period')}</p>
                )}
              <p className="text-xs text-slate-700 pt-1">{localizedTransferSizesText}</p>
            </Card>

            {facts.data_note && !noDashboardData && (
              <Card className="p-3">
                <p className="text-[11px] uppercase text-slate-400">{t('data_availability')}</p>
                <p className="text-xs text-slate-700 leading-relaxed">{localizedDataNote}</p>
              </Card>
            )}

            <Card className="p-3 space-y-1">
              <p className="text-[11px] uppercase text-slate-400">{t('exchange_flow_meaning')}</p>
              <p className="text-xs text-slate-700 leading-relaxed">{localizedExchangeFlowText}</p>
            </Card>

            <Card className="p-3 space-y-1">
              <p className="text-[11px] uppercase text-slate-400">{t('flow_narrative')}</p>
              <p className="text-xs text-slate-700 leading-relaxed">{localizedFlowNarrative}</p>
            </Card>

            <Card className="p-3 space-y-1">
              <p className="text-[11px] uppercase text-slate-400">{t('risk_reasons')}</p>
              {localizedRiskReasons.length ? (
                localizedRiskReasons.map((reason, idx) => (
                  <p key={`${reason}-${idx}`} className="text-xs text-slate-700">
                    {idx + 1}. {reason}
                  </p>
                ))
              ) : (
                <p className="text-xs text-slate-500">{t('no_critical_risk')}</p>
              )}
            </Card>

            <Button fullWidth variant="secondary" onClick={() => setSelected(null)}>
              {t('close')}
            </Button>
          </div>
        )}
      </Modal>
    </>
  );
}
