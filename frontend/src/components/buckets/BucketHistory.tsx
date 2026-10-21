import * as React from 'react';
import { TimeSeriesChart } from '@/components/charts/TimeSeriesChart';
import { Segmented } from '@/components/ui/segmented';
import { useRangeQuery, type TimeRange } from '@/hooks/useTelemetry';
import { useTranslation } from '@/lib/i18n';
import type { MetricQuery } from '@/lib/telemetry';

const RANGES = ['24h', '7d', '30d', '90d'] as const;

/** Size and object-count history of one bucket from the metrics collector. */
export function BucketHistory({ bucket }: { bucket: string }) {
  const { t } = useTranslation();
  const [key, setKey] = React.useState<(typeof RANGES)[number]>('7d');
  const range: TimeRange = React.useMemo(() => ({ kind: 'relative', key }), [key]);
  const bytesQ = React.useMemo<MetricQuery[]>(() => [{ id: 'bytes', metric: 'bucket.bytes', match: { bucket }, fn: 'last' }], [bucket]);
  const objectsQ = React.useMemo<MetricQuery[]>(() => [{ id: 'objects', metric: 'bucket.objects', match: { bucket }, fn: 'last' }], [bucket]);
  const bytes = useRangeQuery(bytesQ, range, 60_000);
  const objects = useRangeQuery(objectsQ, range, 60_000);

  const series = (data: typeof bytes.data, label: string, color: string) =>
    data?.series?.[0] ? [{ key: label, label, color, values: data.series[0].values }] : [];

  return (
    <section className="rounded-lg border border-[var(--border)] bg-[var(--card)]">
      <header className="flex flex-wrap items-center gap-3 border-b border-[var(--border)] px-5 py-3">
        <div className="min-w-0 flex-1">
          <h2 className="text-[0.9375rem] font-semibold">{t('buckets.history.title')}</h2>
          <p className="text-[0.75rem] text-[var(--muted-foreground)]">{t('buckets.history.desc')}</p>
        </div>
        <Segmented size="sm" value={key} onChange={setKey} options={RANGES.map((r) => ({ value: r, label: r }))} />
      </header>
      <div className="grid grid-cols-1 gap-4 p-3 lg:grid-cols-2">
        <div>
          <div className="px-1 pb-1 text-[0.75rem] text-[var(--muted-foreground)]">{t('buckets.history.size')}</div>
          <TimeSeriesChart syncKey="bucket" timestamps={bytes.data?.timestamps ?? []} series={series(bytes.data, t('buckets.history.size'), 'var(--chart-blue)')} unit="bytes" height={160} legend="none" />
        </div>
        <div>
          <div className="px-1 pb-1 text-[0.75rem] text-[var(--muted-foreground)]">{t('buckets.history.objects')}</div>
          <TimeSeriesChart syncKey="bucket" timestamps={objects.data?.timestamps ?? []} series={series(objects.data, t('buckets.history.objects'), 'var(--chart-purple)')} unit="count" height={160} legend="none" />
        </div>
      </div>
    </section>
  );
}
