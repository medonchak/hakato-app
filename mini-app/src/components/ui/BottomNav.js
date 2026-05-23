import React from 'react';
import { motion } from 'framer-motion';
import { BarChart3Icon, WalletIcon, BotIcon, BellIcon } from 'lucide-react';
import { useI18n } from '../../i18n';

export function BottomNav({ activeScreen, onNavigate }) {
  const { t } = useI18n();
  const navItems = [
    { id: 'dashboard', label: t('nav_dashboard'), icon: <BarChart3Icon className="w-5 h-5" /> },
    { id: 'portfolios', label: t('nav_portfolios'), icon: <WalletIcon className="w-5 h-5" /> },
    { id: 'analytics', label: 'Agent', icon: <BotIcon className="w-5 h-5" /> },
    { id: 'alerts', label: t('nav_alerts'), icon: <BellIcon className="w-5 h-5" /> }
  ];

  return (
    <nav
      className="fixed bottom-0 left-0 right-0 z-30 bg-white/80 backdrop-blur-xl border-t border-slate-100"
      aria-label={t('main_navigation')}
    >
      <div className="max-w-md mx-auto flex items-center justify-around px-2 py-2">
        {navItems.map((item) => {
          const isActive = activeScreen === item.id;
          return (
            <button
              key={item.id}
              onClick={() => onNavigate(item.id)}
              className={`
                relative flex flex-col items-center gap-0.5 px-4 py-1.5 rounded-xl
                transition-colors duration-150
                ${isActive ? 'text-cyan-500' : 'text-slate-400'}
              `}
              aria-label={item.label}
              aria-current={isActive ? 'page' : undefined}
            >
              {isActive && (
                <motion.div
                  layoutId="nav-indicator"
                  className="absolute inset-0 bg-cyan-50 rounded-xl"
                  transition={{
                    type: 'spring',
                    damping: 25,
                    stiffness: 300
                  }}
                />
              )}
              <span className="relative z-10">{item.icon}</span>
              <span className="relative z-10 text-[10px] font-medium">{item.label}</span>
            </button>
          );
        })}
      </div>
      <div className="h-[env(safe-area-inset-bottom)]" />
    </nav>
  );
}
