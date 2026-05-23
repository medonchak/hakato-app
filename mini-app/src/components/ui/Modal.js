import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { XIcon } from 'lucide-react';

export function Modal({ isOpen, onClose, children, title }) {
  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            className="fixed inset-0 bg-black/30 backdrop-blur-sm z-40"
            onClick={onClose}
            aria-hidden="true"
          />

          <motion.div
            initial={{ scale: 0.95, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.95, opacity: 0 }}
            transition={{ type: 'spring', damping: 28, stiffness: 300 }}
            className="fixed inset-0 flex items-center justify-center z-50 p-3 sm:p-4 pointer-events-none"
            role="dialog"
            aria-modal="true"
            aria-label={title}
          >
            <div className="bg-white rounded-2xl sm:rounded-3xl shadow-2xl w-full max-w-full sm:max-w-md max-h-[85vh] overflow-hidden flex flex-col pointer-events-auto">
              <div className="flex justify-center pt-3 pb-1 flex-shrink-0">
                <div className="w-10 h-1 rounded-full bg-slate-200" />
              </div>

              <div className="px-4 sm:px-5 pb-2 flex items-center justify-between flex-shrink-0 gap-2">
                {title && <h3 className="text-base sm:text-lg font-bold text-slate-800 truncate">{title}</h3>}
                <button
                  onClick={onClose}
                  className="ml-auto p-2 rounded-full hover:bg-slate-100 transition-colors flex-shrink-0"
                  aria-label="Close modal"
                >
                  <XIcon className="w-5 h-5 text-slate-400" />
                </button>
              </div>

              <div className="px-4 sm:px-5 pb-6 sm:pb-8 overflow-y-auto flex-1">{children}</div>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
