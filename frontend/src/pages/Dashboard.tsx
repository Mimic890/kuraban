import * as React from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Activity, AlertTriangle, Check, Info, LayoutGrid, RotateCcw, Server } from 'lucide-react';
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip as ReTooltip } from 'recharts';
import { PageHeader } from '@/components/ui/page-header';
import { EmptyState } from '@/components/ui/empty-state';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useClusterStore } from '@/store/cluster-store';
import { useSettingsStore } from '@/store/settings-store';
import { useTranslation } from '@/lib/i18n';
import { garageApi } from '@/lib/api';
import { useClusterHealth, useDashboardMetrics } from '@/hooks/useApi';
import { refreshMs, telemetryKeys, useAppSettings, useTelemetryStatus, type TimeRange } from '@/hooks/useTelemetry';
import type { MetricQuery } from '@/lib/telemetry';
import { formatBytesValue, formatRelative, formatValue } from '@/lib/units';
import { seriesColor } from '@/lib/chart-theme';
import {
  BarGauge,
  DashboardContext,
  DashboardTooltipProvider,
  PanelFrame,
  StatPanel,
  TimeSeriesPanel,
  type DashboardCtx,
  type Tone,
} from '@/components/dashboard/panels';
import { RefreshPicker, TimeRangePicker } from '@/components/dashboard/time-controls';
import { DashboardBoard } from '@/components/dashboard/board';
import { QueryBatchProvider } from '@/components/dashboard/batch';
import { useDashboardLayout, type PanelSize } from '@/store/dashboard-layout-store';
import type { ClusterNode } from '@/types';

// Query definitions live at module scope so their identity (and the React
// Query keys derived from them) stay stable across renders.
const Q = {
  s3Rate: { id: 's3', metric: 's3.requests', fn: 'rate' } as MetricQuery,
  errorPct: { id: 'err', metric: 's3.errors', fn: 'ratio', denominator: 's3.requests', scale: 100 } as MetricQuery,
  p95: { id: 'p95', metric: 's3.latency', match: { quantile: 'p95' }, scale: 1000 } as MetricQuery,
  storageBytes: { id: 'bytes', metric: 'storage.bytes', fn: 'last' } as MetricQuery,
  storageObjects: { id: 'objects', metric: 'storage.objects', fn: 'last' } as MetricQuery,
  buckets: { id: 'buckets', metric: 'storage.buckets', fn: 'last' } as MetricQuery,
  keys: { id: 'keys', metric: 'storage.keys', fn: 'last' } as MetricQuery,
  diskUsed: { id: 'disk', metric: 'node.data_used_pct', fn: 'max', agg: 'max' } as MetricQuery,
  nodesUp: { id: 'up', metric: 'cluster.storage_nodes_up', fn: 'min', agg: 'min' } as MetricQuery,
  partitionsOk: { id: 'parts', metric: 'cluster.partitions_all_ok', fn: 'min', agg: 'min' } as MetricQuery,
  ioRate: { id: 'io', metric: 'block.bytes_written', fn: 'rate' } as MetricQuery,
  ioRead: { id: 'ior', metric: 'block.bytes_read', fn: 'rate' } as MetricQuery,
  resync: { id: 'resync', metric: 'block.resync_queue', fn: 'last' } as MetricQuery,
};

