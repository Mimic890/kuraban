import { useQuery } from '@tanstack/react-query';
import { garageApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { useClusterStore } from '@/store/cluster-store';
import type { BlockError, ClusterNode, WorkerInfo } from '@/types';

export function nodeLabel(nodes: ClusterNode[], id: string): string {
  return nodes.find((n) => n.id === id)?.hostname ?? id.slice(0, 12);
}

const isActive = (w: WorkerInfo) => w.state === 'busy' || typeof w.state === 'object';

// Garage returns workers in no fixed order, and a node runs about 60 of them.
export function sortWorkers(workers: WorkerInfo[]): WorkerInfo[] {
  return [...workers].sort(
    (a, b) =>
      b.consecutiveErrors - a.consecutiveErrors ||
      b.errors - a.errors ||
      Number(isActive(b)) - Number(isActive(a)) ||
      a.name.localeCompare(b.name, undefined, { numeric: true })
  );
}

export type BlockErrorRow = BlockError & { nodeId: string };

export function useBlockErrors(enabled: boolean) {
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  const query = useQuery({
    queryKey: queryKeys.cluster.blockErrors(activeClusterId),
    queryFn: () => garageApi.listBlockErrors(),
    refetchInterval: 15000,
    enabled,
  });
  const rows: BlockErrorRow[] = Object.entries(query.data?.success ?? {}).flatMap(([nodeId, errors]) =>
    errors.map((e) => ({ ...e, nodeId }))
  );
  return { rows, count: rows.length, errors: query.data?.error };
}
