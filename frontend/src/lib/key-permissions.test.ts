import { describe, expect, it } from 'vitest';
import { changedPermissions, saveAllLimited, savedPermissions } from './key-permissions';

const grant = (bucketId: string, bucketName: string, read = true) => ({ bucketId, bucketName, read, write: false, owner: false });

describe('savedPermissions', () => {
  it('goes by id, since a local alias of another bucket can share the name', () => {
    expect(savedPermissions([grant('other', 'photos')], { id: 'mine', name: 'photos' }).read).toBe(false);
    expect(savedPermissions([grant('mine', 'old-name')], { id: 'mine', name: 'photos' }).read).toBe(true);
  });

  it('falls back to the name for a list cached without ids', () => {
    expect(savedPermissions([grant('x', 'photos')], { name: 'photos' }).read).toBe(true);
  });
});

describe('changedPermissions', () => {
  const buckets = [{ id: 'a', name: 'alpha' }, { id: 'b', name: 'beta' }];

  it('keeps what the user did not touch from the saved state', () => {
    expect(changedPermissions(buckets, [grant('a', 'alpha')], { alpha: { write: true } })).toEqual([
      { bucketName: 'alpha', permissions: { read: true, write: true, owner: false } },
    ]);
  });

  it('leaves out rows toggled back, or saved in the meantime', () => {
    expect(changedPermissions(buckets, [grant('a', 'alpha')], { alpha: { read: true }, beta: { read: false } })).toEqual([]);
  });

  it('leaves out buckets no longer listed', () => {
    expect(changedPermissions(buckets, [], { gone: { read: true } })).toEqual([]);
  });
});

describe('saveAllLimited', () => {
  const change = (bucketName: string) => ({ bucketName, permissions: { read: true, write: false, owner: false } });

  it('tries the rest after a failure and reports why', async () => {
    const tried: string[] = [];
    const failures = await saveAllLimited(['a', 'b', 'c', 'd', 'e', 'f'].map(change), async ({ bucketName }) => {
      tried.push(bucketName);
      if (bucketName === 'b') throw { response: { data: { error: { message: 'denied' } } } };
    });
    expect(tried).toHaveLength(6);
    expect(failures).toEqual([{ bucketName: 'b', message: 'denied' }]);
  });

  it('never runs more than a few at once', async () => {
    let running = 0;
    let peak = 0;
    await saveAllLimited(Array.from({ length: 20 }, (_, i) => change(String(i))), async () => {
      peak = Math.max(peak, ++running);
      await Promise.resolve();
      running--;
    });
    expect(peak).toBeLessThanOrEqual(4);
  });
});