const P = {
  requestsByEndpoint: [{ id: 'req', metric: 's3.requests', fn: 'rate', group_by: 'endpoint', top_k: 10 }] as MetricQuery[],
  errorsByStatus: [{ id: 'err', metric: 's3.errors', fn: 'rate', group_by: 'status', top_k: 8 }] as MetricQuery[],
  errorsByEndpoint: [{ id: 'err', metric: 's3.errors', fn: 'rate', group_by: 'endpoint', top_k: 8 }] as MetricQuery[],
  latency: [{ id: 'lat', metric: 's3.latency', group_by: 'quantile', scale: 1000 }] as MetricQuery[],
  latencyByEndpoint: [{ id: 'avg', metric: 's3.duration_sum', fn: 'ratio', denominator: 's3.requests', group_by: 'endpoint', scale: 1000, top_k: 8 }] as MetricQuery[],
  errorRatio: [
    { id: 'total', metric: 's3.errors', fn: 'ratio', denominator: 's3.requests', scale: 100 },
    { id: '5xx', metric: 's3.errors', fn: 'ratio', denominator: 's3.requests', match: { class: '5xx' }, scale: 100 },
  ] as MetricQuery[],
  otherApis: [
    { id: 'web', metric: 'web.requests', fn: 'rate' },
    { id: 'k2v', metric: 'k2v.requests', fn: 'rate' },
    { id: 'admin', metric: 'admin.requests', fn: 'rate' },
  ] as MetricQuery[],
  webLatency: [{ id: 'web', metric: 'web.latency', group_by: 'quantile', scale: 1000 }] as MetricQuery[],
  storageBytes: [{ id: 'bytes', metric: 'storage.bytes' }] as MetricQuery[],
  storageObjects: [{ id: 'objects', metric: 'storage.objects' }] as MetricQuery[],
  bucketBytes: [{ id: 'b', metric: 'bucket.bytes', group_by: 'bucket', top_k: 8 }] as MetricQuery[],
  bucketObjects: [{ id: 'o', metric: 'bucket.objects', group_by: 'bucket', top_k: 8 }] as MetricQuery[],
  multipart: [
    { id: 'mpu', metric: 'storage.multipart_uploads' },
  ] as MetricQuery[],
  diskUsedByNode: [{ id: 'used', metric: 'node.data_used_pct', group_by: 'node' }] as MetricQuery[],
  dataAvail: [{ id: 'avail', metric: 'node.data_avail', group_by: 'node' }] as MetricQuery[],
  metaAvail: [{ id: 'meta', metric: 'node.meta_avail', group_by: 'node' }] as MetricQuery[],
  nodes: [
    { id: 'known', metric: 'cluster.nodes_known' },
    { id: 'connected', metric: 'cluster.nodes_connected' },
    { id: 'storage', metric: 'cluster.storage_nodes' },
    { id: 'storage_up', metric: 'cluster.storage_nodes_up' },
  ] as MetricQuery[],
  partitions: [
    { id: 'total', metric: 'cluster.partitions' },
    { id: 'quorum', metric: 'cluster.partitions_quorum' },
    { id: 'ok', metric: 'cluster.partitions_all_ok' },
  ] as MetricQuery[],
  blockIo: [
    { id: 'write', metric: 'block.bytes_written', fn: 'rate' },
    { id: 'read', metric: 'block.bytes_read', fn: 'rate' },
  ] as MetricQuery[],
  blockOps: [
    { id: 'writes', metric: 'block.writes', fn: 'rate' },
    { id: 'reads', metric: 'block.reads', fn: 'rate' },
  ] as MetricQuery[],
  blockLatency: [
    { id: 'write', metric: 'block.write_seconds', fn: 'ratio', denominator: 'block.writes', scale: 1000 },
    { id: 'read', metric: 'block.read_seconds', fn: 'ratio', denominator: 'block.reads', scale: 1000 },
  ] as MetricQuery[],
  resync: [
    { id: 'queue', metric: 'block.resync_queue' },
    { id: 'errored', metric: 'block.resync_errored' },
  ] as MetricQuery[],
  tableQueues: [
    { id: 'gc', metric: 'table.gc_queue' },
    { id: 'merkle', metric: 'table.merkle_queue' },
    { id: 'insert', metric: 'table.insert_queue' },
  ] as MetricQuery[],
  tableOps: [
    { id: 'gets', metric: 'table.gets', fn: 'rate' },
    { id: 'puts', metric: 'table.puts', fn: 'rate' },
    { id: 'updates', metric: 'table.updates', fn: 'rate' },
  ] as MetricQuery[],
  tableItems: [{ id: 'items', metric: 'table.items', group_by: 'table', top_k: 8 }] as MetricQuery[],
  rpc: [
    { id: 'rpc', metric: 'rpc.requests', fn: 'rate' },
    { id: 'errors', metric: 'rpc.errors', fn: 'rate' },
  ] as MetricQuery[],
  rpcLatency: [{ id: 'rpc', metric: 'rpc.latency', group_by: 'quantile', scale: 1000 }] as MetricQuery[],
  blockMemory: [
    { id: 'ram', metric: 'block.ram_buffer_free' },
  ] as MetricQuery[],
  collector: [{ id: 'scrape', metric: 'collector.scrape_seconds', scale: 1000 }] as MetricQuery[],
};

const QUANTILE_STYLES = {
  p50: { color: 'var(--chart-green)', label: 'p50' },
  p95: { color: 'var(--chart-olive)', label: 'p95' },
  p99: { color: 'var(--chart-red)', label: 'p99' },
};

function useNodeNames(enabled: boolean) {
  const clusterId = useClusterStore((s) => s.activeClusterId);
  return useQuery({
    queryKey: ['cluster', 'status', clusterId ?? 'none'],
    queryFn: () => garageApi.getClusterStatus(),
    enabled: enabled && !!clusterId,
    refetchInterval: 30_000,
    staleTime: 10_000,
    retry: false,
  });
}

