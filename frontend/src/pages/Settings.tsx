import * as React from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  Activity,
  AlertTriangle,
  Check,
  Database,
  Gauge,
  Globe,
  Info,
  Palette,
  RotateCcw,
  Shield,
  Trash2,
  User,
  Zap,
} from 'lucide-react';
import { PageHeader } from '@/components/ui/page-header';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Select, SelectOption } from '@/components/ui/select';
import { Segmented } from '@/components/ui/segmented';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { useTheme, type Palette as PaletteId } from '@/components/theme-provider';
import { PALETTES } from '@/components/layout/theme-toggle';
import { FONTS } from '@/lib/fonts';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import { useSettingsStore, type FontSize, type Language } from '@/store/settings-store';
import { useClusterStore } from '@/store/cluster-store';
import { usePermissions } from '@/hooks/usePermissions';
import {
  RELATIVE_RANGES,
  REFRESH_OPTIONS,
  useAppSettings,
  usePurgeHistory,
  useScrapeNow,
  useTelemetryStatus,
  useUpdateAppSettings,
} from '@/hooks/useTelemetry';
import type { AppSettings } from '@/lib/telemetry';
import { formatBytesValue, formatRelative, formatValue } from '@/lib/units';
import { SETTINGS_SECTIONS } from '@/components/layout/nav';

type Section = (typeof SETTINGS_SECTIONS)[number];

const SECTION_ICONS: Record<Section, React.ComponentType<{ className?: string }>> = {
  appearance: Palette,
  region: Globe,
  dashboard: Gauge,
  monitoring: Activity,
  security: Shield,
  about: Info,
};

const SERVER_SECTIONS: Section[] = ['dashboard', 'monitoring', 'security'];

// ─── Building blocks ────────────────────────────────────────────────────────

type ScopeKind = 'server' | 'browser' | 'env';

function Scope({ scope }: { scope: ScopeKind }) {
  const { t } = useTranslation();
  return (
    <span
      className={cn(
        'rounded border px-1.5 py-px text-[0.625rem] font-medium uppercase tracking-wider',
        scope === 'server' ? 'border-[var(--accent-primary-border)] text-[var(--primary)]' : 'border-[var(--border)] text-[var(--muted-foreground)]',
      )}
      title={t(`settings.scope.${scope}Hint`)}
    >
      {t(`settings.scope.${scope}`)}
    </span>
  );
}

function Group({ title, description, server, scope, children, actions }: { title: string; description?: string; server?: boolean; scope?: ScopeKind; children: React.ReactNode; actions?: React.ReactNode }) {
  const kind: ScopeKind | undefined = server ? 'server' : scope;
  return (
    <section className="rounded-lg border border-[var(--border)] bg-[var(--card)]">
      <header className="flex items-start gap-3 border-b border-[var(--border)] px-4 py-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="text-[0.875rem] font-semibold">{title}</h2>
            {kind && <Scope scope={kind} />}
          </div>
          {description && <p className="mt-0.5 text-[0.75rem] text-[var(--muted-foreground)]">{description}</p>}
        </div>
        {actions}
      </header>
      <div className="divide-y divide-[var(--border)]">{children}</div>
    </section>
  );
}

function Row({ label, description, children, stacked, badge }: { label: React.ReactNode; description?: React.ReactNode; children: React.ReactNode; stacked?: boolean; badge?: React.ReactNode }) {
  return (
    <div className={cn('flex gap-3 px-4 py-3.5', stacked ? 'flex-col' : 'flex-col sm:flex-row sm:items-center sm:justify-between')}>
      <div className="min-w-0 sm:max-w-[55%]">
        <div className="flex items-center gap-2 text-[0.8125rem] font-medium">
          {label}
          {badge}
        </div>
        {description && <div className="mt-0.5 text-[0.75rem] leading-relaxed text-[var(--muted-foreground)]">{description}</div>}
      </div>
      <div className={cn('min-w-0', !stacked && 'sm:shrink-0')}>{children}</div>
    </div>
  );
}

function KV({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="min-w-0 rounded-md border border-[var(--border)] bg-[var(--background)] px-3 py-2">
      <div className="text-[0.6875rem] text-[var(--muted-foreground)]">{label}</div>
      <div className={cn('mt-0.5 truncate text-[0.8125rem]', mono && 'font-mono')}>{value}</div>
    </div>
  );
}

