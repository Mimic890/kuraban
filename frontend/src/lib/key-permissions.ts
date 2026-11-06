import type { BucketPermission } from '@/types';

export type Permissions = Pick<BucketPermission, 'read' | 'write' | 'owner'>;
export type PermissionFlag = keyof Permissions;
/** Only what the user touched, by bucket name, so a refresh can't undo the rest of a row. */
export type PermissionDraft = Record<string, Partial<Permissions>>;
export type PermissionChange = { bucketName: string; permissions: Permissions };
export type PermissionFailure = { bucketName: string; message: string };
type BucketRef = { id?: string; name: string };

/** Saving one bucket takes two Garage calls. Keeps some browser connections free for the rest of the app. */
const SAVES_AT_ONCE = 4;

export function savedPermissions(granted: BucketPermission[], bucket: BucketRef): Permissions {
  // A local alias of another bucket can have this name, so the id decides. Lists cached before ids were added lack it.
  const found = granted.find((p) => (bucket.id ? p.bucketId === bucket.id : p.bucketName === bucket.name));
  return { read: found?.read ?? false, write: found?.write ?? false, owner: found?.owner ?? false };
}

export function changedPermissions(buckets: BucketRef[], granted: BucketPermission[], draft: PermissionDraft): PermissionChange[] {
  const changes: PermissionChange[] = [];
  for (const bucket of buckets) {
    if (!draft[bucket.name]) continue;
    const saved = savedPermissions(granted, bucket);
    const permissions = { ...saved, ...draft[bucket.name] };
    if (permissions.read !== saved.read || permissions.write !== saved.write || permissions.owner !== saved.owner) {
      changes.push({ bucketName: bucket.name, permissions });
    }
  }
  return changes;
}

/** Tries every change even when some fail, and returns the ones that did. */
export async function saveAllLimited(
  changes: PermissionChange[],
  save: (change: PermissionChange) => Promise<void>,
): Promise<PermissionFailure[]> {
  const failures: PermissionFailure[] = [];
  const queue = [...changes];
  await Promise.all(
    Array.from({ length: SAVES_AT_ONCE }, async () => {
      for (let change = queue.shift(); change; change = queue.shift()) {
        try {
          await save(change);
        } catch (error) {
          const e = error as { response?: { data?: { error?: { message?: string } } }; message?: string };
          failures.push({ bucketName: change.bucketName, message: e.response?.data?.error?.message ?? e.message ?? '' });
        }
      }
    }),
  );
  return failures;
}
