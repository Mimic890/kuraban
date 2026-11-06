import { describe, expect, it } from 'vitest';
import { capacityToInput, hasStagedChanges, inputToCapacity, layoutKey, layoutRows, sortWorkers } from './helpers';
import type { ClusterLayout, ClusterNode, WorkerInfo } from '@/types';

const worker = (name: string, extra: Partial<WorkerInfo> = {}): WorkerInfo => ({
  id: 0,
  name,
  state: 'idle',
  errors: 0,
  consecutiveErrors: 0,
  freeform: [],
  ...extra,
});

describe('sortWorkers', () => {
  it('puts failing and active workers first, then sorts by name with numbers in order', () => {
    const sorted = sortWorkers([
      worker('worker #10'),
      worker('worker #2'),
      worker('old errors', { errors: 3 }),
      worker('throttled', { state: { throttled: { durationSecs: 1 } } }),
      worker('busy', { state: 'busy' }),
      worker('failing now', { errors: 1, consecutiveErrors: 1 }),
    ]);
    expect(sorted.map((w) => w.name)).toEqual(['failing now', 'old errors', 'busy', 'throttled', 'worker #2', 'worker #10']);
  });
});

const layout = (extra: Partial<ClusterLayout> = {}): ClusterLayout => ({
  version: 3,
  roles: [],
  parameters: { zoneRedundancy: 'maximum' },
  partitionSize: 0,
  stagedRoleChanges: [],
  stagedParameters: null,
  ...extra,
});

describe('hasStagedChanges', () => {
  it('ignores a staged parameter equal to the current one', () => {
    expect(hasStagedChanges(layout())).toBe(false);
    expect(hasStagedChanges(layout({ stagedParameters: { zoneRedundancy: 'maximum' } }))).toBe(false);
  });

  it('sees staged parameters and roles', () => {
    expect(hasStagedChanges(layout({ stagedParameters: { zoneRedundancy: { atLeast: 2 } } }))).toBe(true);
    expect(hasStagedChanges(layout({ stagedRoleChanges: [{ id: 'a', remove: true }] }))).toBe(true);
  });
});

describe('layoutKey', () => {
  it('changes with the version, the roles and the parameters', () => {
    const base = layoutKey(layout());
    expect(layoutKey(layout())).toBe(base);
    expect(layoutKey(layout({ version: 4 }))).not.toBe(base);
    expect(layoutKey(layout({ stagedRoleChanges: [{ id: 'a', remove: true }] }))).not.toBe(base);
    expect(layoutKey(layout({ stagedParameters: { zoneRedundancy: { atLeast: 1 } } }))).not.toBe(base);
  });
});

describe('layoutRows', () => {
  it('lists every node once, known, assigned or staged, with zones first', () => {
    const role = (id: string, zone: string) => ({ id, zone, tags: [], capacity: 1, storedPartitions: 1, usableCapacity: 1 });
    const rows = layoutRows(
      [{ id: 'idle' }, { id: 'b' }] as ClusterNode[],
      layout({ roles: [role('b', 'dc2'), role('a', 'dc1')], stagedRoleChanges: [{ id: 'new', zone: 'dc3', tags: [], capacity: null }] }),
    );
    expect(rows.map((r) => r.id)).toEqual(['a', 'b', 'idle', 'new']);
    expect(rows[0].node).toBeUndefined();
    expect(rows[3].change).toBeDefined();
  });
});

describe('capacity input', () => {
  it('round-trips the units it offers', () => {
    expect(capacityToInput(10 * 1024 ** 3)).toEqual({ value: '10', unit: 'GiB' });
    expect(capacityToInput(1.5 * 1024 ** 4)).toEqual({ value: '1.5', unit: 'TiB' });
    expect(inputToCapacity('10', 'GiB')).toBe(10 * 1024 ** 3);
    expect(inputToCapacity('1,5', 'TiB')).toBe(1.5 * 1024 ** 4);
  });

  it('refuses what Garage would refuse', () => {
    for (const bad of ['', ' ', 'abc', '-1', '0', '1e30']) {
      expect(inputToCapacity(bad, 'TiB')).toBeNull();
    }
  });
});