function StatusBanner({ tone, icon, children }: { tone: 'info' | 'warn'; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <div
      className="flex items-start gap-2.5 rounded-lg border px-3 py-2.5 text-[0.8125rem]"
      style={{
        borderColor: `color-mix(in srgb, ${tone === 'warn' ? 'var(--warning)' : 'var(--info)'} 35%, transparent)`,
        background: `color-mix(in srgb, ${tone === 'warn' ? 'var(--warning)' : 'var(--info)'} 8%, transparent)`,
      }}
    >
      <span className="mt-0.5 shrink-0" style={{ color: tone === 'warn' ? 'var(--warning)' : 'var(--info)' }}>{icon}</span>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

export function Dashboard() {
  const { t, language } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { clusters, activeClusterId } = useClusterStore();
  const { dashboardRange, setDashboardRange, dashboardRefresh, setDashboardRefresh } = useSettingsStore();
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = React.useState(false);
  const resetLayout = useDashboardLayout((s) => s.reset);
  const layoutMode = useDashboardLayout((s) => s.mode);
  const setLayoutMode = useDashboardLayout((s) => s.setMode);
  const hasCluster = clusters.length > 0 && !!activeClusterId;

  const settingsQ = useAppSettings(hasCluster);
  const statusQ = useTelemetryStatus(hasCluster ? 15_000 : false);
  const healthQ = useClusterHealth(hasCluster);
  const metricsQ = useDashboardMetrics(hasCluster);
  const nodesQ = useNodeNames(hasCluster);

  const interval = settingsQ.data?.settings.monitoring.interval_seconds ?? statusQ.data?.monitoring.interval_seconds;
  const thresholds = settingsQ.data?.settings.thresholds ?? { disk_warn_pct: 80, disk_crit_pct: 90, error_rate_warn_pct: 5, latency_warn_ms: 500 };

  const absFrom = Number(params.get('from'));
  const absTo = Number(params.get('to'));
  const range: TimeRange = React.useMemo(
    () => (absFrom > 0 && absTo > absFrom ? { kind: 'absolute', from: absFrom, to: absTo } : { kind: 'relative', key: dashboardRange }),
    [absFrom, absTo, dashboardRange],
  );
  const setRange = React.useCallback(
    (r: TimeRange) => {
      if (r.kind === 'absolute') {
        setParams({ from: String(r.from), to: String(r.to) }, { replace: false });
      } else {
        setDashboardRange(r.key);
        if (params.has('from')) setParams({}, { replace: false });
      }
    },
    [params, setParams, setDashboardRange],
  );

  const nodeNames = React.useMemo(() => {
    const map = new Map<string, string>();
    for (const n of nodesQ.data?.nodes ?? []) map.set(n.id.slice(0, 16), n.hostname || n.id.slice(0, 8));
    return map;
  }, [nodesQ.data]);

  const seriesLabel = React.useCallback((_q: string, name: string) => nodeNames.get(name) ?? name, [nodeNames]);
  const ctx: DashboardCtx = React.useMemo(
    () => ({
      range,
      refetchInterval: refreshMs(dashboardRefresh, interval),
      onZoom: (from: number, to: number) => setRange({ kind: 'absolute', from, to }),
      seriesLabel,
    }),
    [range, dashboardRefresh, interval, setRange, seriesLabel],
  );

  const refreshing = queryClient.isFetching({ queryKey: telemetryKeys.all }) > 0;
  const refreshAll = () => {
    void queryClient.invalidateQueries({ queryKey: telemetryKeys.all });
    void healthQ.refetch();
    void metricsQ.refetch();
    void nodesQ.refetch();
  };

  if (clusters.length === 0) {
    return (
      <div>
        <PageHeader title={t('nav.dashboard')} subtitle={t('dashboard.welcome_subtitle')} />
        <div className="flex items-center justify-center px-6 py-12">
          <EmptyState
            icon={<Server />}
            title={t('dashboard.no_clusters')}
            description={t('dashboard.no_clusters.desc')}
            tone="primary"
            action={<Button onClick={() => navigate('/connections', { state: { addCluster: true } })}>{t('dashboard.add_btn')}</Button>}
          />
        </div>
      </div>
    );
  }

  const health = healthQ.data;
  const collector = statusQ.data?.collector;
  const monitoring = statusQ.data?.monitoring;
  const clusterName = clusters.find((c) => c.id === activeClusterId)?.name ?? '';
  const healthTone: Tone = !health ? 'neutral' : health.status === 'healthy' ? 'ok' : health.status === 'degraded' ? 'warn' : 'crit';
  const number = new Intl.NumberFormat(language);

  const diskTone = (v: number | null): Tone => (v === null ? 'neutral' : v >= thresholds.disk_crit_pct ? 'crit' : v >= thresholds.disk_warn_pct ? 'warn' : 'ok');
  const errTone = (v: number | null): Tone => (v === null ? 'neutral' : v >= thresholds.error_rate_warn_pct ? 'crit' : v > 0 ? 'warn' : 'ok');
  const latTone = (v: number | null): Tone => (v === null ? 'neutral' : v >= thresholds.latency_warn_ms ? 'warn' : 'ok');

  const usage = metricsQ.data?.usageByBucket ?? [];
  const sortedUsage = [...usage].sort((a, b) => b.size - a.size);
  const totalSize = metricsQ.data?.totalSize ?? 0;
  const donut = sortedUsage.slice(0, 7).map((b, i) => ({ name: b.bucketName, value: b.size, color: seriesColor(i) }));
  const rest = sortedUsage.slice(7).reduce((a, b) => a + b.size, 0);
  if (rest > 0) donut.push({ name: t('dashboard.other'), value: rest, color: 'var(--chart-gray)' });

  const nodes: ClusterNode[] = nodesQ.data?.nodes ?? [];

  return (
    <DashboardTooltipProvider>
      <DashboardContext.Provider value={ctx}>
        <QueryBatchProvider range={range} refetchInterval={ctx.refetchInterval}>
        <div className="pb-8">
          <PageHeader
            title={t('nav.dashboard')}
            subtitle={
              <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="font-medium text-[var(--foreground)]">{clusterName}</span>
                {health && (
                  <Badge variant={healthTone === 'ok' ? 'success' : healthTone === 'warn' ? 'warning' : 'danger'} className="px-1.5 py-0 text-[0.6875rem]">
                    {t(`dashboard.health_${health.status === 'healthy' ? 'healthy' : health.status === 'degraded' ? 'degraded' : 'unhealthy'}`)}
                  </Badge>
                )}
                {monitoring?.enabled && collector?.last_scrape && (
                  <span className="flex items-center gap-1.5">
                    <span className="relative flex h-2 w-2">
                      <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--success)] opacity-50" />
                      <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--success)]" />
                    </span>
                    {t('dashboard.collector.live', { interval: monitoring.interval_seconds, ago: formatRelative(collector.last_scrape) })}
                  </span>
                )}
              </span>
            }
            actions={
              <div className="flex flex-wrap items-center gap-2">
                {editing ? (
                  <>
                    <div className="flex rounded-md border border-[var(--border)] p-0.5" role="radiogroup" aria-label={t('dashboard.edit.mode')}>
                      {(['grouped', 'free'] as const).map((m) => (
                        <button
                          key={m}
                          type="button"
                          role="radio"
                          aria-checked={layoutMode === m}
                          onClick={() => setLayoutMode(m)}
                          title={t(`dashboard.edit.mode.${m}.hint`)}
                          className={`rounded px-2 py-0.5 text-[0.75rem] ${layoutMode === m ? 'bg-[var(--accent)] font-medium text-[var(--foreground)]' : 'text-[var(--muted-foreground)] hover:text-[var(--foreground)]'}`}
                        >
                          {t(`dashboard.edit.mode.${m}`)}
                        </button>
                      ))}
                    </div>
                    <Button variant="ghost" size="sm" className="gap-1.5" onClick={() => resetLayout()} title={t('dashboard.edit.resetHint')}>
                      <RotateCcw className="h-3.5 w-3.5" />
                      {t('dashboard.edit.reset')}
                    </Button>
                    <Button size="sm" className="gap-1.5" onClick={() => setEditing(false)}>
                      <Check className="h-3.5 w-3.5" />
                      {t('dashboard.edit.done')}
                    </Button>
                  </>
                ) : (
                  <Button variant="secondary" size="sm" className="gap-1.5" onClick={() => setEditing(true)}>
                    <LayoutGrid className="h-3.5 w-3.5" />
                    {t('dashboard.edit.start')}
                  </Button>
                )}
                <TimeRangePicker range={range} onChange={setRange} />
                <RefreshPicker value={dashboardRefresh} onChange={setDashboardRefresh} onRefresh={refreshAll} refreshing={refreshing} autoSeconds={interval} />
              </div>
            }
          />

          <div className="space-y-5 px-3 pt-3 md:px-4">
            {monitoring && !monitoring.enabled && (
              <StatusBanner tone="info" icon={<Info className="h-4 w-4" />}>
                {t('dashboard.collector.disabled')}{' '}
                <Link to="/settings?section=monitoring" className="font-medium text-[var(--primary)] hover:underline">{t('dashboard.collector.enable')}</Link>
              </StatusBanner>
            )}
            {monitoring?.enabled && collector && !collector.metrics_available && (
              <StatusBanner tone="warn" icon={<AlertTriangle className="h-4 w-4" />}>
                <div className="font-medium">{t('dashboard.collector.noMetrics')}</div>
                <div className="mt-0.5 text-[0.75rem] text-[var(--muted-foreground)]">{t('dashboard.collector.noMetricsHint')}</div>
              </StatusBanner>
            )}
            {monitoring?.enabled && collector?.last_error && collector.metrics_available && (
              <StatusBanner tone="warn" icon={<AlertTriangle className="h-4 w-4" />}>
                <div className="font-medium">{t('dashboard.collector.partial')}</div>
                <div className="mt-0.5 break-all font-mono text-[0.6875rem] text-[var(--muted-foreground)]">{collector.last_error}</div>
              </StatusBanner>
            )}
            {monitoring?.enabled && statusQ.data && statusQ.data.cluster.raw_samples === 0 && (
              <StatusBanner tone="info" icon={<Activity className="h-4 w-4" />}>
                {t('dashboard.collector.warming', { interval: monitoring.interval_seconds })}
              </StatusBanner>
            )}

            <DashboardBoard
              editing={editing}
              rows={[
            { id: 'overview', title: t('dashboard.row.overview'), panels: [
                  { id: 'overview.health', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.health'), render: () => (
                    <StatPanel
                      title={t('dashboard.stat.health')}
                      unit="short"
                      value={health ? t(`dashboard.health_${health.status === 'healthy' ? 'healthy' : health.status === 'degraded' ? 'degraded' : 'unhealthy'}`) : '—'}
                      tone={() => healthTone}
                      query={Q.nodesUp}
                      sub={health ? t('dashboard.stat.nodesSub', { up: number.format(health.storageNodesUp), total: number.format(health.storageNodes), connected: number.format(health.connectedNodes), known: number.format(health.knownNodes) }) : undefined}
                    />
                  ) },
                  { id: 'overview.partitions', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.partitions'), render: () => (
                    <StatPanel
                      title={t('dashboard.stat.partitions')}
                      unit="count"
                      query={Q.partitionsOk}
                      value={health ? `${number.format(health.partitionsAllOk)}/${number.format(health.partitions)}` : undefined}
                      tone={() => (!health ? 'neutral' : health.partitionsAllOk === health.partitions ? 'ok' : health.partitionsQuorum === health.partitions ? 'warn' : 'crit')}
                      sub={health ? t('dashboard.stat.quorumSub', { quorum: number.format(health.partitionsQuorum) }) : undefined}
                    />
                  ) },
                  { id: 'overview.stored', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.stored'), render: () => (
                    <StatPanel title={t('dashboard.stat.stored')} unit="bytes" query={Q.storageBytes} value={metricsQ.data ? formatBytesValue(metricsQ.data.totalSize) : undefined} sub={metricsQ.data ? t('dashboard.stat.bucketsSub', { count: number.format(metricsQ.data.bucketCount) }) : undefined} />
                  ) },
                  { id: 'overview.objects', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.objects'), render: () => (
                    <StatPanel title={t('dashboard.stat.objects')} unit="count" query={Q.storageObjects} value={metricsQ.data ? number.format(metricsQ.data.objectCount) : undefined} />
                  ) },
                  { id: 'overview.requests', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.requests'), render: () => (
                    <StatPanel title={t('dashboard.stat.requests')} unit="reqps" query={Q.s3Rate} tone={() => 'info'} sub={t('dashboard.stat.s3Sub')} />
                  ) },
                  { id: 'overview.errorRate', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.errorRate'), render: () => (
                    <StatPanel title={t('dashboard.stat.errorRate')} unit="percent" query={Q.errorPct} tone={errTone} sub={t('dashboard.stat.errorSub', { pct: thresholds.error_rate_warn_pct })} />
                  ) },
                  { id: 'overview.p95', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.p95'), render: () => (
                    <StatPanel title={t('dashboard.stat.p95')} unit="ms" query={Q.p95} tone={latTone} reduce="last" sub={t('dashboard.stat.latencySub', { ms: thresholds.latency_warn_ms })} />
                  ) },
                  { id: 'overview.write', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.write'), render: () => (
                    <StatPanel title={t('dashboard.stat.write')} unit="bytesPerSec" query={Q.ioRate} tone={() => 'info'} sub={t('dashboard.stat.writeSub')} />
                  ) },
                  { id: 'overview.read', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.read'), render: () => (
                    <StatPanel title={t('dashboard.stat.read')} unit="bytesPerSec" query={Q.ioRead} tone={() => 'info'} sub={t('dashboard.stat.readSub')} />
                  ) },
                  { id: 'overview.disk', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.disk'), render: () => (
                    <StatPanel title={t('dashboard.stat.disk')} unit="percent" query={Q.diskUsed} tone={diskTone} sub={t('dashboard.stat.diskSub', { warn: thresholds.disk_warn_pct, crit: thresholds.disk_crit_pct })} />
                  ) },
                  { id: 'overview.keys', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.keys'), render: () => (
                    <StatPanel title={t('dashboard.stat.keys')} unit="count" query={Q.keys} />
                  ) },
                  { id: 'overview.resync', w: 2, h: 84, compact: true, minH: 50, title: t('dashboard.stat.resync'), render: () => (
                    <StatPanel title={t('dashboard.stat.resync')} unit="count" query={Q.resync} tone={(v) => (v === null ? 'neutral' : v > 1000 ? 'warn' : 'ok')} sub={t('dashboard.stat.resyncSub')} />
                  ) },
            ] },
            { id: 's3', title: t('dashboard.row.s3'), panels: [
                  { id: 's3.requestsByEndpoint', w: 6, h: 230, title: t('dashboard.panel.requestsByEndpoint'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.requestsByEndpoint')} description={t('dashboard.panel.requestsByEndpoint.desc')} queries={P.requestsByEndpoint} unit="reqps" stacked legend="table" height={230} />
                  ) },
                  { id: 's3.latency', w: 6, h: 230, title: t('dashboard.panel.latency'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.latency')} description={t('dashboard.panel.latency.desc')} queries={P.latency} unit="ms" styles={QUANTILE_STYLES} thresholds={[{ value: thresholds.latency_warn_ms, color: 'var(--warning)' }]} fillOpacity={0.08} height={230} />
                  ) },
                  { id: 's3.errorsByStatus', w: 3, h: 170, title: t('dashboard.panel.errorsByStatus'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.errorsByStatus')} queries={P.errorsByStatus} unit="reqps" bars height={170} />
                  ) },
                  { id: 's3.errorRatio', w: 3, h: 170, title: t('dashboard.panel.errorRatio'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.errorRatio')} queries={P.errorRatio} unit="percent" labels={{ total: t('dashboard.series.allErrors'), '5xx': '5xx' }} styles={{ total: { color: 'var(--chart-orange)' }, '5xx': { color: 'var(--chart-red)' } }} thresholds={[{ value: thresholds.error_rate_warn_pct, color: 'var(--destructive)' }]} height={170} />
                  ) },
                  { id: 's3.latencyByEndpoint', w: 3, h: 170, title: t('dashboard.panel.latencyByEndpoint'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.latencyByEndpoint')} queries={P.latencyByEndpoint} unit="ms" fillOpacity={0} height={170} />
                  ) },
                  { id: 's3.errorsByEndpoint', w: 3, h: 170, title: t('dashboard.panel.errorsByEndpoint'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.errorsByEndpoint')} queries={P.errorsByEndpoint} unit="reqps" stacked height={170} />
                  ) },
                  { id: 's3.otherApis', w: 6, h: 160, title: t('dashboard.panel.otherApis'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.otherApis')} description={t('dashboard.panel.otherApis.desc')} queries={P.otherApis} unit="reqps" labels={{ web: t('dashboard.series.web'), k2v: 'K2V', admin: t('dashboard.series.admin') }} height={160} />
                  ) },
                  { id: 's3.webLatency', w: 6, h: 160, title: t('dashboard.panel.webLatency'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.webLatency')} queries={P.webLatency} unit="ms" styles={QUANTILE_STYLES} fillOpacity={0.06} height={160} />
                  ) },
            ] },
            { id: 'storage', title: t('dashboard.row.storage'), panels: [
                  { id: 'storage.storedData', w: 3, h: 180, title: t('dashboard.panel.storedData'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.storedData')} queries={P.storageBytes} unit="bytes" labels={{ bytes: t('dashboard.series.stored') }} styles={{ bytes: { color: 'var(--chart-blue)' } }} height={180} />
                  ) },
                  { id: 'storage.objectCount', w: 3, h: 180, title: t('dashboard.panel.objectCount'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.objectCount')} queries={P.storageObjects} unit="count" labels={{ objects: t('dashboard.series.objects') }} styles={{ objects: { color: 'var(--chart-purple)' } }} height={180} />
                  ) },
                  { id: 'storage.bucketGrowth', w: 3, h: 180, title: t('dashboard.panel.bucketGrowth'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.bucketGrowth')} queries={P.bucketBytes} unit="bytes" fillOpacity={0.04} height={180} />
                  ) },
                  { id: 'storage.bucketObjects', w: 3, h: 180, title: t('dashboard.panel.bucketObjects'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.bucketObjects')} queries={P.bucketObjects} unit="count" fillOpacity={0.04} height={180} />
                  ) },
                  { id: 'storage.distribution', w: 3, h: 220, title: t('dashboard.panel.distribution'), render: (size) => (
                    <PanelFrame title={t('dashboard.panel.distribution')} loading={metricsQ.isFetching}>
                      {donut.length === 0 ? (
                        <div className="flex items-center justify-center text-[0.75rem] text-[var(--muted-foreground)]" style={{ height: size.h }}>{t('charts.noData')}</div>
                      ) : (
                        <div className="relative" style={{ height: size.h }}>
                          <ResponsiveContainer width="100%" height="100%">
                            <PieChart>
                              <Pie data={donut} dataKey="value" nameKey="name" innerRadius="62%" outerRadius="88%" paddingAngle={1.5} stroke="none" isAnimationActive={false}>
                                {donut.map((d) => <Cell key={d.name} fill={d.color} />)}
                              </Pie>
                              <ReTooltip
                                contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 6, fontSize: 12 }}
                                itemStyle={{ color: 'var(--foreground)' }}
                                formatter={(v) => formatBytesValue(Number(v))}
                              />
                            </PieChart>
                          </ResponsiveContainer>
                          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                            <div className="text-[1.125rem] font-semibold tabular">{formatBytesValue(totalSize)}</div>
                            <div className="text-[0.6875rem] text-[var(--muted-foreground)]">{t('dashboard.stat.bucketsSub', { count: number.format(usage.length) })}</div>
                          </div>
                        </div>
                      )}
                    </PanelFrame>
                  ) },
                  { id: 'storage.topBuckets', w: 5, h: 220, title: t('dashboard.panel.topBuckets'), render: (size) => (
                    <PanelFrame title={t('dashboard.panel.topBuckets')} loading={metricsQ.isFetching}>
                      {sortedUsage.length === 0 ? (
                        <div className="flex items-center justify-center text-[0.75rem] text-[var(--muted-foreground)]" style={{ height: size.h }}>{t('charts.noData')}</div>
                      ) : (
                        <div className="overflow-y-auto pr-1 scrollbar-thin" style={{ maxHeight: size.h }}>
                          <BarGauge
                            items={sortedUsage.slice(0, 12).map((b, i) => ({
                              key: b.bucketName,
                              label: <Link to={`/buckets/${encodeURIComponent(b.bucketName)}/objects`} className="hover:text-[var(--primary)] hover:underline">{b.bucketName}</Link>,
                              value: b.size,
                              max: sortedUsage[0].size || 1,
                              detail: t(b.objectCount === 1 ? 'dashboard.one_object_count' : 'dashboard.objects_count', { count: number.format(b.objectCount) }),
                              tone: i === 0 ? 'info' : 'info',
                            }))}
                            format={(v) => formatBytesValue(v)}
                          />
                        </div>
                      )}
                    </PanelFrame>
                  ) },
                  { id: 'storage.multipart', w: 4, h: 200, title: t('dashboard.panel.multipart'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.multipart')} description={t('dashboard.panel.multipart.desc')} queries={P.multipart} unit="count" labels={{ mpu: t('dashboard.series.uploads') }} styles={{ mpu: { color: 'var(--chart-orange)' } }} height={200} />
                  ) },
            ] },
            { id: 'nodes', title: t('dashboard.row.nodes'), extra: <span className="text-[0.75rem] text-[var(--muted-foreground)]">{nodes.length > 0 && t('dashboard.nodesCount', { count: nodes.length })}</span>, panels: [
                  { id: 'nodes.diskUsage', w: 6, h: 170, title: t('dashboard.panel.diskUsage'), render: (size) => (
                    <PanelFrame title={t('dashboard.panel.diskUsage')} description={t('dashboard.panel.diskUsage.desc')} loading={nodesQ.isFetching}>
                      {nodes.filter((n) => n.dataPartition).length === 0 ? (
                        <div className="flex items-center justify-center text-[0.75rem] text-[var(--muted-foreground)]" style={{ height: size.h }}>{t('charts.noData')}</div>
                      ) : (
                        <div className="overflow-y-auto pr-1 scrollbar-thin" style={{ maxHeight: size.h }}>
                        <BarGauge
                          items={nodes
                            .filter((n) => n.dataPartition)
                            .map((n) => {
                              const used = n.dataPartition!.total - n.dataPartition!.available;
                              const pct = (used / n.dataPartition!.total) * 100;
                              return {
                                key: n.id,
                                label: (
                                  <span className="flex items-center gap-1.5">
                                    <span className={`h-1.5 w-1.5 rounded-full ${n.isUp ? 'bg-[var(--success)]' : 'bg-[var(--destructive)]'}`} />
                                    {n.hostname || n.id.slice(0, 12)}
                                    {n.role?.zone && <span className="text-[var(--muted-foreground)]">· {n.role.zone}</span>}
                                  </span>
                                ),
                                value: pct,
                                max: 100,
                                detail: `${formatBytesValue(used)} / ${formatBytesValue(n.dataPartition!.total)}`,
                                tone: diskTone(pct),
                              };
                            })}
                          format={(v) => formatValue(v, 'percent')}
                        />
                        </div>
                      )}
                    </PanelFrame>
                  ) },
                  { id: 'nodes.diskUsedTrend', w: 6, h: 170, title: t('dashboard.panel.diskUsedTrend'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.diskUsedTrend')} queries={P.diskUsedByNode} unit="percent" yMax={100} thresholds={[{ value: thresholds.disk_warn_pct, color: 'var(--warning)' }, { value: thresholds.disk_crit_pct, color: 'var(--destructive)' }]} fillOpacity={0.06} height={170} />
                  ) },
                  { id: 'nodes.dataAvail', w: 3, h: 160, title: t('dashboard.panel.dataAvail'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.dataAvail')} queries={P.dataAvail} unit="bytes" fillOpacity={0.06} height={160} />
                  ) },
                  { id: 'nodes.metaAvail', w: 3, h: 160, title: t('dashboard.panel.metaAvail'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.metaAvail')} queries={P.metaAvail} unit="bytes" fillOpacity={0.06} height={160} />
                  ) },
                  { id: 'nodes.nodes', w: 3, h: 160, title: t('dashboard.panel.nodes'), render: () => (
                    <TimeSeriesPanel
                      title={t('dashboard.panel.nodes')}
                      queries={P.nodes}
                      unit="count"
                      labels={{ known: t('dashboard.series.known'), connected: t('dashboard.series.connected'), storage: t('dashboard.series.storageNodes'), storage_up: t('dashboard.series.storageUp') }}
                      styles={{ known: { color: 'var(--chart-gray)', dashed: true }, connected: { color: 'var(--chart-blue)' }, storage: { color: 'var(--chart-gray)', dashed: true }, storage_up: { color: 'var(--chart-green)' } }}
                      fillOpacity={0}
                      height={160}
                    />
                  ) },
                  { id: 'nodes.partitions', w: 3, h: 160, title: t('dashboard.panel.partitions'), render: () => (
                    <TimeSeriesPanel
                      title={t('dashboard.panel.partitions')}
                      queries={P.partitions}
                      unit="count"
                      labels={{ total: t('dashboard.series.total'), quorum: t('dashboard.series.quorum'), ok: t('dashboard.series.allOk') }}
                      styles={{ total: { color: 'var(--chart-gray)', dashed: true }, quorum: { color: 'var(--chart-olive)' }, ok: { color: 'var(--chart-green)' } }}
                      fillOpacity={0}
                      height={160}
                    />
                  ) },
                  ...(nodes.length > 0
                    ? [{ id: 'nodes.nodeTable', w: 12, h: 220, minH: 80, title: t('dashboard.panel.nodeTable'), render: (size: PanelSize) => (
                    <PanelFrame title={t('dashboard.panel.nodeTable')}>
                      <div className="overflow-auto scrollbar-thin" style={{ maxHeight: size.h }}>
                        <table className="w-full min-w-[720px] text-[0.75rem]">
                          <thead>
                            <tr className="border-b border-[var(--border)] text-left text-[var(--muted-foreground)]">
                              <th className="px-2 py-1.5 font-normal">{t('dashboard.node.host')}</th>
                              <th className="px-2 py-1.5 font-normal">{t('dashboard.node.id')}</th>
                              <th className="px-2 py-1.5 font-normal">{t('dashboard.node.zone')}</th>
                              <th className="px-2 py-1.5 font-normal">{t('dashboard.node.address')}</th>
                              <th className="px-2 py-1.5 font-normal">{t('dashboard.node.version')}</th>
                              <th className="px-2 py-1.5 text-right font-normal">{t('dashboard.node.capacity')}</th>
                              <th className="px-2 py-1.5 text-right font-normal">{t('dashboard.node.data')}</th>
                              <th className="px-2 py-1.5 text-right font-normal">{t('dashboard.node.meta')}</th>
                              <th className="px-2 py-1.5 text-right font-normal">{t('dashboard.node.status')}</th>
                            </tr>
                          </thead>
                          <tbody>
                            {nodes.map((n) => {
                              const pct = (p?: { available: number; total: number }) => (p && p.total ? ((p.total - p.available) / p.total) * 100 : null);
                              const dp = pct(n.dataPartition);
                              const mp = pct(n.metadataPartition);
                              return (
                                <tr key={n.id} className="border-b border-[var(--border)] last:border-0 hover:bg-[var(--accent)]/50">
                                  <td className="px-2 py-1.5 font-medium">{n.hostname || '—'}</td>
                                  <td className="px-2 py-1.5 font-mono text-[var(--muted-foreground)]">{n.id.slice(0, 16)}</td>
                                  <td className="px-2 py-1.5">{n.role?.zone || '—'}</td>
                                  <td className="px-2 py-1.5 font-mono text-[var(--muted-foreground)]">{n.addr || '—'}</td>
                                  <td className="px-2 py-1.5">{n.garageVersion || '—'}</td>
                                  <td className="px-2 py-1.5 text-right tabular">{n.role?.capacity ? formatBytesValue(n.role.capacity) : t('dashboard.node.gateway')}</td>
                                  <td className="px-2 py-1.5 text-right tabular" style={{ color: dp === null ? undefined : `var(--${diskTone(dp) === 'ok' ? 'foreground' : diskTone(dp) === 'warn' ? 'warning' : 'destructive'})` }}>{formatValue(dp, 'percent')}</td>
                                  <td className="px-2 py-1.5 text-right tabular">{formatValue(mp, 'percent')}</td>
                                  <td className="px-2 py-1.5 text-right">
                                    <Badge variant={n.isUp ? (n.draining ? 'warning' : 'success') : 'danger'} className="px-1.5 py-0 text-[0.6875rem]">
                                      {n.isUp ? (n.draining ? t('dashboard.node.draining') : t('dashboard.node.up')) : t('dashboard.node.down')}
                                    </Badge>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </PanelFrame>
                  ) }]
                    : []),
            ] },
            { id: 'internals', title: t('dashboard.row.internals'), panels: [
                  { id: 'internals.blockIo', w: 4, h: 170, title: t('dashboard.panel.blockIo'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.blockIo')} description={t('dashboard.panel.blockIo.desc')} queries={P.blockIo} unit="bytesPerSec" labels={{ write: t('dashboard.series.write'), read: t('dashboard.series.read') }} styles={{ write: { color: 'var(--chart-orange)' }, read: { color: 'var(--chart-blue)' } }} height={170} />
                  ) },
                  { id: 'internals.blockOps', w: 4, h: 170, title: t('dashboard.panel.blockOps'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.blockOps')} queries={P.blockOps} unit="opsps" labels={{ writes: t('dashboard.series.write'), reads: t('dashboard.series.read') }} styles={{ writes: { color: 'var(--chart-orange)' }, reads: { color: 'var(--chart-blue)' } }} height={170} />
                  ) },
                  { id: 'internals.blockLatency', w: 4, h: 170, title: t('dashboard.panel.blockLatency'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.blockLatency')} queries={P.blockLatency} unit="ms" labels={{ write: t('dashboard.series.write'), read: t('dashboard.series.read') }} styles={{ write: { color: 'var(--chart-orange)' }, read: { color: 'var(--chart-blue)' } }} fillOpacity={0.05} height={170} />
                  ) },
                  { id: 'internals.resync', w: 4, h: 170, title: t('dashboard.panel.resync'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.resync')} description={t('dashboard.panel.resync.desc')} queries={P.resync} unit="count" labels={{ queue: t('dashboard.series.queue'), errored: t('dashboard.series.errored') }} styles={{ queue: { color: 'var(--chart-purple)' }, errored: { color: 'var(--chart-red)' } }} height={170} />
                  ) },
                  { id: 'internals.tableQueues', w: 4, h: 170, title: t('dashboard.panel.tableQueues'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.tableQueues')} description={t('dashboard.panel.tableQueues.desc')} queries={P.tableQueues} unit="count" labels={{ gc: 'GC', merkle: 'Merkle', insert: t('dashboard.series.insert') }} height={170} />
                  ) },
                  { id: 'internals.tableOps', w: 4, h: 170, title: t('dashboard.panel.tableOps'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.tableOps')} queries={P.tableOps} unit="opsps" labels={{ gets: 'get', puts: 'put', updates: t('dashboard.series.updates') }} stacked height={170} />
                  ) },
                  { id: 'internals.tableItems', w: 4, h: 170, title: t('dashboard.panel.tableItems'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.tableItems')} queries={P.tableItems} unit="count" fillOpacity={0} legend="table" height={170} />
                  ) },
                  { id: 'internals.rpc', w: 4, h: 170, title: t('dashboard.panel.rpc'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.rpc')} queries={P.rpc} unit="reqps" labels={{ rpc: t('dashboard.series.requests'), errors: t('dashboard.series.errors') }} styles={{ rpc: { color: 'var(--chart-cyan)' }, errors: { color: 'var(--chart-red)' } }} height={170} />
                  ) },
                  { id: 'internals.rpcLatency', w: 4, h: 170, title: t('dashboard.panel.rpcLatency'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.rpcLatency')} queries={P.rpcLatency} unit="ms" styles={QUANTILE_STYLES} fillOpacity={0.05} height={170} />
                  ) },
                  { id: 'internals.ramBuffer', w: 4, h: 170, title: t('dashboard.panel.ramBuffer'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.ramBuffer')} description={t('dashboard.panel.ramBuffer.desc')} queries={P.blockMemory} unit="bytes" labels={{ ram: t('dashboard.series.free') }} styles={{ ram: { color: 'var(--chart-green)' } }} height={170} />
                  ) },
                  { id: 'internals.collector', w: 4, h: 170, title: t('dashboard.panel.collector'), render: () => (
                    <TimeSeriesPanel title={t('dashboard.panel.collector')} description={t('dashboard.panel.collector.desc')} queries={P.collector} unit="ms" labels={{ scrape: t('dashboard.series.scrape') }} styles={{ scrape: { color: 'var(--chart-gray)' } }} height={170} />
                  ) },
            ] },
              ]}
            />
          </div>
        </div>
        </QueryBatchProvider>
      </DashboardContext.Provider>
    </DashboardTooltipProvider>
  );
}
