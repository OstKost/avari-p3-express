import React from 'react';
import { cn } from '@/shared/utils/cn';

export function Skeleton({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'animate-pulse rounded-xl bg-[var(--av-surface-raised)]',
        className
      )}
      {...props}
    />
  );
}