function ListEditor({ value, onChange, placeholder, disabled }: { value: string[]; onChange: (v: string[]) => void; placeholder: string; disabled?: boolean }) {
  const [text, setText] = React.useState(value.join('\n'));
  const joined = value.join('\n');
  React.useEffect(() => setText(joined), [joined]);
  return (
    <textarea
      value={text}
      disabled={disabled}
      onChange={(e) => {
        setText(e.target.value);
        onChange(e.target.value.split(/[\n,]/).map((s) => s.trim()).filter(Boolean));
      }}
      rows={Math.min(8, Math.max(3, value.length + 1))}
      placeholder={placeholder}
      spellCheck={false}
      className="w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-3 py-2 font-mono text-[0.75rem] outline-none focus:border-[var(--ring)] disabled:opacity-60"
    />
  );
}

// ─── Sections ───────────────────────────────────────────────────────────────

function AppearanceSection() {
  const { t } = useTranslation();
  const { palette, mode, setPalette, setMode } = useTheme();
  const s = useSettingsStore();
  const sizes: FontSize[] = ['xs', 'sm', 'md', 'lg', 'xl'];
  const uiFonts = FONTS;
  const monoFonts = FONTS.filter((f) => f.monospace);

  return (
    <div className="space-y-4">
      <Group scope="browser" title={t('settings.appearance.theme')} description={t('settings.appearance.themeDesc')}>
        <Row label={t('settings.appearance.mode')}>
          <Segmented value={mode} onChange={setMode} options={[{ value: 'dark', label: t('theme.mode.dark') }, { value: 'light', label: t('theme.mode.light') }]} />
        </Row>
        <Row label={t('settings.appearance.palette')} stacked>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {PALETTES.map((p) => {
              const c = p.colors[p.fixedMode ?? mode];
              const active = palette === p.id;
              return (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => setPalette(p.id as PaletteId)}
                  className={cn(
                    'group overflow-hidden rounded-md border text-left transition-colors',
                    active ? 'border-[var(--primary)] ring-1 ring-[var(--primary)]' : 'border-[var(--border)] hover:border-[var(--input)]',
                  )}
                >
                  <div className="flex h-12 items-end gap-1 p-2" style={{ background: c.bg }}>
                    <span className="h-5 w-8 rounded-sm" style={{ background: c.primary }} />
                    <span className="h-3 w-5 rounded-sm" style={{ background: c.secondary }} />
                    <span className="ml-auto h-1.5 w-8 rounded-sm" style={{ background: c.text, opacity: 0.8 }} />
                  </div>
                  <div className="flex items-center justify-between px-2 py-1.5 text-[0.75rem]">
                    <span className={active ? 'font-medium' : 'text-[var(--muted-foreground)]'}>{t(`theme.palettes.${p.id}`)}</span>
                    {active && <Check className="h-3.5 w-3.5 text-[var(--primary)]" />}
                  </div>
                </button>
              );
            })}
          </div>
        </Row>
      </Group>

      <Group
        scope="browser"
        title={t('settings.appearance.typography')}
        description={t('settings.appearance.typographyDesc')}
        actions={
          <Button variant="ghost" size="sm" onClick={s.resetAppearance}>
            <RotateCcw /> {t('settings.reset')}
          </Button>
        }
      >
        <Row label={t('settings.appearance.uiFont')} description={t('settings.appearance.uiFontDesc')}>
          <div className="w-full sm:w-64">
            <Select value={s.fontSans} onChange={s.setFontSans}>
              {uiFonts.map((f) => (
                <SelectOption key={f.id} value={f.id}>{f.label}</SelectOption>
              ))}
            </Select>
          </div>
        </Row>
        <Row label={t('settings.appearance.monoFont')} description={t('settings.appearance.monoFontDesc')}>
          <div className="w-full sm:w-64">
            <Select value={s.fontMono} onChange={s.setFontMono}>
              {monoFonts.map((f) => (
                <SelectOption key={f.id} value={f.id}>{f.label}</SelectOption>
              ))}
            </Select>
          </div>
        </Row>
        <Row label={t('settings.appearance.fontSize')} description={t('settings.appearance.fontSizeDesc')}>
          <Segmented value={s.fontSize} onChange={s.setFontSize} options={sizes.map((v) => ({ value: v, label: v.toUpperCase() }))} />
        </Row>
        <Row label={t('settings.appearance.preview')} stacked>
          <div className="rounded-md border border-[var(--border)] bg-[var(--background)] p-3">
            <div className="text-[1.125rem] font-semibold">{t('settings.appearance.previewTitle')}</div>
            <p className="mt-1 text-[0.8125rem] text-[var(--muted-foreground)]">{t('settings.appearance.previewText')}</p>
            <code className="mt-2 block text-[0.75rem]">s3://photos/2026/09/IMG_0042.jpg · 4.2 MiB · 0123456789 {'{ } => != ->'}</code>
          </div>
        </Row>
      </Group>

      <Group scope="browser" title={t('settings.appearance.layout')}>
        <Row label={t('settings.appearance.density')} description={t('settings.appearance.densityDesc')}>
          <Segmented value={s.density} onChange={s.setDensity} options={[{ value: 'comfortable', label: t('settings.appearance.comfortable') }, { value: 'compact', label: t('settings.appearance.compact') }]} />
        </Row>
        <Row label={t('settings.appearance.sidebar')} description={t('settings.appearance.sidebarDesc')}>
          <Switch checked={s.sidebarCollapsed} onCheckedChange={s.setSidebarCollapsed} />
        </Row>
        <Row label={t('settings.appearance.motion')} description={t('settings.appearance.motionDesc')}>
          <Switch checked={s.reduceMotion} onCheckedChange={s.setReduceMotion} />
        </Row>
      </Group>
    </div>
  );
}

