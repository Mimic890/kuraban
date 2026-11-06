import { useMemo, useState } from 'react';
import { Edit, Loader2, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { IconTile } from '@/components/ui/icon-tile';
import { Input } from '@/components/ui/input';
import { useBuckets, useSaveKeyPermissions } from '@/hooks/useApi';
import { useBucketCan } from '@/hooks/usePermissions';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import type { AccessKey, Bucket } from '@/types';
import { changedPermissions, savedPermissions, type PermissionDraft, type PermissionFlag } from '@/lib/key-permissions';

const NO_BUCKETS: Bucket[] = [];
const FLAGS: { flag: PermissionFlag; label: string }[] = [
  { flag: 'read', label: 'access_control.permission_read_short' },
  { flag: 'write', label: 'access_control.permission_write_short' },
  { flag: 'owner', label: 'access_control.permission_owner_short' },
];

export function KeyPermissionsDialog({
  accessKey,
  keysFetching,
  onClose,
}: {
  accessKey: AccessKey;
  keysFetching: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const canBucket = useBucketCan();
  const { data: buckets = NO_BUCKETS, isLoading, isFetching } = useBuckets();
  const save = useSaveKeyPermissions();
  const [search, setSearch] = useState('');
  const [draft, setDraft] = useState<PermissionDraft>({});
  const [failures, setFailures] = useState<Record<string, string>>({});

  const granted = accessKey.permissions;
  const changes = changedPermissions(buckets, granted, draft);
  const changedNames = new Set(changes.map((c) => c.bucketName));

  // Buckets the key can already reach come first, so nobody has to hunt for them among hundreds.
  const sorted = useMemo(() => {
    const hasAccess = new Set(
      buckets.filter((b) => Object.values(savedPermissions(granted, b)).some(Boolean)).map((b) => b.name),
    );
    return [...buckets].sort((a, b) => Number(hasAccess.has(b.name)) - Number(hasAccess.has(a.name)) || a.name.localeCompare(b.name));
  }, [buckets, granted]);
  const query = search.trim().toLowerCase();
  const rows = query ? sorted.filter((b) => b.name.toLowerCase().includes(query)) : sorted;

  // Local-only buckets, and those outside the viewer's teams, have no row. Saving never touches them.
  const hiddenCount = isLoading ? 0 : granted.filter((p) => !buckets.some((b) => b.id === p.bucketId)).length;

  const onSave = () =>
    save.mutate(
      { accessKeyId: accessKey.accessKeyId, changes },
      {
        onSuccess: (failed) => {
          if (failed.length === 0) {
            onClose();
            return;
          }
          // Only the failed rows stay marked as changed, ready for another try.
          setFailures(Object.fromEntries(failed.map((f) => [f.bucketName, f.message])));
          setDraft((d) => Object.fromEntries(failed.map((f) => [f.bucketName, d[f.bucketName]])));
        },
      },
    );

  return (
    <Dialog open onOpenChange={(open) => !open && !save.isPending && onClose()} size="form">
      <DialogContent>
        <DialogHeader>
          <IconTile icon={<Edit />} tone="primary" size="md" />
          <div className="min-w-0 flex-1 pr-6">
            <DialogTitle className="truncate">{t('access_control.permissions_dialog_title', { name: accessKey.name })}</DialogTitle>
            <DialogDescription>{t('access_control.permissions_dialog_description')}</DialogDescription>
          </div>
        </DialogHeader>
        <div className="shrink-0 border-b border-[var(--border)] px-6 py-3">
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--muted-foreground)]" />
            <Input
              className="pl-9"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('access_control.search_buckets_placeholder')}
              aria-label={t('access_control.search_buckets_placeholder')}
            />
          </div>
        </div>
        <DialogBody className="px-0 py-0 sm:min-h-[16rem]">
          {isLoading ? (
            <div className="flex justify-center py-10 text-[var(--muted-foreground)]">
              <Loader2 className="h-5 w-5 animate-spin" />
            </div>
          ) : rows.length === 0 ? (
            <p className="px-6 py-10 text-center text-[0.8438rem] text-[var(--muted-foreground)]">
              {t(query ? 'access_control.no_buckets_found' : 'access_control.no_buckets_available')}
            </p>
          ) : (
            <table className="w-full table-fixed text-[0.8438rem]">
              <thead className="sticky top-0 z-10 bg-[var(--card)] text-[0.75rem] text-[var(--muted-foreground)] shadow-[inset_0_-1px_0_var(--border)]">
                <tr>
                  <th className="px-6 py-2 text-left font-medium">{t('access_control.bucket_label')}</th>
                  {FLAGS.map(({ flag, label }) => (
                    <th key={flag} className="w-[3.75rem] py-2 text-center font-medium last:w-[4.75rem] last:pr-3 sm:w-20 sm:last:w-24">
                      {t(label)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((bucket) => {
                  const saved = savedPermissions(granted, bucket);
                  const editable = canBucket(bucket, 'permission.allow_bucket_key') && canBucket(bucket, 'permission.deny_bucket_key');
                  const failure = changedNames.has(bucket.name) ? failures[bucket.name] : undefined;
                  return (
                    <tr
                      key={bucket.name}
                      className={cn('border-b border-[var(--border)] last:border-b-0', changedNames.has(bucket.name) && 'bg-[var(--accent)]')}
                    >
                      <td className="px-6 py-2">
                        <div className="break-all font-mono">{bucket.name}</div>
                        {failure && <div className="mt-0.5 break-words text-[0.75rem] text-[var(--destructive)]">{failure}</div>}
                      </td>
                      {FLAGS.map(({ flag, label }) => (
                        <td key={flag} className="py-2 text-center last:pr-3">
                          <Checkbox
                            className="align-middle"
                            aria-label={`${bucket.name}: ${t(label)}`}
                            checked={draft[bucket.name]?.[flag] ?? saved[flag]}
                            disabled={!editable || save.isPending}
                            onCheckedChange={(value) => setDraft((d) => ({ ...d, [bucket.name]: { ...d[bucket.name], [flag]: value } }))}
                          />
                        </td>
                      ))}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </DialogBody>
        {hiddenCount > 0 && (
          <p className="shrink-0 border-t border-[var(--border)] px-6 py-2.5 text-[0.7812rem] text-[var(--muted-foreground)]">
            {t('access_control.hidden_buckets_note', { count: hiddenCount })}
          </p>
        )}
        <DialogFooter>
          <Button variant="secondary" disabled={save.isPending} onClick={onClose}>
            {t('common.cancel')}
          </Button>
          {/* Saving writes rows built on the listed state, so wait until both lists are confirmed. */}
          <Button onClick={onSave} disabled={changes.length === 0 || save.isPending || isFetching || keysFetching}>
            {save.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
            {save.isPending
              ? t('access_control.saving_permissions')
              : changes.length > 0
                ? t('access_control.save_permissions_count', { count: changes.length })
                : t('access_control.save_permissions_button')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
