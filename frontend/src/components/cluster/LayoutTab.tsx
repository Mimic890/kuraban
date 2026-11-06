import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Loader2, Pencil, Server, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { ConfirmDialog, GuardedDialog } from '@/components/ui/confirm-dialog';
import { DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { IconTile } from '@/components/ui/icon-tile';
import { Input } from '@/components/ui/input';
import { Select, SelectOption } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { usePermissions } from '@/hooks/usePermissions';
import { garageApi } from '@/lib/api';
import { useTranslation } from '@/lib/i18n';
import { queryKeys } from '@/lib/query-client';
import { formatBytesValue } from '@/lib/units';
import { useClusterStore } from '@/store/cluster-store';
import type { ClusterLayout, ClusterNode, LayoutHistory, LayoutParameters, LayoutPreview, LayoutRoleChange } from '@/types';
import {
  CAPACITY_UNITS,
  capacityToInput,
  hasStagedChanges,
  inputToCapacity,
  layoutKey,
  layoutRows,
  nodeLabel,
  parametersChanged,
  type CapacityUnit,
  type LayoutRow,
} from './helpers';

const garageReason = (error: unknown) =>
  (error as { response?: { data?: { error?: { message?: string } } } }).response?.data?.error?.message;

// Refreshes everything a layout change can touch, and shows Garage's reason on failure.
function useLayoutAction<T, R>(action: (input: T) => Promise<R>, onDone?: (result: R) => void) {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  return useMutation({
    mutationFn: action,
    onSuccess: (result) => onDone?.(result),
    onError: (error: unknown) => {
      toast.error(t('api.errors.generic'), { description: garageReason(error) ?? (error as Error).message ?? t('api.errors.unknown') });
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.cluster.all }),
  });
}

function RoleDialog({
  row,
  zones,
  nodes,
  onClose,
}: {
  row: LayoutRow;
  zones: string[];
  nodes: ClusterNode[];
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const start = row.change && !row.change.remove ? row.change : row.role;
  const startCapacity = start?.capacity ? capacityToInput(start.capacity) : { value: '', unit: 'GiB' as CapacityUnit };
  const startTags = start?.tags.join(', ') ?? '';
  const startGateway = !!start && start.capacity === null;

  const [zone, setZone] = useState(start?.zone ?? '');
  const [gateway, setGateway] = useState(startGateway);
  const [value, setValue] = useState(startCapacity.value);
  const [unit, setUnit] = useState<CapacityUnit>(startCapacity.unit);
  const [tags, setTags] = useState(startTags);

  // The input rounds to two decimals. Untouched, it must not change a capacity set
  // elsewhere, such as 10G from the Garage CLI, which is not a round number of GiB.
  const capacityUntouched = !!start?.capacity && value === startCapacity.value && unit === startCapacity.unit;
  const capacity = capacityUntouched ? start?.capacity ?? null : inputToCapacity(value, unit);

  const prepare = useLayoutAction(garageApi.updateClusterLayout, () => {
    toast.success(t('cluster.layout.prepared'));
    onClose();
  });
  const dirty =
    !prepare.isPending &&
    (zone !== (start?.zone ?? '') || gateway !== startGateway || tags !== startTags || value !== startCapacity.value || unit !== startCapacity.unit);
  const valid = !!zone.trim() && (gateway || capacity !== null);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!valid || prepare.isPending) return;
    prepare.mutate({
      roles: [{
        id: row.id,
        zone: zone.trim(),
        capacity: gateway ? null : capacity,
        tags: tags.split(',').map((tag) => tag.trim()).filter(Boolean),
      }],
    });
  };

  return (
    <GuardedDialog open onOpenChange={(open) => !open && !prepare.isPending && onClose()} size="form" dirty={dirty}>
      <DialogContent>
        <DialogHeader>
          <IconTile icon={<Server />} tone="primary" size="md" />
          <div className="min-w-0 flex-1 pr-6">
            <DialogTitle className="truncate">
              {t(row.role ? 'cluster.layout.edit' : 'cluster.layout.assign')}: {nodeLabel(nodes, row.id)}
            </DialogTitle>
            <DialogDescription>{t('cluster.layout.dialog_description')}</DialogDescription>
          </div>
        </DialogHeader>
        <form onSubmit={submit} className="contents">
          <DialogBody className="space-y-4">
            <label className="block space-y-1.5">
              <span className="text-[0.8125rem] font-medium">{t('cluster.layout.zone')}</span>
              <Input required list="layout-zones" value={zone} onChange={(e) => setZone(e.target.value)} placeholder="dc1" />
              <datalist id="layout-zones">
                {zones.map((z) => <option key={z} value={z} />)}
              </datalist>
              <span className="block text-[0.6875rem] text-[var(--muted-foreground)]">{t('cluster.layout.zone_hint')}</span>
            </label>
            <div className="flex items-center gap-2 text-[0.8125rem]">
              <Switch checked={gateway} onCheckedChange={setGateway} aria-label={t('cluster.layout.is_gateway')} />
              <span>{t('cluster.layout.is_gateway')}</span>
            </div>
            {!gateway && (
              <div className="space-y-1.5">
                <label htmlFor="layout-capacity" className="text-[0.8125rem] font-medium">{t('cluster.layout.capacity')}</label>
                <div className="flex gap-2">
                  <Input id="layout-capacity" required inputMode="decimal" value={value} onChange={(e) => setValue(e.target.value)} placeholder="100" className="min-w-0 flex-1" />
                  <Select value={unit} onChange={(u) => setUnit(u as CapacityUnit)} className="w-24">
                    {Object.keys(CAPACITY_UNITS).map((u) => <SelectOption key={u} value={u}>{u}</SelectOption>)}
                  </Select>
                </div>
                <span className="block text-[0.6875rem] text-[var(--muted-foreground)]">{t('cluster.layout.capacity_hint')}</span>
              </div>
            )}
            <label className="block space-y-1.5">
              <span className="text-[0.8125rem] font-medium">{t('cluster.layout.tags')}</span>
              <Input value={tags} onChange={(e) => setTags(e.target.value)} />
              <span className="block text-[0.6875rem] text-[var(--muted-foreground)]">{t('cluster.layout.tags_hint')}</span>
            </label>
          </DialogBody>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={onClose} disabled={prepare.isPending}>{t('common.cancel')}</Button>
            <Button type="submit" disabled={!valid || prepare.isPending} className="gap-1.5">
              {prepare.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('cluster.layout.prepare')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </GuardedDialog>
  );
}

