import { StatPanel, usePanelData } from '@/components/dashboard/panels';
import { errorRateTone, MIN_REQUESTS_FOR_PCT } from '@/lib/error-rate';
import { useTranslation } from '@/lib/i18n';
import type { MetricQuery, RangeResult } from '@/lib/telemetry';
import { formatValue } from '@/lib/units';

const QUERIES: MetricQuery[] = [
  { id: 'requests', metric: 's3.requests', fn: 'increase' },
  { id: 'errors', metric: 's3.errors', fn: 'increase' },
  { id: 'server', metric: 's3.errors', fn: 'increase', match: { class: '5xx' } },
];
const SPARKLINE: MetricQuery = { id: 'err', metric: 's3.errors', fn: 'ratio', denominator: 's3.requests', scale: 100 };

function total(data: RangeResult | undefined, id: string) {
  const values = data?.series?.find((s) => s.query === id)?.values ?? [];
  return Math.round(values.reduce<number>((sum, v) => sum + (v ?? 0), 0));
}

/** Counts errors over the whole selected range, so a lone failure cannot read as 100% and stick while idle. */
export function ErrorRateStat({ warnPct }: { warnPct: number }) {
  const { t, language } = useTranslation();
  const { data } = usePanelData(QUERIES);
  const counts = { requests: total(data, 'requests'), errors: total(data, 'errors'), serverErrors: total(data, 'server') };
  const number = new Intl.NumberFormat(language);
  const errors = number.format(counts.errors);
  const requests = number.format(counts.requests);
  const tone = errorRateTone(counts, warnPct);

  let value: string;
  let sub: string;
  if (counts.requests <= 0) {
    value = '—';
    sub = t('dashboard.stat.errorNone');
  } else if (counts.requests < MIN_REQUESTS_FOR_PCT) {
    value = t('dashboard.stat.errorOf', { errors, requests });
    sub = t('dashboard.stat.errorFailed');
  } else {
    value = formatValue((counts.errors / counts.requests) * 100, 'percent');
    sub = t('dashboard.stat.errorOf', { errors, requests });
  }
  // First, because it drives the alert and the end of a long line gets cut off.
  if (counts.serverErrors > 0) sub = `5xx: ${number.format(counts.serverErrors)} · ${sub}`;

  return (
    <StatPanel
      title={t('dashboard.stat.errorRate')}
      description={t('dashboard.stat.errorDesc', { pct: warnPct })}
      unit="percent"
      query={SPARKLINE}
      value={value}
      tone={() => tone}
      sub={sub}
    />
  );
}
