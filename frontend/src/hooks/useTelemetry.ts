import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { settingsApi, telemetryApi, type AppSettings, type MetricQuery } from '@/lib/telemetry';
import { useClusterStore } from '@/store/cluster-store';
import { useTranslation } from '@/lib/i18n';

export interface RelativeRange {
  key: string;
  seconds: number;
}

export const RELATIVE_RANGES: RelativeRange[] = [
  { key: '5m', seconds: 300 },
  { key: '15m', seconds: 900 },
  { key: '30m', seconds: 1800 },
  { key: '1h', seconds: 3600 },
  { key: '3h', seconds: 3 * 3600 },
  { key: '6h', seconds: 6 * 3600 },
  { key: '12h', seconds: 12 * 3600 },
  { key: '24h', seconds: 86400 },
  { key: '2d', seconds: 2 * 86400 },
  { key: '7d', seconds: 7 * 86400 },
  { key: '30d', seconds: 30 * 86400 },
  { key: '90d', seconds: 90 * 86400 },
];

export const REFRESH_OPTIONS = ['off', 'auto', '5s', '10s', '30s', '1m', '5m'] as const;

export type TimeRange = { kind: 'relative'; key: string } | { kind: 'absolute'; from: number; to: number };

export function rangeSeconds(key: string): number {
  return RELATIVE_RANGES.find((r) => r.key === key)?.seconds ?? 3600;
}

export function resolveRange(range: TimeRange): { from: number; to: number } {
  if (range.kind === 'absolute') return { from: range.from, to: range.to };
  const to = Math.floor(Date.now() / 1000);
  return { from: to - rangeSeconds(range.key), to };
}

export function refreshMs(refresh: string, intervalSeconds: number | undefined): number | false {
  if (refresh === 'off') return false;
  if (refresh === 'auto') return Math.max(5, intervalSeconds ?? 15) * 1000;
  const m = refresh.match(/^(\d+)(s|m)$/);
  if (!m) return false;
  return Number(m[1]) * (m[2] === 'm' ? 60_000 : 1000);
}

export const telemetryKeys = {
  all: ['telemetry'] as const,
  status: (clusterId: string | null) => ['telemetry', 'status', clusterId ?? 'none'] as const,
  range: (clusterId: string | null, queries: MetricQuery[], range: TimeRange) =>
    ['telemetry', 'range', clusterId ?? 'none', JSON.stringify(queries), range.kind === 'relative' ? range.key : `${range.from}-${range.to}`] as const,
};

export function useRangeQuery(queries: MetricQuery[], range: TimeRange, refetchInterval: number | false, enabled = true) {
  const clusterId = useClusterStore((s) => s.activeClusterId);
  return useQuery({
    queryKey: telemetryKeys.range(clusterId, queries, range),
    queryFn: ({ signal }) => telemetryApi.query({ ...resolveRange(range), queries }, signal),
    enabled: enabled && !!clusterId && queries.length > 0,
    refetchInterval: range.kind === 'relative' ? refetchInterval : false,
    refetchIntervalInBackground: false,
    staleTime: 2000,
    refetchOnMount: true,
    placeholderData: keepPreviousData,
    retry: false,
  });
}

export function useTelemetryStatus(refetchInterval: number | false = 15000) {
  const clusterId = useClusterStore((s) => s.activeClusterId);
  return useQuery({
    queryKey: telemetryKeys.status(clusterId),
    queryFn: () => telemetryApi.status(),
    enabled: !!clusterId,
    refetchInterval,
    staleTime: 5000,
    refetchOnMount: true,
    retry: false,
  });
}

export const settingsKey = ['app-settings'] as const;

export function useAppSettings(enabled = true) {
  return useQuery({
    queryKey: settingsKey,
    queryFn: () => settingsApi.get(),
    enabled,
    staleTime: 60_000,
    retry: false,
  });
}

export function useUpdateAppSettings() {
  const qc = useQueryClient();
  const { t } = useTranslation();
  return useMutation({
    mutationFn: (s: AppSettings) => settingsApi.update(s),
    onSuccess: (data) => {
      qc.setQueryData(settingsKey, data);
      qc.invalidateQueries({ queryKey: telemetryKeys.all });
      toast.success(t('settings.saved'));
    },
  });
}

export function usePurgeHistory() {
  const qc = useQueryClient();
  const { t } = useTranslation();
  return useMutation({
    mutationFn: (scope: 'cluster' | 'all') => telemetryApi.purge(scope),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: telemetryKeys.all });
      toast.success(t('settings.monitoring.purged'));
    },
  });
}

export function useScrapeNow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => telemetryApi.scrape(),
    onSuccess: () => qc.invalidateQueries({ queryKey: telemetryKeys.all }),
  });
}
