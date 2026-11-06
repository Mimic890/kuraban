import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2 } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { EmptyState } from '@/components/ui/empty-state';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useBuckets } from '@/hooks/useApi';
import { usePermissions } from '@/hooks/usePermissions';
import { garageApi } from '@/lib/api';
import { useTranslation } from '@/lib/i18n';
import { queryKeys } from '@/lib/query-client';
import { formatSecsFromNow } from '@/lib/units';
import { useClusterStore } from '@/store/cluster-store';
import type { BlockVersion, ClusterNode } from '@/types';
import { nodeLabel, useBlockErrors, type BlockErrorRow } from './helpers';
import { NodeErrors } from './WorkersTab';

// shortcut: a node that lost a disk can report many thousands of blocks, only the first rows render; add paging if someone needs to browse them.
const MAX_ROWS = 200;

export function BlockErrorsTab({ nodes }: { nodes: ClusterNode[] }) {
  const { t, language } = useTranslation();
  const { hasClusterPerm } = usePermissions();
  const queryClient = useQueryClient();
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  const { rows, count, errors } = useBlockErrors(true);
  const [details, setDetails] = useState<BlockErrorRow | null>(null);
  const canRetry = hasClusterPerm('block.retry_resync');

  const retry = useMutation({
    mutationFn: (row?: BlockErrorRow) => garageApi.retryBlockResync(row?.nodeId, row?.blockHash),
    onSuccess: (result) => {
      // Garage answers 200 even when a node refused, for example a block no longer in error.
      Object.entries(result.error).forEach(([id, error]) =>
        toast.error(t('cluster.node_no_answer', { node: nodeLabel(nodes, id), error }))
      );
      const total = Object.values(result.success).reduce((sum, r) => sum + r.count, 0);
      if (total > 0) toast.success(t('cluster.block_errors.retried', { count: total.toLocaleString(language) }));
      queryClient.invalidateQueries({ queryKey: queryKeys.cluster.blockErrors(activeClusterId) });
    },
  });

  const sorted = [...rows].sort((a, b) => b.errorCount - a.errorCount).slice(0, MAX_ROWS);

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <CardTitle>{t('cluster.block_errors.title')}</CardTitle>
            <CardDescription>{t('cluster.block_errors.description')}</CardDescription>
          </div>
          {canRetry && count > 0 && (
            <Button variant="secondary" disabled={retry.isPending} onClick={() => retry.mutate(undefined)}>
              {t('cluster.block_errors.retry_all')}
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <NodeErrors nodes={nodes} errors={errors} />
        {count === 0 ? (
          <EmptyState
            icon={<CheckCircle2 />}
            tone="neutral"
            title={t('cluster.block_errors.none')}
            description={t('cluster.block_errors.none_description')}
          />
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('cluster.block_errors.block')}</TableHead>
                  <TableHead>{t('cluster.block_errors.node')}</TableHead>
                  <TableHead className="text-right">{t('cluster.block_errors.references')}</TableHead>
                  <TableHead className="text-right">{t('cluster.block_errors.attempts')}</TableHead>
                  <TableHead>{t('cluster.block_errors.last_try')}</TableHead>
                  <TableHead>{t('cluster.block_errors.next_try')}</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {sorted.map((row) => (
                  <TableRow key={`${row.nodeId}:${row.blockHash}`}>
                    <TableCell className="font-mono text-[0.8125rem]" title={row.blockHash}>{row.blockHash.slice(0, 16)}</TableCell>
                    <TableCell className="whitespace-nowrap">{nodeLabel(nodes, row.nodeId)}</TableCell>
                    <TableCell className="text-right">{row.refcount.toLocaleString(language)}</TableCell>
                    <TableCell className="text-right">{row.errorCount.toLocaleString(language)}</TableCell>
                    <TableCell className="whitespace-nowrap">{formatSecsFromNow(-row.lastTrySecsAgo)}</TableCell>
                    <TableCell className="whitespace-nowrap">{formatSecsFromNow(row.nextTryInSecs)}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        {hasClusterPerm('block.info') && (
                          <Button size="sm" variant="ghost" onClick={() => setDetails(row)}>
                            {t('cluster.block_errors.details')}
                          </Button>
                        )}
                        {canRetry && (
                          <Button size="sm" variant="secondary" disabled={retry.isPending} onClick={() => retry.mutate(row)}>
                            {t('cluster.block_errors.retry')}
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {count > sorted.length && (
              <p className="text-[0.8125rem] text-[var(--muted-foreground)]">
                {t('cluster.block_errors.shown_limit', { shown: sorted.length.toLocaleString(language), total: count.toLocaleString(language) })}
              </p>
            )}
          </>
        )}
      </CardContent>
      {details && <BlockDetails row={details} nodes={nodes} onClose={() => setDetails(null)} />}
    </Card>
  );
}

function BlockDetails({ row, nodes, onClose }: { row: BlockErrorRow; nodes: ClusterNode[]; onClose: () => void }) {
  const { t } = useTranslation();
  const { hasAnyPerm } = usePermissions();
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  const { data: buckets = [] } = useBuckets(hasAnyPerm('bucket.list'));
  const { data } = useQuery({
    queryKey: queryKeys.cluster.block(activeClusterId, row.nodeId, row.blockHash),
    queryFn: () => garageApi.getBlockInfo(row.nodeId, row.blockHash),
  });
  const info = data?.success[row.nodeId];
  const bucketName = (id?: string | null) => (id && buckets.find((b) => b.id === id)?.name) || id?.slice(0, 16);

  const describe = (v: BlockVersion) => {
    const link = v.backlink;
    const deleted = v.refDeleted || v.versionDeleted || v.garbageCollected;
    if (link && 'object' in link) {
      return { label: t('cluster.block_errors.object'), bucket: bucketName(link.object.bucketId), key: link.object.key, deleted };
    }
    if (link && 'upload' in link) {
      return {
        label: t('cluster.block_errors.upload'),
        bucket: bucketName(link.upload.bucketId),
        key: link.upload.key,
        deleted: deleted || link.upload.uploadDeleted,
      };
    }
    return { label: t('cluster.block_errors.unknown_object'), deleted };
  };

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <div className="min-w-0 flex-1 pr-6">
            <DialogTitle>{t('cluster.block_errors.details_title')}</DialogTitle>
          </div>
        </DialogHeader>
        <DialogBody className="space-y-4 text-[0.875rem]">
          <p className="break-all font-mono text-[0.8125rem]">{row.blockHash}</p>
          <p>{t('cluster.block_errors.node')}: {nodeLabel(nodes, row.nodeId)}</p>
          <NodeErrors nodes={nodes} errors={data?.error} />
          {info && (
            <div className="space-y-2">
              <h3 className="font-medium">{t('cluster.block_errors.used_by')}</h3>
              {info.versions.length === 0 && (
                <p className="text-[var(--muted-foreground)]">{t('cluster.block_errors.no_references')}</p>
              )}
              {info.versions.map((v) => {
                const d = describe(v);
                return (
                  <div key={v.versionId} className="rounded-lg border border-[var(--border)] px-3 py-2">
                    <div className="flex flex-wrap items-center gap-2 text-[0.8125rem] text-[var(--muted-foreground)]">
                      {d.label}
                      {d.deleted && <Badge>{t('cluster.block_errors.deleted')}</Badge>}
                    </div>
                    {d.key && <div className="break-all">{d.bucket} / {d.key}</div>}
                  </div>
                );
              })}
            </div>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>{t('common.close')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
