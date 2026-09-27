import React from 'react';
import { cn } from '@/shared/utils/cn';
import { Loader2 } from 'lucide-react';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  isLoading?: boolean;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'primary', size = 'md', isLoading = false, leftIcon, rightIcon, children, disabled, ...props }, ref) => {
    const variants = {
      primary: 'avari-shine bg-[var(--av-gold)] text-[var(--av-on-accent)] hover:bg-[var(--av-gold-hover)]',
      secondary: 'bg-[var(--av-surface-raised)] text-[var(--av-text)] border border-[var(--av-border-control)] hover:bg-[var(--av-surface-hover)]',
      outline: 'bg-transparent text-[var(--av-text)] border border-[var(--av-border-control)] hover:bg-[var(--av-surface-hover)]',
      ghost: 'text-[var(--av-text-secondary)] hover:text-[var(--av-text)] hover:bg-[var(--av-surface-hover)]',
      danger: 'bg-[var(--av-danger)] text-[var(--av-bg)] hover:opacity-90',
    };
    const sizes = {
      sm: 'min-h-11 px-3 text-xs rounded-lg gap-1.5',
      md: 'min-h-12 px-4 text-sm rounded-xl gap-2',
      lg: 'min-h-12 px-5 text-sm rounded-xl gap-2.5',
    };
    return (
      <button
        ref={ref}
        disabled={disabled || isLoading}
        aria-busy={isLoading || undefined}
        className={cn('avari-button inline-flex items-center justify-center font-medium avari-interactive disabled:opacity-50 disabled:cursor-not-allowed select-none', variants[variant], sizes[size], className)}
        {...props}
      >
        {isLoading ? <Loader2 className="w-4 h-4 animate-spin" aria-hidden="true" /> : leftIcon}
        {children}
        {!isLoading && rightIcon}
      </button>
    );
  }
);
Button.displayName = 'Button';
