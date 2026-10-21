import * as React from 'react';
import { IconTile } from './icon-tile';
import { cn } from '@/lib/utils';

interface EmptyStateProps extends React.HTMLAttributes<HTMLDivElement> {
  icon: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
  tone?: 'primary' | 'neutral' | 'destructive';
}

export function EmptyState({ icon, title, description, action, tone = 'primary', className, ...props }: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-3 rounded-xl border border-[var(--border)] bg-[var(--card)] px-6 py-12 text-center',
        className,
      )}
      {...props}
    >
      <IconTile icon={icon} tone={tone} size="lg" />
      <div className="space-y-1">
        <h3 className="text-[1.0625rem] font-semibold tracking-tight">{title}</h3>
        {description && (
          <p className="max-w-sm text-[0.8438rem] text-[var(--muted-foreground)]">{description}</p>
        )}
      </div>
      {action && <div className="pt-1">{action}</div>}
    </div>
  );
}
