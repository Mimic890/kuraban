import * as React from 'react';
import { Check, ChevronDown, Clock, RefreshCw, ZoomOut } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { RELATIVE_RANGES, REFRESH_OPTIONS, type TimeRange } from '@/hooks/useTelemetry';
import { formatTime } from '@/lib/units';
import { dateTimeInputToIso, dateTimeInputValue } from '@/lib/utils';

const triggerClass =
  'flex h-8 items-center gap-1.5 rounded-md border border-[var(--border)] bg-[var(--card)] px-2.5 text-[0.8125rem] text-[var(--foreground)] transition-colors hover:border-[var(--input)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]';

export function rangeLabel(range: TimeRange, t: (k: string, v?: Record<string, string | number>) => string) {
  if (range.kind === 'relative') return t(`time.last.${range.key}`);
  const multiDay = range.to - range.from > 86400;
  return `${formatTime(range.from, false, true)} → ${formatTime(range.to, false, multiDay)}`;
}

export function TimeRangePicker({ range, onChange }: { range: TimeRange; onChange: (r: TimeRange) => void }) {
  const { t } = useTranslation();
  const [open, setOpen] = React.useState(false);
  const now = new Date();
  const [from, setFrom] = React.useState(() => dateTimeInputValue(new Date(now.getTime() - 3600_000)));
  const [to, setTo] = React.useState(() => dateTimeInputValue(now));

  const applyAbsolute = () => {
    const f = Math.floor(new Date(dateTimeInputToIso(from)).getTime() / 1000);
    const tt = Math.floor(new Date(dateTimeInputToIso(to)).getTime() / 1000);
    if (Number.isFinite(f) && Number.isFinite(tt) && tt > f) {
      onChange({ kind: 'absolute', from: f, to: tt });
      setOpen(false);
    }
  };

  const zoomOut = () => {
    const { from: f, to: tt } = range.kind === 'absolute' ? range : { from: Date.now() / 1000 - (RELATIVE_RANGES.find((r) => r.key === range.key)?.seconds ?? 3600), to: Date.now() / 1000 };
    const span = tt - f;
    const nowS = Math.floor(Date.now() / 1000);
    const next = { from: Math.floor(f - span / 2), to: Math.min(nowS, Math.ceil(tt + span / 2)) };
    const rel = RELATIVE_RANGES.find((r) => r.seconds >= next.to - next.from && nowS - next.to < 60);
    onChange(rel ? { kind: 'relative', key: rel.key } : { kind: 'absolute', ...next });
  };

  return (
    <div className="flex items-center">
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger className={cn(triggerClass, 'rounded-r-none')}>
          <Clock className="h-3.5 w-3.5 text-[var(--muted-foreground)]" />
          <span className="max-w-[260px] truncate tabular">{rangeLabel(range, t)}</span>
          <ChevronDown className="h-3.5 w-3.5 opacity-60" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-[min(92vw,460px)] p-0">
          <div className="grid grid-cols-1 sm:grid-cols-[1fr_180px]">
            <div className="space-y-2 border-b border-[var(--border)] p-3 sm:border-b-0 sm:border-r">
              <div className="text-[0.75rem] font-medium">{t('time.absolute')}</div>
              <label className="block text-[0.6875rem] text-[var(--muted-foreground)]">
                {t('time.from')}
                <input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} className="mt-1 h-8 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-2 text-[0.75rem] text-[var(--foreground)]" />
              </label>
              <label className="block text-[0.6875rem] text-[var(--muted-foreground)]">
                {t('time.to')}
                <input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} className="mt-1 h-8 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-2 text-[0.75rem] text-[var(--foreground)]" />
              </label>
              <button type="button" onClick={applyAbsolute} className="h-8 w-full rounded-md bg-[var(--primary)] text-[0.75rem] font-medium text-[var(--primary-foreground)] hover:brightness-105">
                {t('time.apply')}
              </button>
              <p className="text-[0.6875rem] leading-snug text-[var(--muted-foreground)]">{t('time.zoomHint')}</p>
            </div>
            <div className="max-h-[320px] overflow-y-auto p-1 scrollbar-thin">
              {RELATIVE_RANGES.map((r) => {
                const active = range.kind === 'relative' && range.key === r.key;
                return (
                  <DropdownMenuItem
                    key={r.key}
                    onClick={() => onChange({ kind: 'relative', key: r.key })}
                    className={cn('flex cursor-pointer items-center justify-between rounded px-2.5 py-1.5 text-[0.8125rem]', active && 'text-[var(--primary)]')}
                  >
                    {t(`time.last.${r.key}`)}
                    {active && <Check className="h-3.5 w-3.5" />}
                  </DropdownMenuItem>
                );
              })}
            </div>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
      <button type="button" onClick={zoomOut} className={cn(triggerClass, '-ml-px rounded-l-none px-2')} title={t('time.zoomOut')} aria-label={t('time.zoomOut')}>
        <ZoomOut className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

export function RefreshPicker({
  value,
  onChange,
  onRefresh,
  refreshing,
  autoSeconds,
}: {
  value: string;
  onChange: (v: string) => void;
  onRefresh: () => void;
  refreshing: boolean;
  autoSeconds?: number;
}) {
  const { t } = useTranslation();
  const label = (v: string) => (v === 'off' ? t('time.refresh.off') : v === 'auto' ? t('time.refresh.auto', { seconds: autoSeconds ?? 15 }) : v);
  return (
    <div className="flex items-center">
      <button type="button" onClick={onRefresh} className={cn(triggerClass, 'rounded-r-none px-2')} title={t('time.refreshNow')} aria-label={t('time.refreshNow')}>
        <RefreshCw className={cn('h-3.5 w-3.5', refreshing && 'animate-spin')} />
      </button>
      <DropdownMenu>
        <DropdownMenuTrigger className={cn(triggerClass, '-ml-px rounded-l-none')}>
          <span className={cn('tabular', value !== 'off' && 'text-[var(--primary)]')}>{label(value)}</span>
          <ChevronDown className="h-3.5 w-3.5 opacity-60" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-40 p-1">
          {REFRESH_OPTIONS.map((opt) => (
            <DropdownMenuItem key={opt} onClick={() => onChange(opt)} className="flex cursor-pointer items-center justify-between rounded px-2.5 py-1.5 text-[0.8125rem]">
              {label(opt)}
              {opt === value && <Check className="h-3.5 w-3.5 text-[var(--primary)]" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