function RegionSection() {
  const { t, language } = useTranslation();
  const { timezone, setTimezone, setLanguage, hour12, setHour12 } = useSettingsStore();
  const timezones = React.useMemo(() => {
    try {
      return Intl.supportedValuesOf('timeZone')
        .map((tz) => {
          const parts = new Intl.DateTimeFormat(language, { timeZone: tz, timeZoneName: 'shortOffset' }).formatToParts(new Date());
          const offset = (parts.find((p) => p.type === 'timeZoneName')?.value || 'UTC').replace('GMT', 'UTC');
          return { value: tz, label: `(${offset}) ${tz.replace(/_/g, ' ')}` };
        })
        .sort((a, b) => a.label.localeCompare(b.label));
    } catch {
      return [{ value: timezone, label: timezone }];
    }
  }, [language, timezone]);
  const sample = new Intl.DateTimeFormat(language, { timeZone: timezone, dateStyle: 'medium', timeStyle: 'medium', hour12 }).format(new Date());

  return (
    <Group scope="browser" title={t('settings.section.region')} description={t('settings.region.desc')}>
      <Row label={t('settings.language')} description={t('settings.language.desc')}>
        <Segmented value={language} onChange={(v) => setLanguage(v as Language)} options={[{ value: 'en', label: 'English' }, { value: 'ru', label: 'Русский' }]} />
      </Row>
      <Row label={t('settings.timezone')} description={t('settings.timezone.desc')}>
        <div className="w-full sm:w-72">
          <Select value={timezone} onChange={setTimezone}>
            {timezones.map((tz) => (
              <SelectOption key={tz.value} value={tz.value}>{tz.label}</SelectOption>
            ))}
          </Select>
        </div>
      </Row>
      <Row label={t('settings.region.clock')} description={sample}>
        <Segmented value={hour12 ? '12' : '24'} onChange={(v) => setHour12(v === '12')} options={[{ value: '24', label: '24h' }, { value: '12', label: '12h' }]} />
      </Row>
    </Group>
  );
}

interface ServerProps {
  draft: AppSettings;
  setDraft: (fn: (d: AppSettings) => AppSettings) => void;
  canEdit: boolean;
}

