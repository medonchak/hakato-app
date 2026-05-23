import React, { useEffect, useMemo, useState } from 'react';
import { ChevronLeftIcon, PlusIcon, ActivityIcon, InfoIcon } from 'lucide-react';
import {
  addTokenToPortfolio,
  getPortfolioAssets,
  getPortfolioNotifications,
  portfolioOperation,
  togglePortfolioAnomalyAlerts,
  updatePortfolioAssetNetworkTracking,
  updatePortfolioAssetTracking
} from '../api';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import TokenDashboard from '../comp/TokenDashboard';
import { RealizePositionModal } from './RealizePositionModal';
import { useI18n } from '../i18n';

function formatUsd(value) {
  if (value == null || Number.isNaN(Number(value))) return '—';
  return `$${Number(value).toFixed(2)}`;
}

function formatQty(value) {
  if (value == null || Number.isNaN(Number(value))) return '—';
  return Number(value).toFixed(6).replace(/0+$/, '').replace(/\.$/, '');
}

function formatCompact(value) {
  if (value == null || Number.isNaN(Number(value))) return '—';
  const num = Number(value);
  
  if (num >= 1_000_000_000) {
    return (num / 1_000_000_000).toFixed(1) + 'B';
  }
  if (num >= 1_000_000) {
    return (num / 1_000_000).toFixed(1) + 'M';
  }
  if (num >= 1_000) {
    return (num / 1_000).toFixed(1) + 'K';
  }
  
  return num.toFixed(2);
}

