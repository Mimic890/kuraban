import * as React from 'react';
import { Info, Maximize2, Minimize2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import { TimeSeriesChart, type ChartSeries, type Threshold } from '@/components/charts/TimeSeriesChart';
import { useRangeQuery, type TimeRange } from '@/hooks/useTelemetry';
import { useBatchedRange, type PanelData } from '@/components/dashboard/batch';
import type { MetricQuery, RangeResult } from '@/lib/telemetry';
import { seriesColor } from '@/lib/chart-theme';
import { formatValue, type Unit } from '@/lib/units';
import { useSettingsStore } from '@/store/settings-store';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

export interface DashboardCtx {
  range: TimeRange;
  refetchInterval: number | false;
  onZoom: (from: number, to: number) => void;
  seriesLabel?: (queryId: string, name: string) => string;
}

export const DashboardContext = React.createContext<DashboardCtx | null>(null);

export function useDashboard() {
  const ctx = React.useContext(DashboardContext);
  if (!ctx) throw new Error('useDashboard outside DashboardContext');
  return ctx;
}

/**
 * Set by the dashboard grid for the panel it wraps: the size the user chose, whether
 * the dashboard is being edited, and the edit controls to show in the header.
 */
export interface PanelSlot {
  height: number;
  editing: boolean;
  /** Hidden panel shown in edit mode: rendered, but not fetched or refreshed. */
  inactive: boolean;
  expanded: boolean;
  toggleExpanded: () => void;
  controls: React.ReactNode;
  resizeHandle: React.ReactNode;
}

export const PanelSlotContext = React.createContext<PanelSlot | null>(null);

/** Body height for a panel: the grid slot's size, or the given default. */
export function usePanelHeight(fallback: number) {
  return React.useContext(PanelSlotContext)?.height ?? fallback;
}

/**
 * Data for a panel: from the dashboard's shared batch request when there is
 * one, otherwise a request of its own. Inactive (hidden) panels fetch nothing.
 */
function usePanelData(queries: MetricQuery[], enabled = true): PanelData {
  const { range, refetchInterval } = useDashboard();
  const inactive = !!React.useContext(PanelSlotContext)?.inactive;
  const batched = useBatchedRange(queries, enabled && !inactive);
  const direct = useRangeQuery(queries, range, refetchInterval, enabled && !inactive && !batched);
  return batched ?? { data: direct.data, isLoading: direct.isLoading, isFetching: direct.isFetching, isError: direct.isError };
}

export function PanelFrame({
  title,
  description,
  children,
  className,
  actions,
  loading,
  error,
  bodyClassName,
}: {
  title: React.ReactNode;
  description?: string;
  children: React.ReactNode;
  className?: string;
  actions?: React.ReactNode;
  loading?: boolean;
  error?: boolean;
  bodyClassName?: string;
}) {
  const { t } = useTranslation();
  const slot = React.useContext(PanelSlotContext);
  const editing = !!slot?.editing;
  return (
    <section
      className={cn(
        'group/panel relative flex h-full min-w-0 flex-col rounded-lg border border-[var(--border)] bg-[var(--card)]',
        editing && 'border-dashed border-[var(--input)]',
        className,
      )}
    >
      <header className="flex h-9 shrink-0 items-center gap-1.5 px-3">
        <h3 className="min-w-0 truncate text-[0.8125rem] font-medium text-[var(--foreground)]">{title}</h3>
        {description && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button type="button" className="text-[var(--muted-foreground)] opacity-60 hover:opacity-100" aria-label={description}>
                <Info className="h-3.5 w-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top" className="max-w-xs">{description}</TooltipContent>
          </Tooltip>
        )}
        <div className="ml-auto flex items-center gap-1">
          {error && <span className="text-[0.6875rem] text-[var(--destructive)]">{t('charts.error')}</span>}
          {loading && <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--primary)]" />}
          {editing ? slot?.controls : actions}
        </div>
      </header>
      <div className={cn('min-h-0 flex-1 px-2 pb-2', bodyClassName)}>{children}</div>
      {editing && slot?.resizeHandle}
    </section>
  );
}

export function PanelSkeleton({ height }: { height: number }) {
  return (
    <div className="relative overflow-hidden rounded-md" style={{ height }}>
      <div className="absolute inset-0 animate-pulse bg-[linear-gradient(90deg,transparent,var(--accent),transparent)] opacity-60" />
    </div>
  );
}

export interface SeriesStyle {
  color?: string;
  label?: string;
  dashed?: boolean;
}

