import React from 'react';
import { cn } from '@/shared/utils/cn';

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: 'success' | 'warning' | 'danger' | 'neutral' | 'indigo';
}

export function Badge({ variant = 'neutral', className, children, ...props }: BadgeProps) {
  const variants = {
    success: 'text-[var(--av-success)] border-[var(--av-success)]/50',
    warning: 'text-[var(--av-warning)] border-[var(--av-warning)]/50',
    danger: 'text-[var(--av-danger)] border-[var(--av-danger)]/50',
    neutral: 'text-[var(--av-text-secondary)] border-[var(--av-border-control)]',
    indigo: 'text-[var(--av-cyan)] border-[var(--av-cyan)]/50',
  };
  return <span className={cn('inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-mono font-medium border bg-[var(--av-bg)]', variants[variant], className)} {...props}>{children}</span>;
}