export function PortfolioDetails({ portfolio, onBack, onChanged }) {
  const { t } = useI18n();
  const [assets, setAssets] = useState([]);
  const [notifications, setNotifications] = useState([]);
  const [loading, setLoading] = useState(true);
  const [addTokenStep, setAddTokenStep] = useState(null); // null | 'select' | 'contract' | 'amounts'
  const [tokenModal, setTokenModal] = useState(null);
  const [realizeToken, setRealizeToken] = useState(null);
  const [trackingEnabled, setTrackingEnabled] = useState(!!portfolio?.anomalyAlertsEnabled);
  const [trackingLoading, setTrackingLoading] = useState(false);
  const [assetTrackingLoading, setAssetTrackingLoading] = useState({});
  const [networkTrackingLoading, setNetworkTrackingLoading] = useState({});
  const [expandedAssets, setExpandedAssets] = useState({});
  const [showTrackingInfo, setShowTrackingInfo] = useState(false);
  const [addTokenError, setAddTokenError] = useState('');

  const [newToken, setNewToken] = useState({
    chain: 'ETH',
    contract: '',
    amount: '',
    invested: ''
  });

  const PRESET_TOKENS = {
    ETH: [
      { label: "USDT", contract: "0xdAC17F958D2ee523a2206206994597C13D831ec7", symbol: "USDT" },
      { label: "USDC", contract: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606e48", symbol: "USDC" },
      { label: "DAI", contract: "0x6B175474E89094C44Da98b954EedeAC495271d0F", symbol: "DAI" },
      { label: "ETH", contract: "native", symbol: "ETH" },
    ],
    BSC: [
      { label: "USDT", contract: "0x55d398326f99059fF775485246999027B3197955", symbol: "USDT" },
      { label: "USDC", contract: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", symbol: "USDC" },
      { label: "BUSD", contract: "0xe9e7cea3dedca5984780bafc599bd69add087d56", symbol: "BUSD" },
      { label: "BNB", contract: "native", symbol: "BNB" },
    ],
  };

  const getPresetContractBySymbol = (chain, symbol) => {
    const entry = (PRESET_TOKENS[chain] || []).find((t) => t.symbol === symbol);
    return entry ? entry.contract : null;
  };

  const loadPortfolioState = async () => {
    if (!portfolio?.id) return;

    setLoading(true);
    try {
      const [assetData, notificationData] = await Promise.all([
        getPortfolioAssets(portfolio.id),
        getPortfolioNotifications(portfolio.id)
      ]);

      const normalizedAssets = Array.isArray(assetData)
        ? assetData.map((asset) => ({
            assetKey: asset.asset_key,
            symbol: asset.symbol,
            portfolioTrackingEnabled: !!asset.portfolio_tracking_enabled,
            assetTrackingEnabled: !!asset.asset_tracking_enabled,
            trackingEnabled: !!asset.tracking_enabled,
            dailyReportEnabled: !!asset.daily_report_enabled,
            trackingProfile: asset.tracking_profile || 'balanced',
            totalAmount: Number(asset.total_amount || 0),
            totalInvested: Number(asset.total_invested || 0),
            totalValue: Number(asset.total_value || 0),
            totalRealized: Number(asset.total_realized || 0),
            totalPnL: Number(asset.total_pnl || 0),
            trackedNetworks: Number(asset.tracked_networks || 0),
            networkCount: Number(asset.network_count || 0),
            networks: Array.isArray(asset.networks)
              ? asset.networks.map((network) => ({
                  id: network.id,
                  chainId: Number(network.chain_id || 0),
                  chain: network.chain,
                  contract: network.contract,
                  symbol: network.symbol || asset.symbol,
                  amount: Number(network.amount || 0),
                  invested: Number(network.invested || 0),
                  realized: Number(network.realized || 0),
                  buyPriceUSD: Number(network.buy_price_usd || 0),
                  currentPriceUSD: Number(network.current_price_usd || 0),
                  value: Number(network.value || 0),
                  totalPnL: Number(network.total_pnl || 0),
                  networkTrackingEnabled: !!network.network_tracking_enabled,
                  trackingEnabled: !!network.tracking_enabled
                }))
              : []
          }))
        : [];

      setAssets(normalizedAssets);
      setNotifications(Array.isArray(notificationData) ? notificationData : []);
    } finally {
      setLoading(false);
    }
  };

  const tokens = useMemo(
    () =>
      assets.flatMap((asset) =>
        asset.networks.map((network) => ({
          id: network.id,
          symbol: asset.symbol,
          amount: network.amount,
          invested: network.invested,
          realized: network.realized,
          buyPrice: network.buyPriceUSD,
          currentPrice: network.currentPriceUSD,
          chainId: network.chainId,
          chain: network.chain,
          contract: network.contract,
          assetKey: asset.assetKey
        }))
      ),
    [assets]
  );

  useEffect(() => {
    loadPortfolioState();
    // Не робимо постійний рефреш - це викликає мигтіння
    // Рефреш буде лише при додаванні/зміні токена
  }, [portfolio?.id]);

  useEffect(() => {
    setTrackingEnabled(!!portfolio?.anomalyAlertsEnabled);
  }, [portfolio?.anomalyAlertsEnabled]);

  const totals = useMemo(() => {
    const totalValue = assets.reduce((sum, asset) => sum + asset.totalValue, 0);
    const totalInvested = assets.reduce((sum, asset) => sum + asset.totalInvested, 0);
    const totalRealized = assets.reduce((sum, asset) => sum + asset.totalRealized, 0);
    const totalPnL = assets.reduce((sum, asset) => sum + asset.totalPnL, 0);
    return { totalValue, totalInvested, totalRealized, totalPnL };
  }, [assets]);

  const handleAddToken = async () => {
    if (!portfolio?.id) return;

    if (!newToken.contract.trim() || Number(newToken.amount) <= 0 || Number(newToken.invested) <= 0) {
      setAddTokenError(t('error_fill_contract_amount_invested'));
      return;
    }

    setAddTokenError('');

    const res = await addTokenToPortfolio({
      portfolioId: portfolio.id,
      chain: newToken.chain,
      contract: newToken.contract.trim().toLowerCase(),
      amount: Number(newToken.amount),
      invested: Number(newToken.invested)
    });

    if (!res?.ok) {
      setAddTokenError(t('error_add_token_failed'));
      return;
    }

    setShowAddForm(false);
    setNewToken({ chain: 'ETH', contract: '', amount: '', invested: '' });
    await loadPortfolioState();
    setTimeout(() => {
      loadPortfolioState();
    }, 1500);
    onChanged?.();
  };

  const handleToggleTracking = async () => {
    if (!portfolio?.id || trackingLoading) return;

    setTrackingLoading(true);
    try {
      const nextValue = !trackingEnabled;
      await togglePortfolioAnomalyAlerts(portfolio.id, nextValue);
      setTrackingEnabled(nextValue);
      if (nextValue) {
        setShowTrackingInfo(true);
      }
      await loadPortfolioState();
      onChanged?.();
    } finally {
      setTrackingLoading(false);
    }
  };

  const handleToggleAssetTracking = async (asset) => {
    const key = asset.assetKey;
    if (!key) return;
    setAssetTrackingLoading((prev) => ({ ...prev, [key]: true }));
    try {
      await updatePortfolioAssetTracking({
        portfolio_id: portfolio.id,
        asset_key: key,
        asset_symbol: asset.symbol,
        enabled: !asset.assetTrackingEnabled
      });
      await loadPortfolioState();
    } finally {
      setAssetTrackingLoading((prev) => ({ ...prev, [key]: false }));
    }
  };

  const handleToggleAssetDailyReport = async (asset) => {
    const key = asset.assetKey;
    if (!key) return;
    setAssetTrackingLoading((prev) => ({ ...prev, [key]: true }));
    try {
      await updatePortfolioAssetTracking({
        portfolio_id: portfolio.id,
        asset_key: key,
        asset_symbol: asset.symbol,
        daily_report_enabled: !asset.dailyReportEnabled
      });
      await loadPortfolioState();
    } finally {
      setAssetTrackingLoading((prev) => ({ ...prev, [key]: false }));
    }
  };

  const handleChangeAssetProfile = async (asset, profile) => {
    const key = asset.assetKey;
    if (!key) return;
    setAssetTrackingLoading((prev) => ({ ...prev, [key]: true }));
    try {
      await updatePortfolioAssetTracking({
        portfolio_id: portfolio.id,
        asset_key: key,
        asset_symbol: asset.symbol,
        profile
      });
      await loadPortfolioState();
    } finally {
      setAssetTrackingLoading((prev) => ({ ...prev, [key]: false }));
    }
  };

  const handleToggleNetworkTracking = async (asset, network) => {
    const key = `${asset.assetKey}:${network.chainId}:${network.contract}`;
    setNetworkTrackingLoading((prev) => ({ ...prev, [key]: true }));
    try {
      await updatePortfolioAssetNetworkTracking({
        portfolio_id: portfolio.id,
        asset_key: asset.assetKey,
        chain_id: network.chainId,
        token: network.contract,
        enabled: !network.networkTrackingEnabled
      });
      await loadPortfolioState();
    } finally {
      setNetworkTrackingLoading((prev) => ({ ...prev, [key]: false }));
    }
  };

  const [showAddForm, setShowAddForm] = useState(false);
  const [showPortfolioDetails, setShowPortfolioDetails] = useState(true);

  return (
    <div className="h-screen w-full flex flex-col bg-white overflow-hidden">
      {/* HEADER */}
      <div className="px-4 pt-4 pb-2 border-b border-slate-200 flex-shrink-0">
        <button
          onClick={onBack}
          className="flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 transition-colors -ml-1"
          aria-label={t('back_to_portfolios')}
        >
          <ChevronLeftIcon className="w-4 h-4" />
          <span>{t('portfolios')}</span>
        </button>
      </div>

      {/* SCROLLABLE CONTENT */}
      <div className="flex-1 overflow-y-auto min-h-0 px-4 pt-4 pb-0">
        <div className="space-y-4 pb-12">

      {/* PORTFOLIO INFO - COLLAPSIBLE */}
      <Card className="space-y-0">
        <button
          onClick={() => setShowPortfolioDetails(!showPortfolioDetails)}
          className="w-full p-3 flex items-center justify-between hover:bg-slate-50 rounded-lg transition-colors"
        >
          <div className="text-left">
            <h1 className="text-lg font-bold text-slate-800">{portfolio?.name}</h1>
            <p className="text-xs text-slate-400">{t('portfolio_details')}</p>
          </div>
          <span className="text-sm text-slate-400">{showPortfolioDetails ? '▼' : '▶'}</span>
        </button>

        {showPortfolioDetails && (
          <div className="border-t border-slate-200 p-3 space-y-3">
            {/* TRACKING + ADD BUTTON */}
	            <div className="flex gap-2">
	              <Button
	                variant={trackingEnabled ? 'primary' : 'secondary'}
	                className="flex-1 px-3 py-2 text-xs"
	                onClick={handleToggleTracking}
	                disabled={trackingLoading}
	              >
	                <ActivityIcon className="w-3.5 h-3.5" />
	                {trackingEnabled ? t('tracking_on') : t('tracking_off')}
	              </Button>

	              <button
	                type="button"
	                className="px-2.5 py-2 rounded-xl border border-slate-200 text-slate-500 hover:text-slate-700 hover:border-slate-300 transition-colors"
	                onClick={() => setShowTrackingInfo(true)}
	                aria-label="Tracking rules information"
	                title="Tracking rules information"
	              >
	                <InfoIcon className="w-4 h-4" />
	              </button>
	              
	              <Button
	                variant="secondary"
	                className="flex-1 px-3 py-2 text-xs"
                onClick={() => setShowAddForm(!showAddForm)}
              >
                <PlusIcon className="w-3.5 h-3.5" />
                {showAddForm ? t('hide') : t('add_token')}
              </Button>
            </div>

            {/* STATS */}
            <div className="grid grid-cols-2 gap-2.5">
              <Card className="p-3">
                <p className="text-[10px] uppercase text-slate-400">{t('value')}</p>
                <p className="text-sm font-bold text-slate-800 tabular-nums">{formatUsd(totals.totalValue)}</p>
              </Card>
              <Card className="p-3">
                <p className="text-[10px] uppercase text-slate-400">{t('invested')}</p>
                <p className="text-sm font-bold text-slate-800 tabular-nums">{formatUsd(totals.totalInvested)}</p>
              </Card>
              <Card className="p-3 col-span-2">
                <p className="text-[10px] uppercase text-slate-400">{t('total_pnl')}</p>
                <p className={`text-sm font-bold tabular-nums ${totals.totalPnL >= 0 ? 'text-emerald-500' : 'text-rose-500'}`}>
                  {formatUsd(totals.totalPnL)}
                </p>
              </Card>
            </div>

            {/* ADD TOKEN FORM - INLINE */}
            {showAddForm && (
              <div className="border-t border-slate-200 pt-3 space-y-3">
                {/* NETWORK SELECT */}
                <div>
                  <p className="text-xs font-semibold text-slate-600 mb-2">{t('network')}</p>
                  <div className="grid grid-cols-2 gap-2">
                    {['ETH', 'BSC'].map(net => (
                      <Button
                        key={net}
                        variant={newToken.chain === net ? 'primary' : 'secondary'}
                        onClick={() => setNewToken({ ...newToken, chain: net, contract: '', amount: '', invested: '' })}
                      >
                        {net === 'ETH' ? 'Ethereum' : 'BSC'}
                      </Button>
                    ))}
                  </div>
                </div>

                {/* PRESET TOKENS */}
                {PRESET_TOKENS[newToken.chain] && (
                  <>
                    <p className="text-xs font-semibold text-slate-600">{t('popular_tokens')}</p>
                    <div className="grid grid-cols-2 gap-2">
                      {PRESET_TOKENS[newToken.chain].map(t => (
                        <button
                          key={t.contract}
                          onClick={() => {
                            setAddTokenError('');
                            setNewToken({ ...newToken, contract: t.contract, amount: '', invested: '' });
                            setAddTokenStep('amounts');
                          }}
                          className="p-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:border-slate-300 text-xs font-medium transition-all"
                        >
                          {t.label}
                        </button>
                      ))}
                    </div>
                  </>
                )}

                {/* PORTFOLIO TOKENS */}
                {tokens.length > 0 && (
                  <>
                    <p className="text-xs font-semibold text-slate-600">{t('portfolio_tokens_add_more')}</p>
                    <div className="grid grid-cols-2 gap-2">
                      {tokens.map(t => (
                        <button
                          key={t.id}
                          onClick={() => {
                            setAddTokenError('');
                            const contract = getPresetContractBySymbol(newToken.chain, t.symbol);
                            if (contract) {
                              setNewToken({ ...newToken, contract, amount: '', invested: '' });
                              setAddTokenStep('amounts');
                            } else {
                              setNewToken({ ...newToken, contract: '', amount: '', invested: '' });
                              setAddTokenStep('contract');
                            }
                          }}
                          className="p-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:border-slate-300 text-xs font-medium transition-all"
                        >
                          {t.symbol}
                        </button>
                      ))}
                    </div>
                  </>
                )}

                {/* {t('custom_contract')} */}
                <Button
                  variant="secondary"
                  fullWidth
                  onClick={() => {
                    setAddTokenError('');
                    setNewToken({ ...newToken, contract: '', amount: '', invested: '' });
                    setAddTokenStep('contract');
                  }}
                >
                  ✏️ {t('custom_contract')}
                </Button>
              </div>
            )}
          </div>
        )}
      </Card>

	        <div className="space-y-2.5">
	          {loading && <Card className="p-4 text-sm text-slate-400">{t('loading_tokens')}</Card>}

	          {!loading && assets.length === 0 && (
	            <Card className="p-4 text-sm text-slate-400">{t('no_tokens_portfolio')}</Card>
	          )}

	          {assets.map((asset) => {
	            const expanded = !!expandedAssets[asset.assetKey];
	            const assetBusy = !!assetTrackingLoading[asset.assetKey];

	            return (
	              <Card key={asset.assetKey} className="p-4 space-y-3">
	                <div className="flex items-start justify-between gap-3">
	                  <button
	                    type="button"
	                    className="text-left flex-1"
	                    onClick={() =>
	                      setExpandedAssets((prev) => ({ ...prev, [asset.assetKey]: !prev[asset.assetKey] }))
	                    }
	                  >
	                    <p className="text-sm font-bold text-cyan-600 hover:text-cyan-700">{asset.symbol}</p>
	                    <p className="text-[11px] text-slate-400">
	                      {asset.networkCount} {t('networks_label')} · {asset.trackedNetworks}/{asset.networkCount} {t('tracked_networks_label')}
	                    </p>
	                  </button>
	                  <div className="text-right">
	                    <p className="text-[10px] uppercase text-slate-400">{t('total_pnl')}</p>
	                    <p className={`text-sm font-semibold ${asset.totalPnL >= 0 ? 'text-emerald-500' : 'text-rose-500'}`}>
	                      {formatUsd(asset.totalPnL)}
	                    </p>
	                  </div>
	                </div>

	                <div className="grid grid-cols-2 gap-3 text-xs">
	                  <div className="space-y-2">
	                    <div>
	                      <p className="text-slate-400 text-[10px]">{t('value')}</p>
	                      <p className="font-medium text-slate-700">{formatUsd(asset.totalValue)}</p>
	                    </div>
	                    <div>
	                      <p className="text-slate-400 text-[10px]">{t('amount')}</p>
	                      <p className="font-medium text-slate-700">{formatCompact(asset.totalAmount)}</p>
	                    </div>
	                  </div>

	                  <div className="space-y-2">
	                    <div>
	                      <p className="text-slate-400 text-[10px]">{t('invested')}</p>
	                      <p className="font-medium text-slate-700">{formatUsd(asset.totalInvested)}</p>
	                    </div>
	                    <div>
	                      <p className="text-slate-400 text-[10px]">{t('tracking_profile_label')}</p>
	                      <p className="font-medium text-slate-700">{t(`tracking_profile_${asset.trackingProfile}`)}</p>
	                    </div>
	                  </div>
	                </div>

	                <div className="flex flex-wrap gap-2">
	                  <Button
	                    variant={asset.assetTrackingEnabled ? 'primary' : 'secondary'}
	                    className="px-3 py-2 text-xs"
	                    onClick={() => handleToggleAssetTracking(asset)}
	                    disabled={assetBusy}
	                  >
	                    <ActivityIcon className="w-3.5 h-3.5" />
	                    {asset.assetTrackingEnabled ? t('tracking_on') : t('tracking_off')}
	                  </Button>

	                  <Button
	                    variant={asset.dailyReportEnabled ? 'primary' : 'secondary'}
	                    className="px-3 py-2 text-xs"
	                    onClick={() => handleToggleAssetDailyReport(asset)}
	                    disabled={assetBusy}
	                  >
	                    {asset.dailyReportEnabled ? t('daily_report_on') : t('daily_report_off')}
	                  </Button>

	                  <select
	                    className="px-3 py-2 rounded-xl border border-slate-200 text-xs text-slate-700 bg-white"
	                    value={asset.trackingProfile}
	                    onChange={(event) => handleChangeAssetProfile(asset, event.target.value)}
	                    disabled={assetBusy}
	                  >
	                    <option value="quiet">{t('tracking_profile_quiet')}</option>
	                    <option value="balanced">{t('tracking_profile_balanced')}</option>
	                    <option value="aggressive">{t('tracking_profile_aggressive')}</option>
	                  </select>
	                </div>

	                {expanded && (
	                  <div className="space-y-2 border-t border-slate-200 pt-3">
	                    <p className="text-[10px] uppercase text-slate-400">{t('network_positions')}</p>
	                    {asset.networks.map((network) => {
	                      const networkBusy = !!networkTrackingLoading[`${asset.assetKey}:${network.chainId}:${network.contract}`];
	                      return (
	                        <div
	                          key={`${network.chainId}:${network.contract}`}
	                          className="rounded-xl border border-slate-200 px-3 py-2 space-y-2"
	                        >
	                          <div className="flex items-center justify-between gap-2">
	                            <button
	                              type="button"
	                              className="text-left"
	                              onClick={() =>
	                                setTokenModal({
	                                  id: network.id,
	                                  symbol: asset.symbol,
	                                  amount: network.amount,
	                                  invested: network.invested,
	                                  realized: network.realized,
	                                  buyPrice: network.buyPriceUSD,
	                                  currentPrice: network.currentPriceUSD,
	                                  chainId: network.chainId,
	                                  chain: network.chain,
	                                  contract: network.contract
	                                })
	                              }
	                            >
	                              <p className="text-sm font-semibold text-slate-800">{network.chain}</p>
	                              <p className="text-[11px] text-slate-400">{network.contract}</p>
	                            </button>
	                            <button
	                              type="button"
	                              className={`px-2.5 py-1 rounded-full text-[11px] font-medium ${
	                                network.networkTrackingEnabled
	                                  ? 'bg-emerald-50 text-emerald-600'
	                                  : 'bg-slate-100 text-slate-500'
	                              }`}
	                              onClick={() => handleToggleNetworkTracking(asset, network)}
	                              disabled={networkBusy}
	                            >
	                              {network.networkTrackingEnabled ? t('network_tracking_on') : t('network_tracking_off')}
	                            </button>
	                          </div>

	                          <div className="grid grid-cols-2 gap-2 text-xs">
	                            <div>
	                              <p className="text-slate-400 text-[10px]">{t('value')}</p>
	                              <p className="font-medium text-slate-700">{formatUsd(network.value)}</p>
	                            </div>
	                            <div>
	                              <p className="text-slate-400 text-[10px]">{t('amount')}</p>
	                              <p className="font-medium text-slate-700">{formatCompact(network.amount)}</p>
	                            </div>
	                            <div>
	                              <p className="text-slate-400 text-[10px]">{t('invested')}</p>
	                              <p className="font-medium text-slate-700">{formatUsd(network.invested)}</p>
	                            </div>
	                            <div>
	                              <p className="text-slate-400 text-[10px]">{t('total_pnl')}</p>
	                              <p className={`font-medium ${network.totalPnL >= 0 ? 'text-emerald-500' : 'text-rose-500'}`}>
	                                {formatUsd(network.totalPnL)}
	                              </p>
	                            </div>
	                          </div>

	                          <Button
	                            variant="secondary"
	                            fullWidth
	                            onClick={() =>
	                              setRealizeToken({
	                                id: network.id,
	                                symbol: asset.symbol,
	                                amount: network.amount,
	                                invested: network.invested,
	                                realized: network.realized,
	                                buyPrice: network.buyPriceUSD,
	                                currentPrice: network.currentPriceUSD,
	                                chainId: network.chainId,
	                                chain: network.chain,
	                                contract: network.contract,
	                                assetKey: asset.assetKey
	                              })
	                            }
	                          >
	                            {t('realize_position')}
	                          </Button>
	                        </div>
	                      );
	                    })}
	                  </div>
	                )}
	              </Card>
	            );
	          })}
	        </div>

	        {!loading && notifications.length > 0 && (
	          <Card className="p-4 space-y-2">
	            <p className="text-[10px] uppercase text-slate-400">{t('recent_tracking_alerts')}</p>
	            {notifications.slice(0, 8).map((item, index) => (
	              <div key={`${item.Time || item.time || index}`} className="rounded-xl bg-slate-50 px-3 py-2">
	                <p className="text-xs font-medium text-slate-700">{item.Text || item.text || '—'}</p>
	                <p className="text-[11px] text-slate-400">{item.Type || item.type || ''}</p>
	              </div>
	            ))}
	          </Card>
	        )}
	        </div>
	      </div>

      {/* MODALS FOR {t('custom_contract')} AND AMOUNTS */}
      <Modal isOpen={addTokenStep === 'contract'} onClose={() => setAddTokenStep(null)} title={t('enter_contract')}>
        <div className="space-y-3">
          <Input
            placeholder="0x..."
            value={newToken.contract}
            onChange={(e) => setNewToken({ ...newToken, contract: e.target.value })}
          />
          {addTokenError && <div className="text-xs text-rose-500 font-medium">{addTokenError}</div>}
          <div className="flex gap-2">
            <Button
              variant="secondary"
              fullWidth
              onClick={() => setAddTokenStep(null)}
            >
              {t('back')}
            </Button>
            <Button
              fullWidth
              disabled={!newToken.contract.trim()}
              onClick={() => setAddTokenStep('amounts')}
            >
              {t('next')}
            </Button>
          </div>
        </div>
      </Modal>

      {/* AMOUNTS INPUT */}
      <Modal isOpen={addTokenStep === 'amounts'} onClose={() => setAddTokenStep(null)} title={t('token_details')}>
        <div className="space-y-3">
          <Input
            label={t('amount')}
            type="number"
            placeholder="0"
            value={newToken.amount}
            onChange={(event) => setNewToken((prev) => ({ ...prev, amount: event.target.value }))}
          />
          <Input
            label={`${t('invested')} (USD)`}
            type="number"
            placeholder="0"
            value={newToken.invested}
            onChange={(event) => setNewToken((prev) => ({ ...prev, invested: event.target.value }))}
          />
          {addTokenError && <div className="text-xs text-rose-500 font-medium">{addTokenError}</div>}
          <div className="flex gap-2 pt-2">
            <Button
              variant="secondary"
              fullWidth
              onClick={() => setAddTokenStep(null)}
            >
              {t('back')}
            </Button>
            <Button
              fullWidth
              disabled={!newToken.amount || !newToken.invested}
              onClick={() => {
                handleAddToken();
                setAddTokenStep(null);
                setShowAddForm(false);
              }}
            >
              {t('save_token')}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={tokenModal !== null} onClose={() => setTokenModal(null)} title={tokenModal?.symbol || t('token_label')}>
        {tokenModal && <TokenDashboard token={tokenModal} />}
      </Modal>

	      <Modal isOpen={realizeToken !== null} onClose={() => setRealizeToken(null)} title={t('realize_position')}>
	        {realizeToken && (
          <RealizePositionModal
            token={realizeToken}
            tokens={tokens}
            chain={newToken.chain}
            onCancel={() => setRealizeToken(null)}
            onConfirm={async (data) => {
              const base = {
                portfolioId: portfolio.id,
                from: {
                  tokenId: realizeToken.id,
                  amountDelta: -data.amountToRealize,
                  realizedDelta: data.valueToRealize
                }
              };

              if (data.targetType === 'cash') {
                await portfolioOperation({
                  ...base,
                  type: 'REALIZE_CASH'
                });
              } else if (data.targetType === 'portfolio') {
                const targetToken = tokens.find((t) => t.id === data.targetTokenId);
                if (!targetToken || !targetToken.currentPrice) {
                  return;
                }
                await portfolioOperation({
                  ...base,
                  type: 'REALIZE_SWAP',
                  to: {
                    tokenId: data.targetTokenId,
                    amountDelta: data.valueToRealize / targetToken.currentPrice,
                    investedDelta: data.valueToRealize
                  }
                });
              } else {
                await portfolioOperation({
                  ...base,
                  type: 'REALIZE_NEW_TOKEN',
                  newToken: {
                    contract: data.targetContract,
                    amount: 0,
                    invested: data.valueToRealize
                  }
                });
              }
	              await loadPortfolioState();
              onChanged?.();
              setRealizeToken(null);
            }}
          />
	        )}
	      </Modal>

	      <Modal
	        isOpen={showTrackingInfo}
	        onClose={() => setShowTrackingInfo(false)}
	        title={t('tracking_info_title')}
	      >
	        <div className="space-y-4 text-sm text-slate-700">
	          <p>{t('tracking_info_intro')}</p>

	          <div className="space-y-1">
	            <p className="font-semibold text-slate-800">{t('tracking_info_when_title')}</p>
	            <p>{t('tracking_info_when_1')}</p>
	            <p>{t('tracking_info_when_2')}</p>
	            <p>{t('tracking_info_when_3')}</p>
	          </div>

	          <div className="space-y-1">
	            <p className="font-semibold text-slate-800">{t('tracking_info_thresholds_title')}</p>
	            <p>{t('tracking_info_thresholds_1')}</p>
	            <p>{t('tracking_info_thresholds_2')}</p>
	            <p>{t('tracking_info_thresholds_3')}</p>
	            <p>{t('tracking_info_thresholds_4')}</p>
	          </div>

	          <div className="space-y-1">
	            <p className="font-semibold text-slate-800">{t('tracking_info_spam_title')}</p>
	            <p>{t('tracking_info_spam_1')}</p>
	            <p>{t('tracking_info_spam_2')}</p>
	            <p>{t('tracking_info_spam_3')}</p>
	          </div>

	          <div className="space-y-1">
	            <p className="font-semibold text-slate-800">{t('tracking_info_payload_title')}</p>
	            <p>{t('tracking_info_payload_text')}</p>
	          </div>
	        </div>
	      </Modal>
	    </div>
	  );
	}