function DashboardSection({ draft, setDraft, canEdit }: ServerProps) {
  const { t } = useTranslation();
  const { dashboardRange, setDashboardRange, dashboardRefresh, setDashboardRefresh } = useSettingsStore();
  const th = draft.thresholds;
  const setTh = (k: keyof AppSettings['thresholds'], v: number) => setDraft((d) => ({ ...d, thresholds: { ...d.thresholds, [k]: v } }));
  const num = (k: keyof AppSettings['thresholds'], suffix: string, max: number) => (
    <div className="flex items-center gap-2">
      <input
        type="number"
        min={0}
        max={max}
        step="any"
        disabled={!canEdit}
        value={th[k]}
        onChange={(e) => setTh(k, Number(e.target.value))}
        className="h-8 w-24 rounded-md border border-[var(--border)] bg-[var(--background)] px-2 text-right text-[0.8125rem] tabular outline-none focus:border-[var(--ring)] disabled:opacity-60"
      />
      <span className="w-6 text-[0.75rem] text-[var(--muted-foreground)]">{suffix}</span>
    </div>
  );
  return (
    <div className="space-y-4">
      <Group scope="browser" title={t('settings.dashboard.defaults')} description={t('settings.dashboard.defaultsDesc')}>
        <Row label={t('settings.dashboard.range')}>
          <div className="w-full sm:w-56">
            <Select value={dashboardRange} onChange={setDashboardRange}>
              {RELATIVE_RANGES.map((r) => (
                <SelectOption key={r.key} value={r.key}>{t(`time.last.${r.key}`)}</SelectOption>
              ))}
            </Select>
          </div>
        </Row>
        <Row label={t('settings.dashboard.refresh')} description={t('settings.dashboard.refreshDesc')}>
          <Segmented size="sm" value={dashboardRefresh} onChange={setDashboardRefresh} options={REFRESH_OPTIONS.map((o) => ({ value: o, label: o === 'off' ? t('time.refresh.off') : o === 'auto' ? t('settings.dashboard.auto') : o }))} />
        </Row>
      </Group>
      <Group server title={t('settings.dashboard.thresholds')} description={t('settings.dashboard.thresholdsDesc')}>
        <Row label={t('settings.dashboard.diskWarn')} description={t('settings.dashboard.diskWarnDesc')}>{num('disk_warn_pct', '%', 100)}</Row>
        <Row label={t('settings.dashboard.diskCrit')}>{num('disk_crit_pct', '%', 100)}</Row>
        <Row label={t('settings.dashboard.errorRate')} description={t('settings.dashboard.errorRateDesc')}>{num('error_rate_warn_pct', '%', 100)}</Row>
        <Row label={t('settings.dashboard.latency')} description={t('settings.dashboard.latencyDesc')}>{num('latency_warn_ms', 'ms', 600000)}</Row>
      </Group>
    </div>
  );
}

const RETENTION_PRESETS = [24, 72, 168, 336, 720, 2160, 4320, 8760];
const BUCKET_INTERVALS = [30, 60, 300, 900, 3600];

function humanHours(h: number, t: (k: string, v?: Record<string, string | number>) => string) {
  if (h % 24 === 0) return t('settings.monitoring.days', { count: h / 24 });
  return t('settings.monitoring.hours', { count: h });
}

function humanSeconds(s: number) {
  return s >= 3600 ? `${s / 3600}h` : s >= 60 ? `${s / 60}m` : `${s}s`;
}