function redundancyLabel(t: ReturnType<typeof useTranslation>['t'], parameters: LayoutParameters) {
  const value = parameters.zoneRedundancy;
  return value === 'maximum' ? t('cluster.layout.redundancy_maximum') : t('cluster.layout.redundancy_at_least', { count: value.atLeast });
}

function roleLabel(t: ReturnType<typeof useTranslation>['t'], change: Exclude<LayoutRoleChange, { remove: true }>) {
  return `${change.zone}, ${change.capacity === null ? t('cluster.layout.gateway') : formatBytesValue(change.capacity)}`;
}

function StagedCard({ layout, history, nodes, canUpdate, canApply }: {
  layout: ClusterLayout;
  history?: LayoutHistory;
  nodes: ClusterNode[];
  canUpdate: boolean;
  canApply: boolean;
}) {
  const { t, language } = useTranslation();
  const clusterName = useClusterStore((state) => state.clusters.find((c) => c.id === state.activeClusterId)?.name ?? '');
  const [preview, setPreview] = useState<{ key: string; result: LayoutPreview }>();
  const [confirming, setConfirming] = useState<'apply' | 'revert'>();
  const key = layoutKey(layout);
  // Any newer staging or applied version makes the result stale, whoever caused it.
  const current = preview?.key === key ? preview.result : undefined;
  const ready = current && 'newLayout' in current ? current : undefined;

  const runPreview = useLayoutAction(
    async () => {
      // Read first: a change staged in between then leaves an old key, which blocks applying.
      const before = await garageApi.getClusterLayout(true);
      return { key: layoutKey(before), result: await garageApi.previewLayout() };
    },
    setPreview,
  );

  const revert = useLayoutAction(garageApi.revertLayout, () => {
    setConfirming(undefined);
    setPreview(undefined);
    toast.success(t('cluster.layout.reverted'));
  });

  const apply = useLayoutAction(
    async (version: number) => {
      const fresh = await garageApi.getClusterLayout(true);
      if (layoutKey(fresh) !== key) throw new Error(t('cluster.layout.apply_stale'));
      await garageApi.applyLayout(version);
      return version;
    },
    (version) => {
      setPreview(undefined);
      toast.success(t('cluster.layout.applied', { version }));
    },
  );

  const roleIds = new Set([...layout.roles.map((r) => r.id), ...layout.stagedRoleChanges.map((c) => c.id)]);
  const downCount = nodes.filter((n) => !n.isUp && roleIds.has(n.id)).length;
  const removing = layout.stagedRoleChanges.some((c) => c.remove);
  const moved = ready?.statistics?.totalMovedPartitions;
  const applyNote = [
    t('cluster.layout.apply_note'),
    moved != null && t('cluster.layout.apply_note_moved', { count: moved.toLocaleString(language) }),
    removing && t('cluster.layout.apply_note_removed'),
    history?.updateTrackers && t('cluster.layout.apply_note_migrating'),
    downCount > 0 && t('cluster.layout.apply_note_down', { count: downCount }),
  ].filter(Boolean).join(' ');

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle>{t('cluster.layout.staged_title')}</CardTitle>
            <CardDescription>
              {t('cluster.layout.staged_count', { count: layout.stagedRoleChanges.length + (parametersChanged(layout) ? 1 : 0) })}
            </CardDescription>
            {layout.stagedParameters && parametersChanged(layout) && (
              <p className="mt-1 text-[0.8125rem]">{t('cluster.layout.staged_redundancy', { value: redundancyLabel(t, layout.stagedParameters) })}</p>
            )}
          </div>
          <div className="flex flex-wrap gap-2">
            {canUpdate && (
              <Button variant="ghost" onClick={() => setConfirming('revert')} disabled={apply.isPending}>{t('cluster.layout.revert')}</Button>
            )}
            <Button variant={ready ? 'secondary' : 'primary'} onClick={() => runPreview.mutate(undefined)} disabled={runPreview.isPending || apply.isPending} className="gap-1.5">
              {runPreview.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t(preview ? 'cluster.layout.preview_again' : 'cluster.layout.preview')}
            </Button>
            {canApply && (
              <Button onClick={() => setConfirming('apply')} disabled={!ready || apply.isPending} title={ready ? undefined : t('cluster.layout.preview_needed')} className="gap-1.5">
                {apply.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                {t('cluster.layout.apply')}
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      {current && (
        <CardContent className="space-y-3">
          {!ready ? (
            <div role="alert" className="rounded-md border border-[var(--danger-border)] bg-[var(--danger-soft)] px-3 py-2.5 text-[0.8125rem]">
              <p className="font-medium">{t('cluster.layout.preview_failed')}</p>
              <p className="mt-1 break-words">{'error' in current && current.error}</p>
            </div>
          ) : (
            <>
              <ul className="flex flex-wrap gap-x-6 gap-y-1 text-[0.875rem]">
                <li>{t('cluster.layout.preview_new_version', { version: ready.newLayout.version })}</li>
                {moved != null && <li>{t('cluster.layout.preview_moved', { count: moved.toLocaleString(language) })}</li>}
                {ready.statistics && <li>{t('cluster.layout.preview_effective', { size: formatBytesValue(ready.statistics.effectiveCapacity) })}</li>}
              </ul>
              {[ready.statistics?.lowUsableCapacity && 'preview_low_usable', ready.statistics?.lowPartitionSize && 'preview_low_partition']
                .filter(Boolean)
                .map((warning) => (
                  <p key={String(warning)} className="flex gap-2 text-[0.8125rem] text-[var(--warning,inherit)]">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                    {t(`cluster.layout.${warning}`)}
                  </p>
                ))}
              <details open>
                <summary className="cursor-pointer text-[0.8125rem] font-medium">{t('cluster.layout.preview_details')}</summary>
                <pre className="mt-2 overflow-x-auto rounded-lg bg-[var(--surface-sunken)] p-3 font-mono text-xs">{ready.message.join('\n')}</pre>
              </details>
            </>
          )}
        </CardContent>
      )}
      <ConfirmDialog
        open={confirming === 'revert'}
        onOpenChange={(open) => !open && setConfirming(undefined)}
        title={t('cluster.layout.revert_title')}
        note={t('cluster.layout.revert_note')}
        icon={<AlertTriangle />}
        confirmLabel={t('cluster.layout.revert')}
        loading={revert.isPending}
        onConfirm={() => revert.mutate(undefined)}
      />
      <ConfirmDialog
        open={confirming === 'apply'}
        onOpenChange={(open) => !open && setConfirming(undefined)}
        title={t('cluster.layout.apply_title', { version: ready?.newLayout.version ?? '', cluster: clusterName })}
        note={applyNote}
        icon={<AlertTriangle />}
        confirmLabel={t('cluster.layout.apply')}
        onConfirm={() => {
          setConfirming(undefined);
          if (ready) apply.mutate(ready.newLayout.version);
        }}
      />
    </Card>
  );
}

function SkipDeadNodes({ history, nodes }: { history: LayoutHistory; nodes: ClusterNode[] }) {
  const { t } = useTranslation();
  const [acknowledged, setAcknowledged] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const tracked = Object.keys(history.updateTrackers ?? {});
  const downCount = nodes.filter((n) => !n.isUp && tracked.includes(n.id)).length;
  // Same order as the Garage CLI: first skip nodes that have not taken up the new version.
  // Forcing only makes sense once all have, and offline nodes still hold back the data sync.
  const force = history.minAck >= history.currentVersion;

  const skip = useLayoutAction(
    () => garageApi.skipDeadNodes(history.currentVersion, force),
    ({ ackUpdated, syncUpdated }) => {
      const count = new Set([...ackUpdated, ...syncUpdated]).size;
      if (count === 0) toast.info(t('cluster.layout.skip_nothing'));
      else toast.success(t('cluster.layout.skipped', { count }));
      setAcknowledged(false);
    },
  );

  if (downCount === 0) {
    return <p className="text-[0.8125rem] text-[var(--muted-foreground)]">{t('cluster.layout.waiting')}</p>;
  }
  return (
    <div className="space-y-3 rounded-lg border border-[var(--border)] p-4">
      <p className="text-[0.8125rem]">{force ? t('cluster.layout.force_help') : t('cluster.layout.skip_help', { count: downCount })}</p>
      {force && (
        <label className="flex cursor-pointer items-start gap-2 text-[0.8125rem]">
          <Checkbox checked={acknowledged} onCheckedChange={(checked) => setAcknowledged(checked as boolean)} className="mt-0.5" />
          {t('cluster.layout.force_ack')}
        </label>
      )}
      <Button variant={force ? 'destructive' : 'secondary'} size="sm" disabled={skip.isPending || (force && !acknowledged)} onClick={() => setConfirming(true)}>
        {t(force ? 'cluster.layout.force' : 'cluster.layout.skip')}
      </Button>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t(force ? 'cluster.layout.force_title' : 'cluster.layout.skip_title', { version: history.currentVersion })}
        note={t(force ? 'cluster.layout.force_note' : 'cluster.layout.skip_note')}
        icon={<AlertTriangle />}
        confirmLabel={t(force ? 'cluster.layout.force' : 'cluster.layout.skip')}
        onConfirm={() => {
          setConfirming(false);
          skip.mutate(undefined);
        }}
      />
    </div>
  );
}

const STATUS_VARIANT = { Current: 'primary', Draining: 'warning', Historical: 'neutral' } as const;

function HistoryCard({ history, nodes, canSkip }: { history: LayoutHistory; nodes: ClusterNode[]; canSkip: boolean }) {
  const { t, language } = useTranslation();
  const trackers = Object.entries(history.updateTrackers ?? {}).sort(([a], [b]) => a.localeCompare(b));
  const number = (value: number) => value.toLocaleString(language);
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('cluster.layout.history_title')}</CardTitle>
        <CardDescription>{t('cluster.layout.history_description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {trackers.length > 0 && (
          <div className="space-y-3">
            <div>
              <h3 className="flex items-center gap-2 text-[0.875rem] font-medium">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t('cluster.layout.migrating_title')}
              </h3>
              <p className="mt-1 text-[0.8125rem] text-[var(--muted-foreground)]">{t('cluster.layout.migrating_description')}</p>
            </div>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('cluster.layout.col_node')}</TableHead>
                    <TableHead className="text-right">{t('cluster.layout.col_ack')}</TableHead>
                    <TableHead className="text-right">{t('cluster.layout.col_sync')}</TableHead>
                    <TableHead className="text-right">{t('cluster.layout.col_sync_ack')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {trackers.map(([id, tracker]) => (
                    <TableRow key={id}>
                      <TableCell className="whitespace-nowrap" title={id}>
                        {nodeLabel(nodes, id)}
                        {nodes.some((n) => n.id === id && !n.isUp) && <Badge variant="danger" className="ml-2">{t('cluster.layout.node_down')}</Badge>}
                      </TableCell>
                      <TableCell className="text-right">{number(tracker.ack)}</TableCell>
                      <TableCell className="text-right">{number(tracker.sync)}</TableCell>
                      <TableCell className="text-right">{number(tracker.syncAck)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            {canSkip && <SkipDeadNodes history={history} nodes={nodes} />}
          </div>
        )}
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('cluster.layout.col_version')}</TableHead>
                <TableHead>{t('cluster.layout.col_status')}</TableHead>
                <TableHead className="text-right">{t('cluster.layout.col_storage')}</TableHead>
                <TableHead className="text-right">{t('cluster.layout.col_gateways')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {history.versions.map((version) => (
                <TableRow key={version.version}>
                  <TableCell>{number(version.version)}</TableCell>
                  <TableCell>
                    <Badge variant={STATUS_VARIANT[version.status]}>{t(`cluster.layout.status_${version.status.toLowerCase()}`)}</Badge>
                  </TableCell>
                  <TableCell className="text-right">{number(version.storageNodes)}</TableCell>
                  <TableCell className="text-right">{number(version.gatewayNodes)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}

export function LayoutTab({ nodes }: { nodes: ClusterNode[] }) {
  const { t, language } = useTranslation();
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  const { hasClusterPerm } = usePermissions();
  const canUpdate = hasClusterPerm('cluster.layout.update');
  const canApply = hasClusterPerm('cluster.layout.apply');
  const canHistory = hasClusterPerm('cluster.layout.history');
  const canSkip = hasClusterPerm('cluster.layout.skip_dead_nodes');
  const [editing, setEditing] = useState<LayoutRow>();
  const [removing, setRemoving] = useState<LayoutRow>();

  const { data: layout, isError } = useQuery({
    queryKey: queryKeys.cluster.layout(activeClusterId),
    queryFn: () => garageApi.getClusterLayout(),
    refetchInterval: 10000,
  });
  const { data: history } = useQuery({
    queryKey: queryKeys.cluster.layoutHistory(activeClusterId),
    queryFn: () => garageApi.getLayoutHistory(),
    refetchInterval: 10000,
    enabled: canHistory,
  });

  const remove = useLayoutAction(garageApi.updateClusterLayout, () => setRemoving(undefined));
  const setRedundancy = useLayoutAction(garageApi.updateClusterLayout);

  if (!layout) {
    return <div className="py-8 text-center text-[0.875rem] text-[var(--muted-foreground)]">{t(isError ? 'cluster.unavailable_title' : 'common.loading')}</div>;
  }

  const rows = layoutRows(nodes, layout);
  const zones = [...new Set(layout.roles.map((r) => r.zone))].sort();
  const redundancy = (layout.stagedParameters ?? layout.parameters).zoneRedundancy;
  const redundancyValue = redundancy === 'maximum' ? 'maximum' : String(redundancy.atLeast);
  // Garage refuses more than the replication factor with a clear reason, and the API does not report that factor.
  const redundancyOptions = Array.from({ length: Math.max(zones.length, 3, Number(redundancyValue) || 0) }, (_, i) => i + 1);

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <CardTitle>{t('cluster.layout.title')}</CardTitle>
              <CardDescription>{t('cluster.layout.description')}</CardDescription>
              <p className="mt-2 flex flex-wrap gap-x-4 text-[0.8125rem] text-[var(--muted-foreground)]">
                <span>{t('cluster.layout.version', { version: layout.version })}</span>
                {layout.partitionSize > 0 && <span>{t('cluster.layout.partition_size', { size: formatBytesValue(layout.partitionSize) })}</span>}
              </p>
            </div>
            <label className="flex items-center gap-2 text-[0.875rem]">
              <span>{t('cluster.layout.redundancy')}</span>
              {canUpdate ? (
                <Select
                  value={redundancyValue}
                  onChange={(value) => setRedundancy.mutate({
                    parameters: { zoneRedundancy: value === 'maximum' ? 'maximum' : { atLeast: Number(value) } },
                  })}
                  disabled={setRedundancy.isPending}
                  className="w-40"
                >
                  <SelectOption value="maximum">{t('cluster.layout.redundancy_maximum')}</SelectOption>
                  {redundancyOptions.map((count) => (
                    <SelectOption key={count} value={String(count)}>{t('cluster.layout.redundancy_at_least', { count })}</SelectOption>
                  ))}
                </Select>
              ) : (
                <strong className="font-medium">{redundancyLabel(t, layout.parameters)}</strong>
              )}
            </label>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('cluster.layout.col_node')}</TableHead>
                  <TableHead>{t('cluster.layout.col_zone')}</TableHead>
                  <TableHead>{t('cluster.layout.col_capacity')}</TableHead>
                  <TableHead>{t('cluster.layout.col_tags')}</TableHead>
                  <TableHead className="text-right">{t('cluster.layout.col_partitions')}</TableHead>
                  <TableHead>{t('cluster.layout.col_change')}</TableHead>
                  {canUpdate && <TableHead />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="whitespace-nowrap" title={row.id}>
                      <span className="font-mono text-[0.8125rem]">{row.id.slice(0, 12)}</span>
                      {row.node?.hostname && <span className="ml-2 text-[var(--muted-foreground)]">{row.node.hostname}</span>}
                      {row.node?.isUp === false && <Badge variant="danger" className="ml-2">{t('cluster.layout.node_down')}</Badge>}
                    </TableCell>
                    <TableCell className="whitespace-nowrap">{row.role?.zone ?? <span className="text-[var(--muted-foreground)]">{t('cluster.layout.no_role')}</span>}</TableCell>
                    <TableCell className="whitespace-nowrap">
                      {row.role && (row.role.capacity === null ? t('cluster.layout.gateway') : formatBytesValue(row.role.capacity))}
                    </TableCell>
                    <TableCell>{row.role?.tags.join(', ')}</TableCell>
                    <TableCell className="text-right">{row.role?.storedPartitions?.toLocaleString(language)}</TableCell>
                    <TableCell className="whitespace-nowrap">
                      {row.change && (
                        <Badge variant={row.change.remove ? 'danger' : 'warning'}>
                          {row.change.remove
                            ? t('cluster.layout.change_remove')
                            : t(row.role ? 'cluster.layout.change_to' : 'cluster.layout.change_new', { role: roleLabel(t, row.change) })}
                        </Badge>
                      )}
                    </TableCell>
                    {canUpdate && (
                      <TableCell>
                        <div className="flex justify-end gap-1">
                          <Button size="sm" variant="ghost" onClick={() => setEditing(row)} aria-label={t(row.role ? 'cluster.layout.edit' : 'cluster.layout.assign')} title={t(row.role ? 'cluster.layout.edit' : 'cluster.layout.assign')}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          {row.role && !row.change?.remove && (
                            <Button size="sm" variant="ghost" onClick={() => setRemoving(row)} aria-label={t('cluster.layout.remove')} title={t('cluster.layout.remove')}>
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {hasStagedChanges(layout) && (
        <StagedCard layout={layout} history={history} nodes={nodes} canUpdate={canUpdate} canApply={canApply} />
      )}
      {history && <HistoryCard history={history} nodes={nodes} canSkip={canSkip} />}

      {editing && <RoleDialog key={editing.id} row={editing} zones={zones} nodes={nodes} onClose={() => setEditing(undefined)} />}
      <ConfirmDialog
        open={!!removing}
        onOpenChange={(open) => !open && setRemoving(undefined)}
        title={t('cluster.layout.remove_title', { node: removing ? nodeLabel(nodes, removing.id) : '' })}
        note={t('cluster.layout.remove_note')}
        confirmLabel={t('cluster.layout.remove_confirm')}
        loading={remove.isPending}
        onConfirm={() => removing && remove.mutate({ roles: [{ id: removing.id, remove: true }] })}
      />
    </>
  );
}
