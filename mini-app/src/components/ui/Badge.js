import React from 'react';

const variantStyles = {
  success: 'bg-emerald-50 text-emerald-600 border-emerald-100',
  danger: 'bg-rose-50 text-rose-600 border-rose-100',
  info: 'bg-cyan-50 text-cyan-600 border-cyan-100',
  neutral: 'bg-slate-50 text-slate-600 border-slate-100'
};

export function Badge({ children, variant = 'neutral', className = '' }) {
  return (
    <span
      className={`
        inline-flex items-center gap-1 rounded-lg
        px-2.5 py-1 text-xs font-medium border
        ${variantStyles[variant]}
        ${className}
      `}
    >
      {children}
    </span>
  );
}
