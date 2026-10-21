import * as React from 'react';
import { useLocation } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { CheckCircle2, Loader2, Pencil, Plug, Plus, RefreshCw, Server, ShieldPlus, Trash2, XCircle } from 'lucide-react';
import { PageHeader } from '@/components/ui/page-header';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { useClusterStore, type ClusterConfig } from '@/store/cluster-store';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { clusterApi, describeApiError, normalizeDraft, type BlockedEndpoint, type ClusterDraft, type ConnectionCheck } from '@/lib/clusters';
import { settingsApi } from '@/lib/telemetry';
import { settingsKey } from '@/hooks/useTelemetry';

const EMPTY_DRAFT: ClusterDraft = {
  name: '',
  region: 'garage',
  endpoint: '',
  admin_endpoint: '',
  admin_token: '',
  use_ssl: false,
  force_path_style: true,
};

// The add form survives reloads and navigation. The admin token is left out on
// purpose: secrets never go to web storage.
const DRAFT_KEY = 'connections-add-draft';

function loadDraft(): ClusterDraft {
  try {
    const raw = sessionStorage.getItem(DRAFT_KEY);
    if (raw) return { ...EMPTY_DRAFT, ...JSON.parse(raw), admin_token: '' };
  } catch {
    // storage unavailable
  }
  return EMPTY_DRAFT;
}

function saveDraft(d: ClusterDraft | null) {
  try {
    if (d) sessionStorage.setItem(DRAFT_KEY, JSON.stringify({ ...d, admin_token: undefined }));
    else sessionStorage.removeItem(DRAFT_KEY);
  } catch {
    // storage unavailable
  }
}

function clusterToDraft(c: ClusterConfig): ClusterDraft {
  return {
    name: c.name,
    region: c.region,
    endpoint: c.endpoint,
    admin_endpoint: c.admin_endpoint,
    admin_token: '',
    use_ssl: c.use_ssl,
    force_path_style: c.force_path_style,
  };
}

function s3Url(c: Pick<ClusterConfig, 'endpoint' | 'use_ssl'>) {
  return /^https?:\/\//i.test(c.endpoint) ? c.endpoint : `${c.use_ssl ? 'https' : 'http'}://${c.endpoint}`;
}

function CheckRow({ label, check }: { label: string; check: { ok: boolean; status?: number; message?: string } }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-start gap-2 text-[0.8125rem]">
      {check.ok ? (
        <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-[var(--success)]" />
      ) : (
        <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-[var(--destructive)]" />
      )}
      <span className="w-24 shrink-0 font-medium">{label}</span>
      <span className="min-w-0 break-words text-[var(--muted-foreground)]">
        {check.ok ? (check.status ? t('connections.check.okStatus', { status: check.status }) : t('connections.check.ok')) : check.message}
      </span>
    </div>
  );
}

function CheckResult({ result }: { result: ConnectionCheck }) {
  const { t } = useTranslation();
  return (
    <div
      className={cn(
        'space-y-1.5 rounded-md border px-3 py-2.5',
        result.ok ? 'border-[var(--success)]/40 bg-[var(--success-soft)]' : 'border-[var(--danger-border)] bg-[var(--danger-soft)]'
      )}
    >
      <div className="text-[0.8125rem] font-semibold">
        {result.ok ? t('connections.check.success', { version: result.api_version ?? '?' }) : t('connections.check.failed')}
      </div>
      <CheckRow label={t('connections.admin_api')} check={result.admin} />
      <CheckRow label="S3" check={result.s3} />
    </div>
  );
}

