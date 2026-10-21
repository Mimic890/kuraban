import * as React from 'react';
import { cn } from '@/lib/utils';
import { cva, type VariantProps } from 'class-variance-authority';

const iconTileVariants = cva(
  'inline-flex items-center justify-center shrink-0 border',
  {
    variants: {
      tone: {
        primary: 'bg-[var(--accent-primary-soft)] border-[var(--accent-primary-border)] text-[var(--primary)]',
        destructive: 'bg-[var(--danger-soft)] border-[var(--danger-border)] text-[var(--destructive)]',
        neutral: 'bg-muted border-border text-muted-foreground',
      },
      size: {
        sm: 'h-8 w-8 rounded-md [&>svg]:h-4 [&>svg]:w-4',
        md: 'h-10 w-10 rounded-lg [&>svg]:h-5 [&>svg]:w-5',
        lg: 'h-14 w-14 rounded-xl [&>svg]:h-7 [&>svg]:w-7',
      },
    },
    defaultVariants: { tone: 'primary', size: 'md' },
  }
);

export interface IconTileProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof iconTileVariants> {
  icon: React.ReactNode;
}

export function IconTile({ icon, tone, size, className, ...props }: IconTileProps) {
  return (
    <div className={cn(iconTileVariants({ tone, size }), className)} {...props}>
      {icon}
    </div>
  );
}
