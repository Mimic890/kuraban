import api from '@/lib/api';

export type QueryFn = 'avg' | 'max' | 'min' | 'last' | 'rate' | 'increase' | 'ratio';

export interface MetricQuery {
  id: string;
  metric: string;
  fn?: QueryFn;
  denominator?: string;
  match?: Record<string, string>;
  group_by?: string;
  agg?: 'sum' | 'avg' | 'max' | 'min';
  scale?: number;
  top_k?: number;
}

export interface RangeSeries {
  query: string;
  name: string;
  values: (number | null)[];
}

export interface RangeResult {
  from: number;
  to: number;
  step: number;
  tier: string;
  timestamps: number[];
  series: RangeSeries[];
}

export interface ScrapeStatus {
  last_scrape?: string;
  last_success?: string;
  last_bucket_scrape?: string;
  last_error?: string;
  last_duration_ms: number;
  samples: number;
  metrics_available: boolean;
}

export interface StorageStats {
  size_bytes: number;
  series: number;
  raw_samples: number;
  oldest_ts: number | null;
  newest_ts: number | null;
}

export interface MonitoringSettings {
  enabled: boolean;
  interval_seconds: number;
  retention_hours: number;
  bucket_stats: boolean;
  bucket_interval_seconds: number;
}

export interface TelemetryStatus {
  monitoring: MonitoringSettings;
  collector?: ScrapeStatus;
  storage: StorageStats;
  cluster: StorageStats;
}

export interface Thresholds {
  disk_warn_pct: number;
  disk_crit_pct: number;
  error_rate_warn_pct: number;
  latency_warn_ms: number;
}

export interface Overrides {
  log_level?: string | null;
  allowed_ips?: string[] | null;
  cluster_endpoint_allowlist?: string[] | null;
}

export interface AppSettings {
  monitoring: MonitoringSettings;
  thresholds: Thresholds;
  overrides: Overrides;
  updated_at?: string;
}

export interface SettingsResponse {
  settings: AppSettings;
  env: { log_level: string; allowed_ips: string[] | null; cluster_endpoint_allowlist: string[] | null };
  effective: { log_level: string; allowed_ips: string[]; cluster_endpoint_allowlist: string[] };
  options: { intervals: number[]; log_levels: string[]; min_retention_hours: number; max_retention_hours: number };
  server: {
    version: string;
    environment: string;
    host: string;
    port: number;
    root_url: string;
    log_format: string;
    max_body_size: number;
    max_header_size: number;
    read_buffer_size: number;
    write_buffer_size: number;
    oidc_enabled: boolean;
    token_enabled: boolean;
    metrics_public: boolean;
    access_control: boolean;
    client_ip: string;
  };
}

export const telemetryApi = {
  query: async (req: { from: number; to: number; step?: number; queries: MetricQuery[] }, signal?: AbortSignal): Promise<RangeResult> => {
    const response = await api.get('/v1/telemetry/query', { params: { q: JSON.stringify(req) }, signal });
    return response.data.data;
  },
  status: async (): Promise<TelemetryStatus> => {
    const response = await api.get('/v1/telemetry/status');
    return response.data.data;
  },
  scrape: async (): Promise<void> => {
    await api.post('/v1/telemetry/scrape');
  },
  purge: async (scope: 'cluster' | 'all'): Promise<void> => {
    await api.delete('/v1/telemetry/history', { params: scope === 'all' ? { scope: 'all' } : {} });
  },
};

export const settingsApi = {
  get: async (): Promise<SettingsResponse> => {
    const response = await api.get('/v1/settings');
    return response.data.data;
  },
  update: async (settings: AppSettings): Promise<SettingsResponse> => {
    const response = await api.put('/v1/settings', settings);
    return response.data.data;
  },
};
