import { describe, expect, it } from 'vitest';
import { sortWorkers } from './helpers';
import type { WorkerInfo } from '@/types';

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
