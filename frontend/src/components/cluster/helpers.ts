import { useQuery } from '@tanstack/react-query';
import { garageApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { useClusterStore } from '@/store/cluster-store';
import type { BlockError, ClusterLayout, ClusterNode, LayoutNodeRole, LayoutRoleChange, WorkerInfo } from '@/types';

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

// Garage keeps a staged parameter even after it is set back to the current value.
export function parametersChanged(layout: ClusterLayout): boolean {
  return !!layout.stagedParameters && JSON.stringify(layout.stagedParameters) !== JSON.stringify(layout.parameters);
}

// Applying with nothing changed would still create a new layout version.
export function hasStagedChanges(layout: ClusterLayout): boolean {
  return layout.stagedRoleChanges.length > 0 || parametersChanged(layout);
}

// Identifies what a preview was computed from, so a preview stops counting as soon as
// anyone stages something else or applies a layout.
export function layoutKey(layout: ClusterLayout): string {
  return JSON.stringify([layout.version, layout.stagedRoleChanges, layout.stagedParameters]);
}

export interface LayoutRow {
  id: string;
  node?: ClusterNode;
  role?: LayoutNodeRole;
  change?: LayoutRoleChange;
}

// A node can be known without a role, and hold a role while missing from the status.
export function layoutRows(nodes: ClusterNode[], layout: ClusterLayout): LayoutRow[] {
  const ids = new Set([
    ...nodes.map((n) => n.id),
    ...layout.roles.map((r) => r.id),
    ...layout.stagedRoleChanges.map((c) => c.id),
  ]);
  return [...ids]
    .map((id) => ({
      id,
      node: nodes.find((n) => n.id === id),
      role: layout.roles.find((r) => r.id === id),
      change: layout.stagedRoleChanges.find((c) => c.id === id),
    }))
    .sort((a, b) =>
      (a.role?.zone ?? '\uffff').localeCompare(b.role?.zone ?? '\uffff') || a.id.localeCompare(b.id)
    );
}

export const CAPACITY_UNITS = { MiB: 1024 ** 2, GiB: 1024 ** 3, TiB: 1024 ** 4 };
export type CapacityUnit = keyof typeof CAPACITY_UNITS;

export function capacityToInput(bytes: number): { value: string; unit: CapacityUnit } {
  const unit: CapacityUnit = bytes >= CAPACITY_UNITS.TiB ? 'TiB' : bytes >= CAPACITY_UNITS.GiB ? 'GiB' : 'MiB';
  return { value: String(Math.round((bytes / CAPACITY_UNITS[unit]) * 100) / 100), unit };
}

// Returns null for anything Garage would refuse, which starts at 1 KiB.
export function inputToCapacity(value: string, unit: CapacityUnit): number | null {
  const bytes = Math.round(Number(value.trim().replace(',', '.')) * CAPACITY_UNITS[unit]);
  return value.trim() !== '' && Number.isSafeInteger(bytes) && bytes >= 1024 ? bytes : null;
}
