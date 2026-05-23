import React from 'react';

export function Skeleton({ className = '' }) {
  return <div className={`rounded-xl bg-slate-200/60 animate-skeleton ${className}`} aria-hidden="true" />;
}

export function SkeletonCard() {
  return (
    <div className="rounded-2xl bg-white border border-slate-100/60 shadow-sm p-4 space-y-3">
      <Skeleton className="h-4 w-24" />
      <Skeleton className="h-7 w-36" />
      <div className="flex gap-3">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-4 w-16" />
      </div>
    </div>
  );
}

export function SkeletonTokenCard() {
  return (
    <div className="rounded-2xl bg-white border border-slate-100/60 shadow-sm p-4 flex items-center gap-3">
      <Skeleton className="h-10 w-10 rounded-full flex-shrink-0" />
      <div className="flex-1 space-y-2">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-3 w-14" />
      </div>
      <div className="space-y-2 text-right">
        <Skeleton className="h-4 w-16 ml-auto" />
        <Skeleton className="h-3 w-12 ml-auto" />
      </div>
    </div>
  );
}