/** Offers to add a refused private address to the endpoint allowlist. */
function BlockedNotice({ blocked, message, onAllowed }: { blocked?: BlockedEndpoint; message: string; onAllowed: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [busy, setBusy] = React.useState(false);

  const allow = async () => {
    if (!blocked) return;
    setBusy(true);
    try {
      const current = await settingsApi.get();
      const list = current.settings.overrides.cluster_endpoint_allowlist ?? current.effective.cluster_endpoint_allowlist ?? [];
      if (!list.includes(blocked.ip)) {
        const next = { ...current.settings, overrides: { ...current.settings.overrides, cluster_endpoint_allowlist: [...list, blocked.ip] } };
        const saved = await settingsApi.update(next);
        qc.setQueryData(settingsKey, saved);
      }
      toast.success(t('connections.allowlist.added', { ip: blocked.ip }));
      onAllowed();
    } catch (err) {
      toast.error(describeApiError(err).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-2 rounded-md border border-[var(--danger-border)] bg-[var(--danger-soft)] px-3 py-2.5 text-[0.8125rem]">
      <div className="flex items-start gap-2">
        <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-[var(--destructive)]" />
        <span className="break-words">{message}</span>
      </div>
      {blocked?.private && (
        <div className="flex flex-wrap items-center gap-2 pl-6">
          <Button type="button" size="sm" variant="secondary" onClick={allow} disabled={busy} className="gap-1.5">
            {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ShieldPlus className="h-3.5 w-3.5" />}
            {t('connections.allowlist.allow', { ip: blocked.ip })}
          </Button>
          <span className="text-[0.75rem] text-[var(--muted-foreground)]">{t('connections.allowlist.hint')}</span>
        </div>
      )}
    </div>
  );
}

function Field({ label, hint, children, className }: { label: string; hint?: string; children: React.ReactNode; className?: string }) {
  return (
    <label className={cn('block space-y-1.5', className)}>
      <span className="text-[0.8125rem] font-medium">{label}</span>
      {children}
      {hint && <span className="block text-[0.6875rem] text-[var(--muted-foreground)]">{hint}</span>}
    </label>
  );
}

function ClusterForm({ cluster, onClose }: { cluster?: ClusterConfig; onClose: () => void }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { addCluster, updateCluster } = useClusterStore();
  const editing = !!cluster;
  const [draft, setDraft] = React.useState<ClusterDraft>(() => (cluster ? clusterToDraft(cluster) : loadDraft()));
  const [saving, setSaving] = React.useState(false);
  const [testing, setTesting] = React.useState(false);
  const [result, setResult] = React.useState<ConnectionCheck | null>(null);
  const [error, setError] = React.useState<{ message: string; blocked?: BlockedEndpoint } | null>(null);

  React.useEffect(() => {
    if (!editing) saveDraft(draft);
  }, [draft, editing]);

  const set = <K extends keyof ClusterDraft>(key: K, value: ClusterDraft[K]) => {
    setDraft((d) => ({ ...d, [key]: value }));
    setResult(null);
  };

  const hasScheme = /^https?:\/\//i.test(draft.endpoint.trim());
  const tokenMissing = !editing && !draft.admin_token;

  const runTest = async () => {
    setTesting(true);
    setError(null);
    setResult(null);
    try {
      setResult(await clusterApi.test({ ...normalizeDraft(draft), id: cluster?.id }));
    } catch (err) {
      setError(describeApiError(err));
    } finally {
      setTesting(false);
    }
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const body = normalizeDraft(draft);
      if (cluster) {
        await updateCluster(cluster.id, body);
        toast.success(t('connections.notifications.updated'));
      } else {
        await addCluster(body);
        saveDraft(null);
        toast.success(t('connections.notifications.added'));
      }
      qc.invalidateQueries({ queryKey: ['cluster-check'] });
      onClose();
    } catch (err) {
      setError(describeApiError(err));
    } finally {
      setSaving(false);
    }
  };

  const cancel = () => {
    if (!editing) saveDraft(null);
    onClose();
  };

  return (
    <Card className="border-[var(--primary)]/50">
      <CardHeader>
        <CardTitle>{editing ? t('connections.edit_title', { name: cluster.name }) : t('connections.add_new')}</CardTitle>
        <CardDescription>{editing ? t('connections.edit_desc') : t('connections.add_desc')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <Field label={t('connections.name')}>
              <Input required value={draft.name} onChange={(e) => set('name', e.target.value)} placeholder={t('connections.name_placeholder')} />
            </Field>
            <Field label={t('connections.region')} hint={t('connections.region_hint')}>
              <Input required value={draft.region} onChange={(e) => set('region', e.target.value)} placeholder="garage" />
            </Field>
            <Field label={t('connections.s3_endpoint')} hint={t('connections.s3_hint')}>
              <Input required value={draft.endpoint} onChange={(e) => set('endpoint', e.target.value)} placeholder="http://garage:3900" />
            </Field>
            <Field label={t('connections.admin_endpoint')} hint={t('connections.admin_hint')}>
              <Input required value={draft.admin_endpoint} onChange={(e) => set('admin_endpoint', e.target.value)} placeholder="http://garage:3903" />
            </Field>
            <Field
              label={t('connections.admin_token')}
              hint={editing ? t('connections.token_keep') : t('connections.token_hint')}
              className="md:col-span-2"
            >
              <Input
                required={!editing}
                type="password"
                autoComplete="off"
                value={draft.admin_token}
                onChange={(e) => set('admin_token', e.target.value)}
                placeholder={editing ? '••••••••' : 'admin_token'}
              />
            </Field>
          </div>

          <div className="flex flex-wrap gap-x-6 gap-y-3">
            <div className="flex items-center gap-2 text-[0.8125rem]">
              <Switch checked={hasScheme ? /^https/i.test(draft.endpoint.trim()) : draft.use_ssl} disabled={hasScheme} onCheckedChange={(v) => set('use_ssl', v)} />
              <span>{t('connections.use_ssl')}</span>
              {hasScheme && <span className="text-[0.6875rem] text-[var(--muted-foreground)]">{t('connections.use_ssl_from_url')}</span>}
            </div>
            <div className="flex items-center gap-2 text-[0.8125rem]">
              <Switch checked={draft.force_path_style} onCheckedChange={(v) => set('force_path_style', v)} />
              <span>{t('connections.path_style')}</span>
            </div>
          </div>

          {error && <BlockedNotice message={error.message} blocked={error.blocked} onAllowed={runTest} />}
          {result && <CheckResult result={result} />}

          <div className="flex flex-wrap items-center justify-end gap-2 pt-2">
            {!editing && <span className="mr-auto text-[0.6875rem] text-[var(--muted-foreground)]">{t('connections.save_unreachable')}</span>}
            <Button type="button" variant="ghost" onClick={cancel}>
              {t('connections.cancel')}
            </Button>
            <Button type="button" variant="secondary" onClick={runTest} disabled={testing || tokenMissing || !draft.admin_endpoint || !draft.endpoint} className="gap-1.5">
              {testing ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plug className="h-4 w-4" />}
              {t('connections.test')}
            </Button>
            <Button type="submit" disabled={saving} className="gap-1.5">
              {saving && <Loader2 className="h-4 w-4 animate-spin" />}
              {editing ? t('connections.save_changes') : t('connections.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function ClusterStatus({ cluster }: { cluster: ClusterConfig }) {
  const { t } = useTranslation();
  const q = useQuery({
    queryKey: ['cluster-check', cluster.id, cluster.endpoint, cluster.admin_endpoint],
    queryFn: () => clusterApi.test({ ...clusterToDraft(cluster), id: cluster.id }),
    staleTime: 30_000,
    retry: false,
  });
  if (q.isLoading) {
    return (
      <Badge className="gap-1">
        <Loader2 className="h-3 w-3 animate-spin" /> {t('connections.status.checking')}
      </Badge>
    );
  }
  const reason = q.data
    ? [!q.data.admin.ok && `${t('connections.admin_api')}: ${q.data.admin.message}`, !q.data.s3.ok && `S3: ${q.data.s3.message}`].filter(Boolean).join(' · ')
    : q.error
      ? describeApiError(q.error).message
      : '';
  const ok = q.data?.ok;
  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <Badge variant={ok ? 'success' : 'danger'}>{ok ? t('connections.status.online', { version: q.data?.api_version ?? '' }) : t('connections.status.offline')}</Badge>
        <button type="button" onClick={() => q.refetch()} className="text-[var(--muted-foreground)] hover:text-[var(--foreground)]" aria-label={t('connections.test')} title={t('connections.test')}>
          <RefreshCw className={cn('h-3.5 w-3.5', q.isFetching && 'animate-spin')} />
        </button>
      </div>
      {!ok && reason && <p className="break-words text-[0.6875rem] text-[var(--destructive)]">{reason}</p>}
    </div>
  );
}

export function Connections() {
  const { clusters, activeClusterId, setActiveCluster, deleteCluster } = useClusterStore();
  const { t } = useTranslation();
  const location = useLocation();
  const [adding, setAdding] = React.useState(() => {
    try {
      return !!sessionStorage.getItem(DRAFT_KEY);
    } catch {
      return false;
    }
  });
  const [editingId, setEditingId] = React.useState<string | null>(null);
  const [removing, setRemoving] = React.useState<ClusterConfig | null>(null);

  React.useEffect(() => {
    if (location.state?.addCluster) {
      setAdding(true);
      window.history.replaceState({}, '');
    }
  }, [location.state]);

  const confirmRemove = async () => {
    if (!removing) return;
    try {
      await deleteCluster(removing.id);
      toast.success(t('connections.notifications.removed'));
    } catch {
      toast.error(t('connections.errors.removeFailed'));
    }
    setRemoving(null);
  };

  return (
    <div>
      <PageHeader title={t('connections.title')} subtitle={t('connections.subtitle')} />

      <div className="max-w-5xl space-y-6 p-6">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold tracking-tight">{t('connections.connected')}</h2>
          {!adding && (
            <Button onClick={() => setAdding(true)} className="gap-2">
              <Plus className="h-4 w-4" />
              {t('connections.add')}
            </Button>
          )}
        </div>

        {adding && <ClusterForm onClose={() => setAdding(false)} />}

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {clusters.map((cluster) =>
            editingId === cluster.id ? (
              <div key={cluster.id} className="md:col-span-2">
                <ClusterForm cluster={cluster} onClose={() => setEditingId(null)} />
              </div>
            ) : (
              <Card key={cluster.id} className={activeClusterId === cluster.id ? 'ring-2 ring-[var(--primary)]' : ''}>
                <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                  <div className="min-w-0 space-y-1">
                    <CardTitle className="flex items-center gap-2 text-base">
                      <Server className="h-4 w-4 text-[var(--primary)]" />
                      <span className="truncate">{cluster.name}</span>
                      {activeClusterId === cluster.id && (
                        <Badge variant="warning" className="gap-1">
                          <CheckCircle2 className="h-3 w-3" /> {t('connections.active')}
                        </Badge>
                      )}
                    </CardTitle>
                    <CardDescription className="truncate font-mono text-[0.6875rem]">{cluster.id}</CardDescription>
                  </div>
                  <div className="-mr-2 flex shrink-0">
                    <Button variant="ghost" size="icon" aria-label={t('connections.editAriaLabel', { name: cluster.name })} title={t('connections.edit')} onClick={() => setEditingId(cluster.id)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={t('connections.removeAriaLabel', { name: cluster.name })}
                      className="text-[var(--destructive)] hover:bg-[var(--danger-soft)]"
                      onClick={() => setRemoving(cluster)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3 pt-3">
                  <ClusterStatus cluster={cluster} />
                  <div className="grid grid-cols-[100px_1fr] gap-1 text-[0.8125rem]">
                    <span className="text-[var(--muted-foreground)]">{t('connections.s3_url')}:</span>
                    <span className="truncate font-medium">{s3Url(cluster)}</span>
                    <span className="text-[var(--muted-foreground)]">{t('connections.admin_api')}:</span>
                    <span className="truncate font-medium">{cluster.admin_endpoint}</span>
                    <span className="text-[var(--muted-foreground)]">{t('connections.region')}:</span>
                    <span className="font-medium">{cluster.region}</span>
                  </div>
                  {activeClusterId !== cluster.id && (
                    <Button variant="secondary" className="mt-2 w-full" onClick={() => setActiveCluster(cluster.id)}>
                      {t('connections.switch')}
                    </Button>
                  )}
                </CardContent>
              </Card>
            )
          )}
          {clusters.length === 0 && !adding && (
            <div className="col-span-full rounded-xl border border-dashed border-[var(--border)] py-12 text-center text-[var(--muted-foreground)]">
              <Server className="mx-auto mb-3 h-12 w-12 opacity-20" />
              <p>{t('connections.no_clusters')}</p>
              <Button variant="link" onClick={() => setAdding(true)}>
                {t('connections.add_first')}
              </Button>
            </div>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={!!removing}
        onOpenChange={(o) => !o && setRemoving(null)}
        title={t('connections.confirmRemove')}
        description={removing?.name}
        tone="destructive"
        onConfirm={confirmRemove}
      />
    </div>
  );
}