function MonitoringSection({ draft, setDraft, canEdit, options }: ServerProps & { options: { intervals: number[] } }) {
  const { t } = useTranslation();
  const status = useTelemetryStatus(10_000);
  const purge = usePurgeHistory();
  const scrape = useScrapeNow();
  const [confirm, setConfirm] = React.useState<null | 'cluster' | 'all'>(null);
  const m = draft.monitoring;
  const setM = (patch: Partial<AppSettings['monitoring']>) => setDraft((d) => ({ ...d, monitoring: { ...d.monitoring, ...patch } }));
  const st = status.data;
  const c = st?.collector;
  const isPreset = RETENTION_PRESETS.includes(m.retention_hours);

  const samplesPerDay = (86400 / m.interval_seconds) * Math.max(st?.cluster.series ?? 100, 1);

  return (
    <div className="space-y-4">
      <Group server title={t('settings.monitoring.collection')} description={t('settings.monitoring.collectionDesc')}>
        <Row label={t('settings.monitoring.enabled')} description={t('settings.monitoring.enabledDesc')}>
          <Switch checked={m.enabled} disabled={!canEdit} onCheckedChange={(v) => setM({ enabled: v })} />
        </Row>
        <Row label={t('settings.monitoring.interval')} description={t('settings.monitoring.intervalDesc', { samples: formatValue(samplesPerDay, 'short') })}>
          <Segmented disabled={!canEdit || !m.enabled} value={m.interval_seconds} onChange={(v) => setM({ interval_seconds: v })} options={options.intervals.map((v) => ({ value: v, label: humanSeconds(v) }))} />
        </Row>
        <Row label={t('settings.monitoring.retention')} description={t('settings.monitoring.retentionDesc')} stacked>
          <div className="flex flex-wrap items-center gap-2">
            <Segmented
              size="sm"
              disabled={!canEdit}
              value={isPreset ? m.retention_hours : -1}
              onChange={(v) => v > 0 && setM({ retention_hours: v })}
              options={[...RETENTION_PRESETS.map((h) => ({ value: h, label: humanHours(h, t) })), { value: -1, label: t('settings.monitoring.custom') }]}
            />
            <div className="flex items-center gap-1.5">
              <input
                type="number"
                min={1}
                max={365}
                disabled={!canEdit}
                value={Math.round((m.retention_hours / 24) * 10) / 10}
                onChange={(e) => setM({ retention_hours: Math.max(1, Math.round(Number(e.target.value) * 24)) })}
                className="h-7 w-20 rounded-md border border-[var(--border)] bg-[var(--background)] px-2 text-right text-[0.75rem] tabular outline-none focus:border-[var(--ring)] disabled:opacity-60"
              />
              <span className="text-[0.75rem] text-[var(--muted-foreground)]">{t('settings.monitoring.daysUnit')}</span>
            </div>
          </div>
          <p className="text-[0.6875rem] text-[var(--muted-foreground)]">{t('settings.monitoring.tiers')}</p>
        </Row>
        <Row label={t('settings.monitoring.bucketStats')} description={t('settings.monitoring.bucketStatsDesc')}>
          <div className="flex flex-wrap items-center gap-2">
            <Switch checked={m.bucket_stats} disabled={!canEdit} onCheckedChange={(v) => setM({ bucket_stats: v })} />
            <Segmented size="sm" disabled={!canEdit || !m.bucket_stats} value={m.bucket_interval_seconds} onChange={(v) => setM({ bucket_interval_seconds: v })} options={BUCKET_INTERVALS.map((v) => ({ value: v, label: humanSeconds(v) }))} />
          </div>
        </Row>
      </Group>

      <Group
        title={t('settings.monitoring.status')}
        description={t('settings.monitoring.statusDesc')}
        server
        actions={
          canEdit && (
            <Button variant="secondary" size="sm" onClick={() => scrape.mutate()} disabled={scrape.isPending}>
              <Zap /> {t('settings.monitoring.scrapeNow')}
            </Button>
          )
        }
      >
        <div className="grid grid-cols-2 gap-2 p-4 md:grid-cols-3">
          <KV label={t('settings.monitoring.lastScrape')} value={c?.last_scrape ? formatRelative(c.last_scrape) : '—'} />
          <KV label={t('settings.monitoring.duration')} value={c ? formatValue(c.last_duration_ms, 'ms') : '—'} />
          <KV label={t('settings.monitoring.metricsEndpoint')} value={c ? (c.metrics_available ? <span className="text-[var(--success)]">{t('settings.monitoring.available')}</span> : <span className="text-[var(--destructive)]">{t('settings.monitoring.unavailable')}</span>) : '—'} />
          <KV label={t('settings.monitoring.dbSize')} value={st ? formatBytesValue(st.storage.size_bytes) : '—'} />
          <KV label={t('settings.monitoring.series')} value={st ? formatValue(st.cluster.series, 'count') : '—'} />
          <KV label={t('settings.monitoring.rawSamples')} value={st ? formatValue(st.cluster.raw_samples, 'count') : '—'} />
          <KV label={t('settings.monitoring.oldest')} value={st?.cluster.oldest_ts ? formatRelative(st.cluster.oldest_ts) : '—'} />
          <KV label={t('settings.monitoring.bucketScrape')} value={c?.last_bucket_scrape ? formatRelative(c.last_bucket_scrape) : '—'} />
          <KV label={t('settings.monitoring.samplesPerScrape')} value={c ? formatValue(c.samples, 'count') : '—'} />
        </div>
        {c?.last_error && (
          <div className="flex gap-2 px-4 py-3 text-[0.75rem]">
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--warning)]" />
            <span className="break-all font-mono text-[var(--muted-foreground)]">{c.last_error}</span>
          </div>
        )}
      </Group>

      {canEdit && (
        <Group title={t('settings.monitoring.danger')} description={t('settings.monitoring.dangerDesc')} server>
          <Row label={t('settings.monitoring.purgeCluster')} description={t('settings.monitoring.purgeClusterDesc')}>
            <Button variant="secondary" size="sm" onClick={() => setConfirm('cluster')}>
              <Trash2 /> {t('settings.monitoring.purge')}
            </Button>
          </Row>
          <Row label={t('settings.monitoring.purgeAll')} description={t('settings.monitoring.purgeAllDesc')}>
            <Button variant="destructive" size="sm" onClick={() => setConfirm('all')}>
              <Trash2 /> {t('settings.monitoring.purgeAllBtn')}
            </Button>
          </Row>
        </Group>
      )}
      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={confirm === 'all' ? t('settings.monitoring.purgeAll') : t('settings.monitoring.purgeCluster')}
        description={t('settings.monitoring.purgeConfirm')}
        confirmLabel={t('settings.monitoring.purge')}
        tone="destructive"
        loading={purge.isPending}
        onConfirm={async () => {
          if (confirm) await purge.mutateAsync(confirm);
          setConfirm(null);
        }}
      />
    </div>
  );
}

