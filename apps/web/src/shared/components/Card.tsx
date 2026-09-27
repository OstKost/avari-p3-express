import React from 'react';
import { cn } from '@/shared/utils/cn';

export function Card({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('avari-surface rounded-2xl p-5', className)} {...props}>{children}</div>;
}
