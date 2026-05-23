import React from 'react';
import { motion } from 'framer-motion';

const variantStyles = {
  primary:
    'bg-gradient-to-r from-cyan-400 to-teal-400 text-white shadow-md shadow-cyan-100/50',
  secondary: 'bg-slate-100 text-slate-700 hover:bg-slate-200',
  danger: 'bg-rose-50 text-rose-600 hover:bg-rose-100'
};

export function Button({
  children,
  variant = 'primary',
  className = '',
  onClick,
  disabled = false,
  fullWidth = false,
  type = 'button'
}) {
  return (
    <motion.button
      type={type}
      onClick={onClick}
      disabled={disabled}
      whileTap={{ scale: 0.97 }}
      className={`
        inline-flex items-center justify-center gap-2
        rounded-xl px-5 py-3 text-sm font-semibold
        transition-colors duration-150
        disabled:opacity-50 disabled:pointer-events-none
        ${variantStyles[variant]}
        ${fullWidth ? 'w-full' : ''}
        ${className}
      `}
    >
      {children}
    </motion.button>
  );
}