function OverrideList({
  label,
  description,
  value,
  envValue,
  onChange,
  canEdit,
  placeholder,
  extra,
}: {
  label: string;
  description: string;
  value: string[] | null | undefined;
  envValue: string[] | null;
  onChange: (v: string[] | null) => void;
  canEdit: boolean;
  placeholder: string;
  extra?: React.ReactNode;
}) {
  const { t } = useTranslation();
  const inherited = value === null || value === undefined;
  return (
    <Row label={label} description={description} stacked badge={inherited ? <Badge className="px-1.5 py-0 text-[0.625rem]">{t('settings.security.fromEnv')}</Badge> : <Badge variant="warning" className="px-1.5 py-0 text-[0.625rem]">{t('settings.security.overridden')}</Badge>}>
      <div className="space-y-2">
        <div className="flex items-center gap-2 text-[0.75rem]">
          <Switch checked={!inherited} disabled={!canEdit} onCheckedChange={(on) => onChange(on ? [...(envValue ?? [])] : null)} />
          <span className="text-[var(--muted-foreground)]">{t('settings.security.override')}</span>
        </div>
        {inherited ? (
          <div className="rounded-md border border-dashed border-[var(--border)] px-3 py-2 font-mono text-[0.75rem] text-[var(--muted-foreground)]">
            {envValue && envValue.length > 0 ? envValue.join(', ') : t('settings.security.empty')}
          </div>
        ) : (
          <ListEditor value={value} onChange={onChange} placeholder={placeholder} disabled={!canEdit} />
        )}
        {extra}
      </div>
    </Row>
  );
}

function SecuritySection({ draft, setDraft, canEdit, info }: ServerProps & { info: NonNullable<ReturnType<typeof useAppSettings>['data']> }) {
  const { t } = useTranslation();
  const o = draft.overrides;
  const setO = (patch: Partial<AppSettings['overrides']>) => setDraft((d) => ({ ...d, overrides: { ...d.overrides, ...patch } }));
  const level = o.log_level ?? null;
  const srv = info.server;
  return (
    <div className="space-y-4">
      <Group server title={t('settings.security.access')} description={t('settings.security.accessDesc')}>
        <OverrideList
          label={t('settings.security.allowedIps')}
          description={t('settings.security.allowedIpsDesc')}
          value={o.allowed_ips}
          envValue={info.env.allowed_ips}
          onChange={(v) => setO({ allowed_ips: v })}
          canEdit={canEdit}
          placeholder={'10.0.0.0/8\n192.168.1.20'}
          extra={<p className="text-[0.6875rem] text-[var(--muted-foreground)]">{t('settings.security.yourIp', { ip: srv.client_ip })}</p>}
        />
        <OverrideList
          label={t('settings.security.endpointAllowlist')}
          description={t('settings.security.endpointAllowlistDesc')}
          value={o.cluster_endpoint_allowlist}
          envValue={info.env.cluster_endpoint_allowlist}
          onChange={(v) => setO({ cluster_endpoint_allowlist: v })}
          canEdit={canEdit}
          placeholder={'10.0.0.0/8\n172.16.0.0/12'}
        />
      </Group>
      <Group server title={t('settings.logging.title')} description={t('settings.security.loggingDesc')}>
        <Row label={t('settings.logging.level')} description={level === null ? t('settings.security.levelInherited', { level: info.env.log_level }) : t('settings.security.levelOverridden')}>
          <div className="flex flex-wrap items-center gap-2">
            <Segmented
              size="sm"
              disabled={!canEdit}
              value={level ?? '__env'}
              onChange={(v) => setO({ log_level: v === '__env' ? null : v })}
              options={[{ value: '__env', label: `env (${info.env.log_level})` }, ...info.options.log_levels.map((l) => ({ value: l, label: l }))]}
            />
          </div>
        </Row>
      </Group>
      <Group scope="env" title={t('settings.security.deployment')} description={t('settings.security.deploymentDesc')}>
        <div className="grid grid-cols-1 gap-2 p-4 sm:grid-cols-2 lg:grid-cols-3">
          <KV label={t('settings.network.bindingHost')} value={srv.host || '::'} mono />
          <KV label={t('settings.network.port')} value={srv.port} mono />
          <KV label={t('settings.network.rootUrl')} value={srv.root_url || window.location.origin} mono />
          <KV label={t('settings.security.environment')} value={srv.environment} mono />
          <KV label={t('settings.limits.maxBodySize')} value={srv.max_body_size ? formatBytesValue(srv.max_body_size) : '64 MiB'} mono />
          <KV label={t('settings.logging.format')} value={srv.log_format || 'text'} mono />
          <KV label={t('settings.security.authMethods')} value={[t('settings.security.local'), srv.oidc_enabled && 'OIDC', srv.token_enabled && t('settings.security.token')].filter(Boolean).join(' · ')} />
          <KV label={t('settings.security.accessControl')} value={srv.access_control ? t('settings.security.on') : t('settings.security.off')} />
          <KV label={t('settings.security.publicMetrics')} value={srv.metrics_public ? t('settings.security.on') : t('settings.security.off')} />
        </div>
      </Group>
    </div>
  );
}

