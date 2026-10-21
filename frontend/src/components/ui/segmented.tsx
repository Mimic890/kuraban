import { cn } from '@/lib/utils';

export interface SegmentedOption<T extends string | number> {
  value: T;
  label: React.ReactNode;
  title?: string;
}

export function Segmented<T extends string | number>({
  value,
  options,
  onChange,
  disabled,
  className,
  size = 'md',
}: {
  value: T;
  options: SegmentedOption<T>[];
  onChange: (value: T) => void;
  disabled?: boolean;
  className?: string;
  size?: 'sm' | 'md';
}) {
  return (
    <div role="radiogroup" className={cn('inline-flex flex-wrap gap-0.5 rounded-md border border-[var(--border)] bg-[var(--background)] p-0.5', disabled && 'opacity-50', className)}>
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={String(o.value)}
            type="button"
            role="radio"
            aria-checked={active}
            title={o.title}
            disabled={disabled}
            onClick={() => onChange(o.value)}
            className={cn(
              'rounded-[5px] font-medium transition-colors tabular',
              size === 'sm' ? 'h-6 px-2 text-[0.6875rem]' : 'h-7 px-2.5 text-[0.75rem]',
              active
                ? 'bg-[var(--accent)] text-[var(--foreground)] shadow-[inset_0_0_0_1px_var(--border)]'
                : 'text-[var(--muted-foreground)] hover:text-[var(--foreground)]',
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
