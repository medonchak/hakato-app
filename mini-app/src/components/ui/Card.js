import React from 'react';
import { motion } from 'framer-motion';

const variantStyles = {
  default: 'bg-white border border-slate-100/60 shadow-sm',
  dashed:
    'bg-white/50 border-2 border-dashed border-slate-200 hover:border-cyan-300 hover:bg-white/80',
  highlighted: 'bg-white border border-cyan-200/60 shadow-md shadow-cyan-50'
};

export function Card({
  children,
  variant = 'default',
  className = '',
  onClick,
  animate = true
}) {
  const BaseComponent = animate ? motion.div : 'div';
  const interactiveProps = onClick
    ? {
        role: 'button',
        tabIndex: 0,
        onClick,
        onKeyDown: (e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onClick();
          }
        },
        ...(animate
          ? {
              whileTap: {
                scale: 0.98
              }
            }
          : {}),
        style: {
          cursor: 'pointer'
        }
      }
    : {};

  return (
    <BaseComponent className={`rounded-2xl p-4 ${variantStyles[variant]} ${className}`} {...interactiveProps}>
      {children}
    </BaseComponent>
  );
}