export function toChartSeries(
  result: RangeResult | undefined,
  queries: MetricQuery[],
  opts: { styles?: Record<string, SeriesStyle>; labels?: Record<string, string>; seriesLabel?: (q: string, n: string) => string },
): ChartSeries[] {
  if (!result) return [];
  const byId = new Map(queries.map((q) => [q.id, q]));
  return (result.series ?? []).map((s, i) => {
    const key = `${s.query}:${s.name}`;
    const style = opts.styles?.[s.name] ?? opts.styles?.[s.query] ?? {};
    const q = byId.get(s.query);
    let label = style.label ?? opts.labels?.[s.query] ?? s.name;
    if (q?.group_by && s.name !== s.query) {
      label = opts.seriesLabel?.(s.query, s.name) ?? s.name;
      if (queries.length > 1 && opts.labels?.[s.query]) label = `${opts.labels[s.query]} · ${label}`;
    }
    return { key, label, color: style.color ?? seriesColor(i), values: s.values, dashed: style.dashed };
  });
}

export interface TimeSeriesPanelProps {
  title: string;
  description?: string;
  queries: MetricQuery[];
  unit: Unit;
  labels?: Record<string, string>;
  styles?: Record<string, SeriesStyle>;
  stacked?: boolean;
  bars?: boolean;
  fillOpacity?: number;
  height?: number;
  thresholds?: Threshold[];
  yMin?: number;
  yMax?: number;
  legend?: 'list' | 'table' | 'none';
  className?: string;
}

