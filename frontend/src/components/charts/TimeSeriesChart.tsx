import * as React from 'react';
import uPlot from 'uplot';
import 'uplot/dist/uPlot.min.css';
import { cn } from '@/lib/utils';
import { formatAxis, formatDate, formatTime, formatValue, type Unit } from '@/lib/units';
import { useThemeVersion, resolveCssColor, withAlpha } from '@/lib/chart-theme';
import { useTranslation } from '@/lib/i18n';

export interface ChartSeries {
  key: string;
  label: string;
  color: string; // CSS color or var(--x)
  values: (number | null)[];
  dashed?: boolean;
}

export interface Threshold {
  value: number;
  color: string;
}

export interface TimeSeriesChartProps {
  timestamps: number[];
  series: ChartSeries[];
  unit: Unit;
  height?: number;
  stacked?: boolean;
  fillOpacity?: number;
  bars?: boolean;
  yMin?: number;
  yMax?: number;
  thresholds?: Threshold[];
  syncKey?: string;
  onZoom?: (from: number, to: number) => void;
  legend?: 'list' | 'table' | 'none';
  className?: string;
}

interface CursorState {
  idx: number;
  left: number;
  top: number;
}

function stack(series: ChartSeries[], hidden: Set<string>): (number | null)[][] {
  const out: (number | null)[][] = [];
  let acc: (number | null)[] | null = null;
  for (const s of series) {
    if (hidden.has(s.key)) {
      out.push(s.values.map(() => null));
      continue;
    }
    const cur = s.values.map((v, i) => {
      const base = acc ? acc[i] : null;
      if (v === null) return base;
      return (base ?? 0) + v;
    });
    out.push(cur);
    acc = cur;
  }
  return out;
}

function summarize(values: (number | null)[]) {
  let last: number | null = null;
  let max = -Infinity;
  let sum = 0;
  let n = 0;
  for (const v of values) {
    if (v === null || !Number.isFinite(v)) continue;
    last = v;
    max = Math.max(max, v);
    sum += v;
    n++;
  }
  return { last, max: n ? max : null, mean: n ? sum / n : null };
}

