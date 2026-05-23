import React, { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { BottomNav } from './components/ui/BottomNav';
import { Dashboard } from './pages/Dashboard';
import { Portfolios } from './pages/Portfolios';
import { PortfolioDetails } from './pages/PortfolioDetails';
import { AgentDashboard } from './comp/AgentDashboard';
import { AlertRules } from './pages/AlertRules';
import { getAlertRules, getPortfolios } from './api';
import { I18nProvider } from './i18n';

const pageTransition = {
  initial: { opacity: 0, y: 10 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -6 },
  transition: { duration: 0.3, ease: 'easeOut' }
};

export default function App() {
  const DEV_TELEGRAM_ID = 574595536;
  const DEV_USER_ID = 4;
  const IS_PROD = process.env.NODE_ENV === 'production';
  const tgUserUnsafe = window.Telegram?.WebApp?.initDataUnsafe?.user;
  const [activeScreen, setActiveScreen] = useState('dashboard');
  const [subScreen, setSubScreen] = useState(null);
  const [selectedPortfolio, setSelectedPortfolio] = useState(null);
  const [userId, setUserId] = useState(IS_PROD ? null : DEV_USER_ID);
  const [telegramId, setTelegramId] = useState(tgUserUnsafe?.id || (IS_PROD ? null : DEV_TELEGRAM_ID));
  const [displayUser, setDisplayUser] = useState(tgUserUnsafe || (!IS_PROD ? { username: 'local_dev', first_name: 'Local' } : null));
  const [portfolios, setPortfolios] = useState([]);
  const [rules, setRules] = useState([]);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    const tg = window.Telegram?.WebApp;
    if (!tg) {
      if (!IS_PROD) {
        setUserId((value) => value || DEV_USER_ID);
        setTelegramId((value) => value || DEV_TELEGRAM_ID);
      }
      return;
    }

    tg.ready();
    tg.expand();

    const initData = tg.initData;
    if (!initData) {
      if (!IS_PROD) {
        setUserId((value) => value || DEV_USER_ID);
        setTelegramId((value) => value || DEV_TELEGRAM_ID);
      }
      return;
    }

    fetch('/api/init-user', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ initData })
    })
      .then(async (response) => {
        if (!response.ok) throw new Error(await response.text());
        return response.json();
      })
      .then((data) => {
        if (data?.user) {
          setUserId(data.user.user_id);
          setTelegramId(data.user.telegram_id);
          setDisplayUser(data.user);
          setRefreshKey((value) => value + 1);
        }
      })
      .catch(() => {
        if (tgUserUnsafe?.id) {
          setTelegramId(tgUserUnsafe.id);
          setDisplayUser(tgUserUnsafe);
          return;
        }

        if (!IS_PROD) {
          setUserId((value) => value || DEV_USER_ID);
          setTelegramId((value) => value || DEV_TELEGRAM_ID);
        }
      });
  }, []);

  useEffect(() => {
    if (!userId) return;

    getPortfolios(userId)
      .then((data) => {
        if (!Array.isArray(data)) {
          setPortfolios([]);
          return;
        }

        const mapped = data.map((portfolio) => ({
          id: portfolio.ID ?? portfolio.id,
          name: portfolio.Name ?? portfolio.name,
          totalInvested: Number(portfolio.TotalInvested ?? portfolio.total_invested ?? 0),
          totalPnL: Number(portfolio.TotalPnL ?? portfolio.total_pnl ?? 0),
          totalValue:
            Number(portfolio.TotalInvested ?? portfolio.total_invested ?? 0) +
            Number(portfolio.TotalPnL ?? portfolio.total_pnl ?? 0),
          pnlPercent:
            Number(portfolio.TotalInvested ?? portfolio.total_invested ?? 0) > 0
              ?
                (Number(portfolio.TotalPnL ?? portfolio.total_pnl ?? 0) /
                  Number(portfolio.TotalInvested ?? portfolio.total_invested ?? 1)) *
                100
              : 0,
          anomalyAlertsEnabled: !!(portfolio.OnchainAlertsEnabled ?? portfolio.onchain_alerts_enabled)
        }));

        setPortfolios(mapped);
      })
      .catch(() => setPortfolios([]));

    getAlertRules(userId)
      .then((data) => {
        if (!Array.isArray(data)) {
          setRules([]);
          return;
        }

        const mapped = data.map((rule) => ({
          rule_id: rule.rule_id ?? rule.id,
          id: rule.rule_id ?? rule.id,
          name: rule.name || `Rule #${rule.rule_id ?? rule.id}`,
          address: rule.filters?.address || 'Address not set',
          newCount: Number(rule.new_count ?? rule.newcount ?? 0)
        }));

        setRules(mapped);
      })
      .catch(() => setRules([]));
  }, [userId, refreshKey]);

  const requestRefresh = () => {
    setRefreshKey((value) => value + 1);
  };

  const handleNavigate = (screen) => {
    setSubScreen(null);
    setSelectedPortfolio(null);
    setActiveScreen(screen);
  };

  const handleSelectPortfolio = (id) => {
    setSelectedPortfolio(id);
    setSubScreen('portfolio-details');
  };

  const handleBackToPortfolios = () => {
    setSubScreen(null);
    setSelectedPortfolio(null);
  };

  const renderScreen = () => {
    if (subScreen === 'portfolio-details' && selectedPortfolio) {
      return (
        <motion.div key="portfolio-details" {...pageTransition}>
          <PortfolioDetails portfolio={selectedPortfolio} onBack={handleBackToPortfolios} onChanged={requestRefresh} />
        </motion.div>
      );
    }

    switch (activeScreen) {
      case 'dashboard':
        return (
          <motion.div key="dashboard" {...pageTransition}>
            <Dashboard displayUser={displayUser} portfolioCount={portfolios.length} alertsCount={rules.length} />
          </motion.div>
        );
      case 'portfolios':
        return (
          <motion.div key="portfolios" {...pageTransition}>
            <Portfolios
              userId={userId}
              portfolios={portfolios}
              onSelectPortfolio={handleSelectPortfolio}
              onChanged={requestRefresh}
            />
          </motion.div>
        );
      case 'analytics':
        return (
          <motion.div key="analytics" {...pageTransition}>
            <AgentDashboard />
          </motion.div>
        );
      case 'alerts':
        return (
          <motion.div key="alerts" {...pageTransition}>
            <AlertRules
              rules={rules}
              userId={userId}
              telegramId={telegramId}
              onChanged={requestRefresh}
            />
          </motion.div>
        );
      default:
        return null;
    }
  };

  return (
    <I18nProvider>
      <div className="min-h-screen w-full">
        <div className="max-w-md mx-auto px-4 py-3">
          <AnimatePresence mode="wait">{renderScreen()}</AnimatePresence>
        </div>
        <BottomNav activeScreen={activeScreen} onNavigate={handleNavigate} />
      </div>
    </I18nProvider>
  );
}