export function TimeSeriesPanel({
  title,
  description,
  queries,
  unit,
  labels,
  styles,
  stacked,
  bars,
  fillOpacity,
  height = 200,
  thresholds,
  yMin,
  yMax,
  legend = 'list',
  className,
}: TimeSeriesPanelProps) {
  const { onZoom, seriesLabel } = useDashboard();
  const slot = React.useContext(PanelSlotContext);
  const [localExpanded, setLocalExpanded] = React.useState(false);
  const expanded = slot ? slot.expanded : localExpanded;
  const toggleExpanded = slot ? slot.toggleExpanded : () => setLocalExpanded((e) => !e);
  const baseHeight = slot?.height ?? height;
  const { t } = useTranslation();
  const q = usePanelData(queries);
  const series = React.useMemo(
    () => toChartSeries(q.data, queries, { styles, labels, seriesLabel }),
    [q.data, queries, styles, labels, seriesLabel],
  );

  const chartHeight = expanded ? Math.max(baseHeight * 2, 420) : baseHeight;
  return (
    <PanelFrame
      title={title}
      description={description}
      loading={q.isFetching}
      error={q.isError}
      className={cn(!slot && expanded && 'col-span-full', className)}
      actions={
        <button
          type="button"
          onClick={toggleExpanded}
          className="rounded p-0.5 text-[var(--muted-foreground)] opacity-0 transition-opacity hover:text-[var(--foreground)] group-hover/panel:opacity-100 focus:opacity-100"
          aria-label={expanded ? t('charts.collapse') : t('charts.expand')}
          title={expanded ? t('charts.collapse') : t('charts.expand')}
        >
          {expanded ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
        </button>
      }
    >
      {q.isLoading ? (
        <PanelSkeleton height={chartHeight} />
      ) : (
        <TimeSeriesChart
          timestamps={q.data?.timestamps ?? []}
          series={series}
          unit={unit}
          height={chartHeight}
          stacked={stacked}
          bars={bars}
          fillOpacity={fillOpacity}
          thresholds={thresholds}
          yMin={yMin}
          yMax={yMax}
          onZoom={onZoom}
          legend={expanded && legend === 'list' ? 'table' : legend}
        />
      )}
    </PanelFrame>
  );
}

export type Tone = 'ok' | 'warn' | 'crit' | 'neutral' | 'info';

export const toneColor: Record<Tone, string> = {
  ok: 'var(--success)',
  warn: 'var(--warning)',
  crit: 'var(--destructive)',
  neutral: 'var(--foreground)',
  info: 'var(--info)',
};

function Sparkline({ values, color, height = 34 }: { values: (number | null)[]; color: string; height?: number }) {
  const id = React.useId();
  const pts = values.map((v, i) => [i, v] as const).filter((p): p is readonly [number, number] => p[1] !== null);
  if (pts.length < 2) return <div style={{ height }} />;
  const min = Math.min(...pts.map((p) => p[1]));
  const max = Math.max(...pts.map((p) => p[1]));
  const n = values.length - 1 || 1;
  const w = 200;
  const y = (v: number) => (max === min ? height / 2 : height - 2 - ((v - min) / (max - min)) * (height - 4));
  const line = pts.map(([i, v], k) => `${k === 0 ? 'M' : 'L'}${((i / n) * w).toFixed(1)},${y(v).toFixed(1)}`).join(' ');
  const area = `${line} L${((pts[pts.length - 1][0] / n) * w).toFixed(1)},${height} L${((pts[0][0] / n) * w).toFixed(1)},${height} Z`;
  return (
    <svg viewBox={`0 0 ${w} ${height}`} preserveAspectRatio="none" className="block w-full" style={{ height }}>
      <defs>
        <linearGradient id={id} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.35" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${id})`} />
      <path d={line} fill="none" stroke={color} strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

export interface StatPanelProps {
  title: string;
  description?: string;
  query?: MetricQuery;
  unit: Unit;
  reduce?: 'last' | 'mean' | 'max' | 'min';
  tone?: (value: number | null) => Tone;
  value?: React.ReactNode;
  sub?: React.ReactNode;
  sparkline?: boolean;
  className?: string;
}

function reduceValues(values: (number | null)[], how: StatPanelProps['reduce']): number | null {
  const nums = values.filter((v): v is number => v !== null && Number.isFinite(v));
  if (nums.length === 0) return null;
  switch (how) {
    case 'mean':
      return nums.reduce((a, b) => a + b, 0) / nums.length;
    case 'max':
      return Math.max(...nums);
    case 'min':
      return Math.min(...nums);
    default:
      return nums[nums.length - 1];
  }
}

export function StatPanel({ title, description, query, unit, reduce = 'last', tone, value, sub, sparkline = true, className }: StatPanelProps) {
  const queries = React.useMemo(() => (query ? [query] : []), [query]);
  const q = usePanelData(queries, !!query);
  const values = q.data?.series?.[0]?.values ?? [];
  const reduced = reduceValues(values, reduce);
  const toneName = tone ? tone(reduced) : 'neutral';
  const color = toneColor[toneName];
  const density = useSettingsStore((s) => s.density);
  const slotHeight = React.useContext(PanelSlotContext)?.height;

  return (
    <PanelFrame title={title} description={description} loading={q.isFetching && !!query} className={className} bodyClassName="px-3 pb-0 flex flex-col">
      <div
        className={cn('flex flex-1 flex-col justify-end', !slotHeight && (density === 'compact' ? 'min-h-[64px]' : 'min-h-[84px]'))}
        style={slotHeight ? { minHeight: slotHeight } : undefined}
      >
        {q.isLoading && query ? (
          <div className="h-7 w-24 animate-pulse rounded bg-[var(--accent)]" />
        ) : (
          <div className="truncate text-[1.625rem] font-semibold leading-none tracking-tight tabular" style={{ color }} data-numeric>
            {value ?? formatValue(reduced, unit)}
          </div>
        )}
        {sub && <div className="mt-1 truncate text-[0.7188rem] text-[var(--muted-foreground)]">{sub}</div>}
        <div className="mt-2 -mx-3">
          {sparkline && query ? <Sparkline values={values} color={toneName === 'neutral' ? 'var(--info)' : color} /> : <div className="h-2" />}
        </div>
      </div>
    </PanelFrame>
  );
}

export interface BarGaugeItem {
  key: string;
  label: React.ReactNode;
  value: number;
  max: number;
  detail?: React.ReactNode;
  tone?: Tone;
}

export function BarGauge({ items, format }: { items: BarGaugeItem[]; format: (v: number, item: BarGaugeItem) => string }) {
  return (
    <div className="space-y-2.5 px-1 py-1">
      {items.map((item) => {
        const pct = item.max > 0 ? Math.min(100, (item.value / item.max) * 100) : 0;
        const color = toneColor[item.tone ?? 'info'];
        return (
          <div key={item.key}>
            <div className="mb-1 flex items-baseline gap-2 text-[0.75rem]">
              <span className="min-w-0 flex-1 truncate">{item.label}</span>
              {item.detail && <span className="shrink-0 text-[var(--muted-foreground)]">{item.detail}</span>}
              <span className="w-20 shrink-0 text-right font-medium tabular" style={{ color }}>{format(item.value, item)}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-sm bg-[var(--accent)]">
              <div className="h-full rounded-sm transition-[width] duration-500" style={{ width: `${pct}%`, background: `linear-gradient(90deg, color-mix(in srgb, ${color} 55%, transparent), ${color})` }} />
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function DashboardTooltipProvider({ children }: { children: React.ReactNode }) {
  return <TooltipProvider delayDuration={250}>{children}</TooltipProvider>;
}