function AboutSection({ version }: { version?: string }) {
  const { t } = useTranslation();
  const shortcuts: [string, string][] = [
    ['Ctrl K', t('settings.about.kPalette')],
    ['/', t('settings.about.kSearch')],
    ['G D', t('nav.dashboard')],
    ['G B', t('nav.buckets')],
    ['G K', t('nav.access')],
    ['G C', t('nav.status')],
    ['G S', t('nav.settings')],
    [t('settings.about.drag'), t('settings.about.kZoom')],
    [t('settings.about.click'), t('settings.about.kLegend')],
  ];
  return (
    <div className="space-y-4">
      <Group title="Kuraban" description={t('settings.about.desc')}>
        <div className="grid grid-cols-1 gap-2 p-4 sm:grid-cols-3">
          <KV label={t('settings.about.version')} value={version ?? '—'} mono />
          <KV label={t('settings.about.storage')} value="SQLite · data/kuraban.db" mono />
          <KV label={t('settings.about.charts')} value="uPlot · Recharts" mono />
        </div>
        <Row label={t('settings.user')} description={t('settings.about.accountDesc')}>
          <Link to="/user-settings" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--border)] px-3 text-[0.8125rem] hover:bg-[var(--accent)]">
            <User className="h-3.5 w-3.5" /> {t('settings.about.openAccount')}
          </Link>
        </Row>
        <Row label={t('nav.connections')} description={t('settings.about.clustersDesc')}>
          <Link to="/connections" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[var(--border)] px-3 text-[0.8125rem] hover:bg-[var(--accent)]">
            <Database className="h-3.5 w-3.5" /> {t('settings.about.openClusters')}
          </Link>
        </Row>
      </Group>
      <Group title={t('settings.about.shortcuts')}>
        <div className="grid grid-cols-1 gap-x-6 gap-y-1.5 p-4 sm:grid-cols-2">
          {shortcuts.map(([k, v]) => (
            <div key={k} className="flex items-center justify-between gap-3 text-[0.8125rem]">
              <span className="text-[var(--muted-foreground)]">{v}</span>
              <kbd className="rounded border border-[var(--border)] bg-[var(--background)] px-1.5 py-0.5 text-[0.6875rem]">{k}</kbd>
            </div>
          ))}
        </div>
      </Group>
    </div>
  );
}

// ─── Page ───────────────────────────────────────────────────────────────────

function errorMessage(err: unknown): string | null {
  const e = err as { response?: { data?: { error?: { message?: string } } }; message?: string } | null;
  return e?.response?.data?.error?.message ?? e?.message ?? null;
}

