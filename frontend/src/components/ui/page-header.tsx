import * as React from 'react';
import { cn } from '@/lib/utils';

interface PageHeaderProps extends React.HTMLAttributes<HTMLDivElement> {
  title: string;
  subtitle?: React.ReactNode;
  actions?: React.ReactNode;
}

export function PageHeader({ title, subtitle, actions, className, ...props }: PageHeaderProps) {
  return (
    <div
      className={cn(
        'flex flex-col gap-3 border-b border-[var(--border)] px-4 py-4 md:px-6 sm:flex-row sm:items-center sm:justify-between sm:gap-6',
        className,
      )}
      {...props}
    >
      <div className="min-w-0">
        <h1 className="text-[1.25rem] font-semibold tracking-[-0.02em] leading-tight truncate">{title}</h1>
        {subtitle && (
          <div className="mt-0.5 text-[0.8125rem] text-[var(--muted-foreground)]">{subtitle}</div>
        )}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}
