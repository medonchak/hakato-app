import React from 'react';

export function Input({ label, error, className = '', id, ...props }) {
  const inputId = id || label?.toLowerCase().replace(/\s+/g, '-');
  return (
    <div className="space-y-1.5">
      {label && (
        <label htmlFor={inputId} className="block text-sm font-medium text-slate-600">
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={`
          w-full rounded-xl bg-slate-50 border border-slate-200
          px-4 py-3 text-sm text-slate-800 placeholder:text-slate-400
          focus:outline-none focus:ring-2 focus:ring-cyan-300/50 focus:border-cyan-300
          transition-all duration-150
          ${error ? 'border-rose-300 focus:ring-rose-300/50' : ''}
          ${className}
        `}
        {...props}
      />

      {error && <p className="text-xs text-rose-500 mt-1">{error}</p>}
    </div>
  );
}