export function Settings() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const raw = params.get('section') as Section | null;
  const section: Section = raw && (SETTINGS_SECTIONS as readonly string[]).includes(raw) ? raw : 'appearance';
  const setSection = (s: Section) => setParams({ section: s }, { replace: true });
  const hasCluster = useClusterStore((s) => !!s.activeClusterId);
  const perms = usePermissions();
  const canEdit = perms.hasClusterPerm('cluster.manage');

  const settingsQ = useAppSettings();
  const update = useUpdateAppSettings();
  const server = settingsQ.data?.settings;
  const [draft, setDraftState] = React.useState<AppSettings | null>(null);
  React.useEffect(() => {
    if (server) setDraftState(structuredClone(server));
  }, [server]);
  const setDraft = (fn: (d: AppSettings) => AppSettings) => setDraftState((d) => (d ? fn(d) : d));

  const normalize = (s: AppSettings | null | undefined) => (s ? JSON.stringify({ ...s, updated_at: undefined }) : '');
  const dirty = !!draft && !!server && normalize(draft) !== normalize(server);

  React.useEffect(() => {
    if (!dirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [dirty]);

  const save = () => {
    if (!draft) return;
    update.mutate(draft);
  };
  const saveError = update.isError ? errorMessage(update.error) : null;

  const needsServer = SERVER_SECTIONS.includes(section);
  let content: React.ReactNode;
  if (section === 'appearance') content = <AppearanceSection />;
  else if (section === 'region') content = <RegionSection />;
  else if (section === 'about') content = <AboutSection version={settingsQ.data?.server.version} />;
  else if (!draft || !settingsQ.data) {
    content = settingsQ.isError ? (
      <div className="rounded-lg border border-[var(--border)] bg-[var(--card)] p-6 text-[0.8125rem] text-[var(--muted-foreground)]">{t('settings.loadError')}</div>
    ) : (
      <div className="space-y-3">
        {[0, 1].map((i) => <div key={i} className="h-40 animate-pulse rounded-lg bg-[var(--card)]" />)}
      </div>
    );
  } else if (section === 'dashboard') content = <DashboardSection draft={draft} setDraft={setDraft} canEdit={canEdit} />;
  else if (section === 'monitoring') content = <MonitoringSection draft={draft} setDraft={setDraft} canEdit={canEdit} options={settingsQ.data.options} />;
  else content = <SecuritySection draft={draft} setDraft={setDraft} canEdit={canEdit} info={settingsQ.data} />;

  return (
    <div className="pb-24">
      <PageHeader title={t('settings.title')} subtitle={t('settings.subtitle')} />
      <div className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-5 md:flex-row md:px-6">
        <nav className="md:w-52 md:shrink-0">
          <ul className="flex gap-1 overflow-x-auto pb-1 md:sticky md:top-4 md:flex-col md:overflow-visible scrollbar-thin">
            {SETTINGS_SECTIONS.map((s) => {
              const Icon = SECTION_ICONS[s];
              const active = s === section;
              return (
                <li key={s} className="shrink-0">
                  <button
                    type="button"
                    onClick={() => setSection(s)}
                    className={cn(
                      'flex h-8 w-full items-center gap-2.5 rounded-md px-2.5 text-left text-[0.8125rem] transition-colors',
                      active ? 'bg-[var(--accent)] font-medium text-[var(--foreground)]' : 'text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)]',
                    )}
                  >
                    <Icon className={cn('h-4 w-4', active && 'text-[var(--primary)]')} />
                    <span className="whitespace-nowrap">{t(`settings.section.${s}`)}</span>
                    {SERVER_SECTIONS.includes(s) && dirty && <span className="ml-auto h-1.5 w-1.5 rounded-full bg-[var(--primary)]" />}
                  </button>
                </li>
              );
            })}
          </ul>
          <div className="mt-4 hidden rounded-md border border-[var(--border)] p-3 text-[0.6875rem] leading-relaxed text-[var(--muted-foreground)] md:block">
            <Info className="mb-1 h-3.5 w-3.5" />
            {t('settings.scopeNote')}
          </div>
        </nav>
        <div className="min-w-0 flex-1 space-y-4">
          {needsServer && !canEdit && !perms.loading && (
            <div className="rounded-md border border-[var(--border)] bg-[var(--card)] px-3 py-2 text-[0.75rem] text-[var(--muted-foreground)]">{t('settings.readOnly')}</div>
          )}
          {needsServer && !hasCluster && section === 'monitoring' && (
            <div className="rounded-md border border-[var(--border)] bg-[var(--card)] px-3 py-2 text-[0.75rem] text-[var(--muted-foreground)]">{t('settings.monitoring.noCluster')}</div>
          )}
          {content}
        </div>
      </div>

      {dirty && (
        <div className="fixed inset-x-0 bottom-4 z-40 flex justify-center px-4">
          <div className="flex w-full max-w-xl items-center gap-3 rounded-lg border border-[var(--border)] bg-[var(--popover)] px-4 py-2.5 shadow-2xl">
            <span className="h-2 w-2 shrink-0 rounded-full bg-[var(--primary)]" />
            <div className="min-w-0 flex-1 text-[0.8125rem]">
              <div className="font-medium">{t('settings.unsaved')}</div>
              {saveError && <div className="truncate text-[0.75rem] text-[var(--destructive)]" title={saveError}>{saveError}</div>}
            </div>
            <Button variant="ghost" size="sm" onClick={() => server && setDraftState(structuredClone(server))} disabled={update.isPending}>
              {t('settings.discard')}
            </Button>
            <Button size="sm" onClick={save} disabled={update.isPending || !canEdit}>
              {update.isPending ? t('settings.saving') : t('settings.save')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
