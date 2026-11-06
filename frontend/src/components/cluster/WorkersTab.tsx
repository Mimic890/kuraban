import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Select, SelectOption } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { garageApi } from '@/lib/api';
import { useTranslation } from '@/lib/i18n';
import { queryKeys } from '@/lib/query-client';
import { formatSecsFromNow } from '@/lib/units';
import { useClusterStore } from '@/store/cluster-store';
import type { ClusterNode, WorkerInfo } from '@/types';
import { nodeLabel, sortWorkers } from './helpers';

function WorkerState({ state }: { state: WorkerInfo['state'] }) {
  const { t } = useTranslation();
  if (typeof state === 'object') return <Badge variant="warning">{t('cluster.workers.state_throttled')}</Badge>;
  if (state === 'busy') return <Badge variant="primary">{t('cluster.workers.state_busy')}</Badge>;
  return <Badge>{t(state === 'done' ? 'cluster.workers.state_done' : 'cluster.workers.state_idle')}</Badge>;
}

export function WorkersTab({ nodes }: { nodes: ClusterNode[] }) {
  const { t, language } = useTranslation();
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  const { data } = useQuery({
    queryKey: queryKeys.cluster.workers(activeClusterId),
    queryFn: () => garageApi.listWorkers(),
    refetchInterval: 10000,
  });
  const nodeIds = Object.keys(data?.success ?? {}).sort();
  const [picked, setPicked] = useState<string>();
  const nodeId = picked && nodeIds.includes(picked) ? picked : nodeIds[0];
  const workers = sortWorkers((nodeId && data?.success[nodeId]) || []);

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle>{t('cluster.workers.title')}</CardTitle>
            <CardDescription>{t('cluster.workers.description')}</CardDescription>
          </div>
          {nodeIds.length > 1 && (
            <label className="flex items-center gap-2 text-[0.875rem]">
              <span>{t('cluster.workers.node')}</span>
              <Select value={nodeId} onChange={setPicked} className="w-44">
                {nodeIds.map((id) => (
                  <SelectOption key={id} value={id}>
                    {nodeLabel(nodes, id)}
                  </SelectOption>
                ))}
              </Select>
            </label>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <NodeErrors nodes={nodes} errors={data?.error} />
        {workers.length === 0 ? (
          <p className="py-6 text-center text-sm text-[var(--muted-foreground)]">{t('cluster.workers.no_data')}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('cluster.workers.name')}</TableHead>
                <TableHead>{t('cluster.workers.state')}</TableHead>
                <TableHead className="text-right">{t('cluster.workers.errors')}</TableHead>
                <TableHead>{t('cluster.workers.status')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workers.map((w) => (
                <TableRow key={w.id}>
                  <TableCell className="whitespace-nowrap font-medium">{w.name}</TableCell>
                  <TableCell><WorkerState state={w.state} /></TableCell>
                  <TableCell className={`text-right ${w.consecutiveErrors > 0 ? 'text-[var(--destructive)]' : ''}`}>
                    {w.errors.toLocaleString(language)}
                  </TableCell>
                  <TableCell className="min-w-64 text-[0.8125rem] text-[var(--muted-foreground)]">
                    {[
                      w.progress,
                      w.queueLength ? t('cluster.workers.queue', { count: w.queueLength.toLocaleString(language) }) : null,
                      w.persistentErrors ? t('cluster.workers.persistent_errors', { count: w.persistentErrors.toLocaleString(language) }) : null,
                      ...w.freeform,
                    ]
                      .filter(Boolean)
                      .map((line, i) => (
                        <div key={i}>{line}</div>
                      ))}
                    {w.lastError && (
                      <div className="text-[var(--destructive)]">
                        {t('cluster.workers.last_error', {
                          when: formatSecsFromNow(-w.lastError.secsAgo),
                          message: w.lastError.message,
                        })}
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

export function NodeErrors({ nodes, errors }: { nodes: ClusterNode[]; errors?: Record<string, string> }) {
  const { t } = useTranslation();
  return (
    <>
      {Object.entries(errors ?? {}).map(([id, error]) => (
        <p key={id} className="flex items-start gap-2 text-[0.8125rem] text-amber-600 dark:text-amber-400">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span className="min-w-0 break-words">{t('cluster.node_no_answer', { node: nodeLabel(nodes, id), error })}</span>
        </p>
      ))}
    </>
  );
}