export const TimeSeriesChart = React.memo(function TimeSeriesChart({
  timestamps,
  series,
  unit,
  height = 220,
  stacked = false,
  fillOpacity = 0.18,
  bars = false,
  yMin,
  yMax,
  thresholds,
  syncKey = 'dashboard',
  onZoom,
  legend = 'list',
  className,
}: TimeSeriesChartProps) {
  const { t } = useTranslation();
  const wrapRef = React.useRef<HTMLDivElement>(null);
  const plotRef = React.useRef<HTMLDivElement>(null);
  const uRef = React.useRef<uPlot | null>(null);
  const [width, setWidth] = React.useState(0);
  const [hidden, setHidden] = React.useState<Set<string>>(new Set());
  const [cursor, setCursor] = React.useState<CursorState | null>(null);
  const hoverRef = React.useRef(false);
  const [hovered, setHovered] = React.useState(false);
  const themeVersion = useThemeVersion();
  const onZoomRef = React.useRef(onZoom);
  onZoomRef.current = onZoom;

  const signature = series.map((s) => `${s.key}|${s.color}|${s.dashed ? 1 : 0}`).join(',');
  const span = timestamps.length > 1 ? timestamps[timestamps.length - 1] - timestamps[0] : 0;

  const data = React.useMemo<uPlot.AlignedData>(() => {
    const ys = stacked ? stack(series, hidden) : series.map((s) => s.values);
    return [timestamps, ...ys] as uPlot.AlignedData;
  }, [timestamps, series, stacked, hidden]);

  const hasData = React.useMemo(() => series.some((s) => s.values.some((v) => v !== null)), [series]);

  React.useLayoutEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const w = Math.floor(entries[0].contentRect.width);
      setWidth((prev) => (Math.abs(prev - w) > 1 ? w : prev));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Build the plot when structure, size or theme changes.
  React.useEffect(() => {
    if (!plotRef.current || width === 0) return;
    const muted = resolveCssColor('var(--muted-foreground)');
    const grid = resolveCssColor('var(--grid)', 'rgba(128,128,128,0.12)');
    const fontFamily = getComputedStyle(document.documentElement).getPropertyValue('--app-font-mono').trim() || 'monospace';
    const axisFont = `11px ${fontFamily}`;
    const resolved = series.map((s) => resolveCssColor(s.color));

    let rafId = 0;
    const opts: uPlot.Options = {
      width,
      height,
      padding: [10, 12, 0, 2],
      legend: { show: false },
      cursor: {
        sync: { key: syncKey },
        drag: { x: true, y: false, setScale: false },
        points: { size: 7, width: 2, fill: (_u, i) => resolved[i - 1] ?? muted },
        focus: { prox: 24 },
      },
      focus: { alpha: 0.4 },
      select: { show: true, left: 0, top: 0, width: 0, height: 0 },
      scales: {
        x: { time: true },
        y: {
          range: (_u, min, max) => {
            const lo = yMin ?? (min >= 0 || min === null ? 0 : min * 1.1);
            let hi = yMax ?? (max === null || max <= lo ? lo + 1 : max + (max - lo) * 0.08);
            // Pull a threshold into view only when the data is already close
            // to it; a far-away threshold would flatten the series.
            if (thresholds?.length && yMax === undefined) {
              for (const th of thresholds) {
                if (th.value > hi && th.value <= hi * 1.6) hi = th.value * 1.05;
              }
            }
            return [lo, hi];
          },
        },
      },
      axes: [
        {
          stroke: muted,
          font: axisFont,
          grid: { stroke: grid, width: 1 },
          ticks: { show: false },
          gap: 6,
          size: 28,
          space: span > 2 * 86400 ? 70 : 60,
          values: (_u, splits) => splits.map((s) => (span > 2 * 86400 ? formatDate(s) : formatTime(s, span <= 900, false))),
        },
        {
          stroke: muted,
          font: axisFont,
          grid: { stroke: grid, width: 1 },
          ticks: { show: false },
          gap: 6,
          size: (_u, values) => {
            if (!values) return 40;
            const longest = values.reduce((m, v) => Math.max(m, String(v).length), 0);
            return Math.max(36, longest * 7 + 12);
          },
          values: (_u, splits) => splits.map((v) => formatAxis(v, unit)),
        },
      ],
      series: [
        {},
        ...series.map((s, i) => {
          const color = resolved[i];
          const base: uPlot.Series = {
            label: s.label,
            stroke: color,
            width: bars ? 0 : 1.6,
            dash: s.dashed ? [6, 4] : undefined,
            points: { show: false },
            spanGaps: false,
            show: !hidden.has(s.key),
          };
          if (bars) {
            base.paths = uPlot.paths.bars!({ size: [0.7, 40], gap: 1 });
            base.fill = withAlpha(color, 0.75);
          } else if (fillOpacity > 0) {
            base.fill = (u: uPlot) => {
              const g = u.ctx.createLinearGradient(0, u.bbox.top, 0, u.bbox.top + u.bbox.height);
              g.addColorStop(0, withAlpha(color, stacked ? fillOpacity * 1.6 : fillOpacity));
              g.addColorStop(1, withAlpha(color, stacked ? fillOpacity * 0.6 : 0.01));
              return g;
            };
          }
          return base;
        }),
      ],
      hooks: {
        setCursor: [
          (u) => {
            cancelAnimationFrame(rafId);
            rafId = requestAnimationFrame(() => {
              const idx = u.cursor.idx;
              if (idx === null || idx === undefined || u.cursor.left === undefined || u.cursor.left < 0) {
                setCursor(null);
              } else {
                setCursor({ idx, left: u.cursor.left + u.over.offsetLeft, top: (u.cursor.top ?? 0) + u.over.offsetTop });
              }
            });
          },
        ],
        setSelect: [
          (u) => {
            if (u.select.width > 4 && onZoomRef.current) {
              const from = u.posToVal(u.select.left, 'x');
              const to = u.posToVal(u.select.left + u.select.width, 'x');
              onZoomRef.current(Math.floor(from), Math.ceil(to));
            }
            u.setSelect({ left: 0, top: 0, width: 0, height: 0 }, false);
          },
        ],
        draw: [
          (u) => {
            if (!thresholds?.length) return;
            const { ctx, bbox } = u;
            ctx.save();
            for (const th of thresholds) {
              const y = u.valToPos(th.value, 'y', true);
              if (y < bbox.top || y > bbox.top + bbox.height) continue;
              ctx.strokeStyle = withAlpha(resolveCssColor(th.color), 0.8);
              ctx.setLineDash([4, 4]);
              ctx.lineWidth = 1;
              ctx.beginPath();
              ctx.moveTo(bbox.left, y);
              ctx.lineTo(bbox.left + bbox.width, y);
              ctx.stroke();
            }
            ctx.restore();
          },
        ],
      },
    };
    const u = new uPlot(opts, data, plotRef.current);
    uRef.current = u;
    return () => {
      cancelAnimationFrame(rafId);
      u.destroy();
      uRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [width, height, signature, unit, stacked, bars, fillOpacity, yMin, yMax, themeVersion, syncKey, span > 900, span > 2 * 86400, JSON.stringify(thresholds)]);

  React.useEffect(() => {
    uRef.current?.setData(data, true);
  }, [data]);

  React.useEffect(() => {
    const u = uRef.current;
    if (!u) return;
    series.forEach((s, i) => {
      const show = !hidden.has(s.key);
      if (u.series[i + 1] && u.series[i + 1].show !== show) u.setSeries(i + 1, { show });
    });
  }, [hidden, series]);

  const toggle = (key: string, solo: boolean) => {
    setHidden((prev) => {
      if (solo) {
        const others = series.filter((s) => s.key !== key).map((s) => s.key);
        const alreadySolo = others.every((k) => prev.has(k)) && !prev.has(key);
        return alreadySolo ? new Set() : new Set(others);
      }
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const tooltip = hovered && cursor && hasData ? (() => {
    const ts = timestamps[cursor.idx];
    if (ts === undefined) return null;
    const rows = series
      .map((s) => ({ s, v: s.values[cursor.idx] }))
      .filter((r) => !hidden.has(r.s.key))
      .sort((a, b) => (b.v ?? -Infinity) - (a.v ?? -Infinity));
    const flip = cursor.left > width * 0.6;
    const total = stacked ? rows.reduce((acc, r) => acc + (r.v ?? 0), 0) : null;
    return (
      <div
        className="pointer-events-none absolute z-20 min-w-[180px] max-w-[320px] rounded-md border border-[var(--border)] bg-[var(--popover)]/95 px-2.5 py-2 text-[0.75rem] shadow-xl backdrop-blur"
        style={{
          top: Math.min(Math.max(cursor.top - 20, 0), height - 60),
          left: flip ? undefined : cursor.left + 16,
          right: flip ? width - cursor.left + 16 : undefined,
        }}
      >
        <div className="mb-1.5 text-[var(--muted-foreground)]">{formatTime(ts, true, span > 86400)}</div>
        <div className="space-y-0.5">
          {rows.slice(0, 12).map(({ s, v }) => (
            <div key={s.key} className="flex items-center gap-2">
              <span className="h-2 w-2 shrink-0 rounded-sm" style={{ background: s.color }} />
              <span className="min-w-0 flex-1 truncate text-[var(--muted-foreground)]">{s.label}</span>
              <span className="tabular font-medium text-[var(--foreground)]">{formatValue(v, unit)}</span>
            </div>
          ))}
          {rows.length > 12 && <div className="text-[var(--muted-foreground)]">+{rows.length - 12}</div>}
          {total !== null && rows.length > 1 && (
            <div className="mt-1 flex items-center gap-2 border-t border-[var(--border)] pt-1">
              <span className="flex-1 text-[var(--muted-foreground)]">{t('charts.total')}</span>
              <span className="tabular font-medium">{formatValue(total, unit)}</span>
            </div>
          )}
        </div>
      </div>
    );
  })() : null;

  return (
    <div className={cn('flex h-full min-w-0 flex-col', className)}>
      <div
        ref={wrapRef}
        className="relative min-w-0"
        style={{ height }}
        onMouseEnter={() => { hoverRef.current = true; setHovered(true); }}
        onMouseLeave={() => { hoverRef.current = false; setHovered(false); }}
      >
        <div ref={plotRef} className="gu-uplot" />
        {!hasData && (
          <div className="absolute inset-0 flex items-center justify-center text-[0.75rem] text-[var(--muted-foreground)]">
            {t('charts.noData')}
          </div>
        )}
        {tooltip}
      </div>
      {legend !== 'none' && series.length > 0 && (
        legend === 'table' ? (
          <div className="mt-1 max-h-28 overflow-y-auto scrollbar-thin">
            <table className="w-full text-[0.7188rem]">
              <thead>
                <tr className="text-[var(--muted-foreground)]">
                  <th className="px-1 text-left font-normal" />
                  <th className="px-1 text-right font-normal">{t('charts.mean')}</th>
                  <th className="px-1 text-right font-normal">{t('charts.max')}</th>
                  <th className="px-1 text-right font-normal">{t('charts.last')}</th>
                </tr>
              </thead>
              <tbody>
                {series.map((s) => {
                  const st = summarize(s.values);
                  const off = hidden.has(s.key);
                  return (
                    <tr key={s.key} className={cn('cursor-pointer hover:bg-[var(--accent)]', off && 'opacity-40')} onClick={(e) => toggle(s.key, !(e.ctrlKey || e.metaKey))}>
                      <td className="px-1 py-0.5">
                        <span className="flex items-center gap-1.5">
                          <span className="h-[3px] w-3 shrink-0 rounded" style={{ background: s.color }} />
                          <span className="truncate">{s.label}</span>
                        </span>
                      </td>
                      <td className="px-1 text-right tabular">{formatValue(st.mean, unit)}</td>
                      <td className="px-1 text-right tabular">{formatValue(st.max, unit)}</td>
                      <td className="px-1 text-right tabular">{formatValue(st.last, unit)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 px-1">
            {series.map((s) => (
              <button
                key={s.key}
                type="button"
                onClick={(e) => toggle(s.key, !(e.ctrlKey || e.metaKey))}
                className={cn('flex items-center gap-1.5 text-[0.7188rem] text-[var(--muted-foreground)] hover:text-[var(--foreground)]', hidden.has(s.key) && 'opacity-40')}
                title={t('charts.legendHint')}
              >
                <span className="h-[3px] w-3 rounded" style={{ background: s.color }} />
                <span className="max-w-[180px] truncate">{s.label}</span>
              </button>
            ))}
          </div>
        )
      )}
    </div>
  );
});
