import React, { useId } from 'react';
import { cn } from '@/shared/utils/cn';

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, helperText, leftIcon, rightIcon, id, ...props }, ref) => {
    const generatedId = useId();
    const inputId = id || generatedId;
    const descriptionId = error || helperText ? `${inputId}-description` : undefined;
    return (
      <div className="w-full space-y-1.5">
        {label && <label htmlFor={inputId} className="block text-sm font-medium avari-secondary">{label}</label>}
        <div className="relative flex items-center">
          {leftIcon && <span className="absolute left-3 avari-muted pointer-events-none flex items-center" aria-hidden="true">{leftIcon}</span>}
          <input
            id={inputId}
            ref={ref}
            aria-invalid={!!error || undefined}
            aria-describedby={descriptionId}
            className={cn('w-full min-h-12 bg-[var(--av-bg)] border border-[var(--av-border-control)] rounded-xl px-3.5 py-2.5 text-base text-[var(--av-text)] placeholder:text-[var(--av-text-muted)] focus:outline-none focus:border-[var(--av-cyan)] focus:ring-2 focus:ring-[var(--av-cyan)]/25 transition-colors', leftIcon && 'pl-10', rightIcon && 'pr-10', error && 'border-[var(--av-danger)]', className)}
            {...props}
          />
          {rightIcon && <span className="absolute right-3 avari-muted flex items-center">{rightIcon}</span>}
        </div>
        {error ? <p id={descriptionId} className="text-sm text-[var(--av-danger)]">{error}</p> : helperText ? <p id={descriptionId} className="text-xs avari-muted">{helperText}</p> : null}
      </div>
    );
  }
);
Input.displayName = 'Input';
