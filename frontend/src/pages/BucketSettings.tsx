import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { BucketHistory } from '@/components/buckets/BucketHistory';
import { AlertTriangle, Eraser, Gauge, Info, Tags, Trash2 } from 'lucide-react';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useBuckets, useBucketAliasChange, useCleanupBucketUploads, useDeleteBucket, useUpdateBucketQuotas } from '@/hooks/useApi';
import { useCapabilities } from '@/hooks/useCapabilities';
import { useBucketCan } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { EmptyState } from '@/components/ui/empty-state';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { DeleteBucketDialog } from '@/components/buckets/DeleteBucketDialog';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Select, SelectOption } from '@/components/ui/select';
import { formatBytes } from '@/lib/file-utils';
import { formatDate as formatDateUtil } from '@/lib/utils';
import type { Bucket } from '@/types';
import { useTranslation } from '@/lib/i18n';
import {
  bytesToQuotaValue,
  quotaValueToBytes,
  QUOTA_UNIT_BYTES,
  type QuotaUnit,
} from '@/lib/quota-utils';

const formatBytesOrDash = (n?: number) => (n == null ? '—' : formatBytes(n));
const formatDateOrDash = (iso?: string) => (iso ? formatDateUtil(iso) : '—');

const quotaFormSchema = z
  .object({
    maxSizeEnabled: z.boolean(),
    maxSizeValue: z.string(),
    maxSizeUnit: z.enum(['MB', 'GB', 'TB']),
    maxObjectsEnabled: z.boolean(),
    maxObjectsValue: z.string(),
  })
  .superRefine((data, ctx) => {
    if (data.maxSizeEnabled) {
      const n = Number(data.maxSizeValue);
      if (!Number.isFinite(n) || !Number.isInteger(n) || n <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['maxSizeValue'],
          message: 'buckets.settings.validation.positive_whole_number',
        });
      }
    }
    if (data.maxObjectsEnabled) {
      const n = Number(data.maxObjectsValue);
      if (!Number.isFinite(n) || !Number.isInteger(n) || n <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['maxObjectsValue'],
          message: 'buckets.settings.validation.positive_whole_number',
        });
      }
    }
  });

type QuotaFormValues = z.infer<typeof quotaFormSchema>;

function deriveDefaults(quotas: { maxSize?: number; maxObjects?: number } | null | undefined): QuotaFormValues {
  const size = quotas?.maxSize;
  const objects = quotas?.maxObjects;
  if (size != null) {
    const { value, unit } = bytesToQuotaValue(size);
    return {
      maxSizeEnabled: true,
      maxSizeValue: String(value),
      maxSizeUnit: unit,
      maxObjectsEnabled: objects != null,
      maxObjectsValue: objects != null ? String(objects) : '',
    };
  }
  return {
    maxSizeEnabled: false,
    maxSizeValue: '',
    maxSizeUnit: 'GB',
    maxObjectsEnabled: objects != null,
    maxObjectsValue: objects != null ? String(objects) : '',
  };
}

export function BucketSettings() {
  const { t, language } = useTranslation();
  const { bucketName = '' } = useParams<{ bucketName: string }>();
  const navigate = useNavigate();
  const { data: buckets = [], isLoading, isFetching } = useBuckets();
  const bucket = buckets.find((b) => b.name === bucketName);
  const deleteMutation = useDeleteBucket();
  const updateQuotasMutation = useUpdateBucketQuotas();
  const canBucket = useBucketCan();
  const { data: capabilities } = useCapabilities();

  const [deleteOpen, setDeleteOpen] = useState(false);

  const defaults = useMemo(() => deriveDefaults(bucket?.quotas), [bucket?.quotas]);

  const {
    control,
    register,
    handleSubmit,
    watch,
    reset,
    formState: { errors, isDirty, isSubmitting },
  } = useForm<QuotaFormValues>({
    resolver: zodResolver(quotaFormSchema),
    values: defaults,
  });

  const watched = watch();

  const currentSize = bucket?.size ?? 0;
  const currentObjects = bucket?.objectCount ?? 0;

  const newMaxSizeBytes =
    watched.maxSizeEnabled && watched.maxSizeValue !== '' && !Number.isNaN(Number(watched.maxSizeValue))
      ? quotaValueToBytes(Number(watched.maxSizeValue), watched.maxSizeUnit)
      : null;
  const newMaxObjects =
    watched.maxObjectsEnabled && watched.maxObjectsValue !== '' && !Number.isNaN(Number(watched.maxObjectsValue))
      ? Number(watched.maxObjectsValue)
      : null;

  const sizeBelowCurrent =
    newMaxSizeBytes !== null && bucket?.size != null && newMaxSizeBytes < currentSize;
  const objectsBelowCurrent =
    newMaxObjects !== null && bucket?.objectCount != null && newMaxObjects < currentObjects;

  // The saved list can be older than the bucket, so wait before saying it does not exist.
  if (isLoading || (!bucket && isFetching)) {
    return <div className="px-7 py-6 text-[0.8438rem] text-[var(--muted-foreground)]">{t('buckets.common.loading')}</div>;
  }
  if (!bucket) {
    return (
      <div className="px-7 py-6">
        <EmptyState
          icon={<AlertTriangle />}
          tone="neutral"
          title={t('buckets.errors.not_found')}
          description={t('buckets.errors.not_found_description')}
        />
      </div>
    );
  }

  const onSubmit = handleSubmit(async (values) => {
    const maxSize = values.maxSizeEnabled
      ? quotaValueToBytes(Number(values.maxSizeValue), values.maxSizeUnit)
      : null;
    const maxObjects = values.maxObjectsEnabled ? Number(values.maxObjectsValue) : null;
    await updateQuotasMutation.mutateAsync({ bucketName: bucket.name, maxSize, maxObjects });
  });

  return (
    <div className="space-y-6 px-7 py-6">
      <BucketHistory bucket={bucket.name} />
      {/* Info */}
      <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
          <Info className="h-4 w-4 text-[var(--primary)]" />
          <h2 className="text-[0.9375rem] font-semibold">{t('buckets.settings.info.heading')}</h2>
        </header>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-4 px-5 py-5 sm:grid-cols-2">
          <Field label={t('buckets.fields.name')} value={<span className="font-mono text-[0.8438rem]">{bucket.name}</span>} />
          <Field label={t('buckets.fields.region')} value={bucket.region ?? '—'} />
          <Field label={t('buckets.fields.created')} value={formatDateOrDash(bucket.creationDate)} />
          <Field label={t('buckets.fields.objects')} value={bucket.objectCount != null ? bucket.objectCount.toLocaleString(language) : '—'} />
          <Field label={t('buckets.fields.size')} value={formatBytesOrDash(bucket.size)} />
          <Field
            label={t('buckets.fields.website')}
            value={
              <Badge variant={bucket.websiteAccess ? 'success' : 'neutral'}>
                {bucket.websiteAccess ? t('buckets.common.enabled') : t('buckets.common.disabled')}
              </Badge>
            }
          />
        </dl>
      </section>

      {/* Quotas */}
      <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
          <Gauge className="h-4 w-4 text-[var(--primary)]" />
          <h2 className="text-[0.9375rem] font-semibold">{t('buckets.settings.quotas.heading')}</h2>
        </header>

        <form onSubmit={onSubmit} className="space-y-6 px-5 py-5">
          {/* Max size row */}
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-3">
              <Controller
                control={control}
                name="maxSizeEnabled"
                render={({ field }) => (
                  <label className="flex items-center gap-2 text-[0.875rem]">
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                    <span>{t('buckets.settings.quotas.limit_size')}</span>
                  </label>
                )}
              />
              <Input
                type="number"
                min={1}
                step={1}
                className="w-32"
                disabled={!watched.maxSizeEnabled}
                {...register('maxSizeValue')}
              />
              <Controller
                control={control}
                name="maxSizeUnit"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    onChange={(v) => field.onChange(v as QuotaUnit)}
                    disabled={!watched.maxSizeEnabled}
                    className="w-24"
                  >
                    {(Object.keys(QUOTA_UNIT_BYTES) as QuotaUnit[]).map((u) => (
                      <SelectOption key={u} value={u}>
                        {u}
                      </SelectOption>
                    ))}
                  </Select>
                )}
              />
            </div>
            <p className="text-[0.8125rem] text-[var(--muted-foreground)]">
              {t('buckets.settings.quotas.current', { value: formatBytesOrDash(bucket.size) })}
            </p>
            {errors.maxSizeValue && (
              <p className="text-[0.8125rem] text-[var(--destructive)]">{t(errors.maxSizeValue.message!)}</p>
            )}
            {sizeBelowCurrent && (
              <p className="text-[0.8125rem] text-amber-600 dark:text-amber-400">
                {t('buckets.settings.quotas.size_exceeded', { size: formatBytes(currentSize) })}
              </p>
            )}
          </div>

          {/* Max objects row */}
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-3">
              <Controller
                control={control}
                name="maxObjectsEnabled"
                render={({ field }) => (
                  <label className="flex items-center gap-2 text-[0.875rem]">
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                    <span>{t('buckets.settings.quotas.limit_objects')}</span>
                  </label>
                )}
              />
              <Input
                type="number"
                min={1}
                step={1}
                className="w-40"
                disabled={!watched.maxObjectsEnabled}
                {...register('maxObjectsValue')}
              />
            </div>
            <p className="text-[0.8125rem] text-[var(--muted-foreground)]">
              {t('buckets.settings.quotas.current', { value: bucket.objectCount != null ? bucket.objectCount.toLocaleString(language) : '—' })}
            </p>
            {errors.maxObjectsValue && (
              <p className="text-[0.8125rem] text-[var(--destructive)]">{t(errors.maxObjectsValue.message!)}</p>
            )}
            {objectsBelowCurrent && (
              <p className="text-[0.8125rem] text-amber-600 dark:text-amber-400">
                {t('buckets.settings.quotas.objects_exceeded', { count: currentObjects.toLocaleString(language) })}
              </p>
            )}
          </div>

          <div className="flex items-center gap-3 border-t border-[var(--border)] pt-4">
            {/* Saving overwrites the quotas, so the form must not rest on an unconfirmed saved list. */}
            <Button type="submit" disabled={!isDirty || isSubmitting || isFetching}>
              {t('buckets.actions.save_changes')}
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => reset(defaults)}
              disabled={!isDirty || isSubmitting}
            >
              {t('buckets.actions.reset')}
            </Button>
          </div>
        </form>
      </section>

      {capabilities?.features.bucketAliases && <BucketAliases bucket={bucket} refreshing={isFetching} />}

      {capabilities?.features.cleanupUploads && canBucket(bucket, 'bucket.cleanup_uploads') && (
        <IncompleteUploads bucket={bucket} refreshing={isFetching} />
      )}

      {/* Danger zone */}
      {canBucket(bucket, 'bucket.delete') && (
        <section className="rounded-xl border border-[var(--danger-border)] bg-[var(--card)]">
          <header className="border-b border-[var(--danger-border)] px-5 py-3">
            <h2 className="text-[0.9375rem] font-semibold text-[var(--destructive)]">{t('buckets.settings.danger.heading')}</h2>
            <p className="mt-0.5 text-[0.8438rem] text-[var(--muted-foreground)]">
              {t('buckets.settings.danger.description')}
            </p>
          </header>
          <div className="flex items-center justify-between gap-4 px-5 py-4">
            <div className="min-w-0">
              <div className="text-[0.875rem] font-medium">{t('buckets.actions.delete_bucket')}</div>
              <div className="text-[0.8438rem] text-[var(--muted-foreground)]">
                {t('buckets.settings.danger.delete_description')}
              </div>
            </div>
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
              {t('buckets.actions.delete_bucket')}
            </Button>
          </div>
        </section>
      )}

      <DeleteBucketDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        bucketName={bucket.name}
        objectCount={bucket.objectCount ?? 0}
        onEmptyBucket={async () => {
          const { bucketsApi } = await import('@/lib/api');
          await bucketsApi.emptyBucket(bucket.name);
        }}
        onDeleteBucket={async () => {
          await deleteMutation.mutateAsync(bucket.name);
          navigate('/buckets');
        }}
      />
    </div>
  );
}

function BucketAliases({ bucket, refreshing }: { bucket: Bucket; refreshing: boolean }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const canBucket = useBucketCan();
  const change = useBucketAliasChange();
  const [alias, setAlias] = useState('');
  const [removing, setRemoving] = useState<string | null>(null);

  const aliases = bucket.globalAliases ?? [bucket.name];
  const newAlias = alias.trim();
  // Garage lists aliases alphabetically and Kuraban calls the bucket by the first.
  const becomesName = newAlias !== '' && newAlias < aliases[0];

  const run = (kind: 'add' | 'remove', value: string, done?: () => void) =>
    change.mutate(
      { kind, bucketName: bucket.name, alias: value },
      {
        onSuccess: ({ name }) => {
          done?.();
          // The page's address is the bucket's name, which this change may have replaced.
          if (name !== bucket.name) navigate(`/buckets/${encodeURIComponent(name)}/settings`, { replace: true });
        },
        onSettled: () => setRemoving(null),
      }
    );

  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
      <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
        <Tags className="h-4 w-4 text-[var(--primary)]" />
        <h2 className="text-[0.9375rem] font-semibold">{t('buckets.settings.aliases.heading')}</h2>
      </header>
      <div className="space-y-4 px-5 py-5">
        <p className="text-[0.8125rem] text-[var(--muted-foreground)]">{t('buckets.settings.aliases.description')}</p>
        <ul className="divide-y divide-[var(--border)] rounded-lg border border-[var(--border)]">
          {aliases.map((name, i) => (
            <li key={name} className="flex min-h-11 items-center justify-between gap-3 px-3 py-1.5">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="break-all font-mono text-[0.8438rem]">{name}</span>
                {i === 0 && <Badge variant="primary">{t('buckets.settings.aliases.main')}</Badge>}
              </div>
              {aliases.length > 1 && canBucket(bucket, 'bucket_alias.remove') && (
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label={t('buckets.settings.aliases.remove_label', { alias: name })}
                  disabled={refreshing || change.isPending}
                  onClick={() => setRemoving(name)}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              )}
            </li>
          ))}
        </ul>
        {bucket.localAliases && bucket.localAliases.length > 0 && (
          <p className="break-words text-[0.8125rem] text-[var(--muted-foreground)]">
            {t('buckets.settings.aliases.local', { aliases: bucket.localAliases.join(', ') })}
          </p>
        )}
        {canBucket(bucket, 'bucket_alias.add') && (
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              run('add', newAlias, () => setAlias(''));
            }}
          >
            <div className="flex flex-wrap items-center gap-3">
              <Input
                className="w-64 max-w-full font-mono"
                value={alias}
                onChange={(e) => setAlias(e.target.value)}
                placeholder="new-alias"
                aria-label={t('buckets.settings.aliases.new')}
              />
              <Button type="submit" variant="secondary" disabled={newAlias === '' || refreshing || change.isPending}>
                {t('buckets.settings.aliases.add')}
              </Button>
            </div>
            {becomesName && (
              <p className="text-[0.8125rem] text-amber-600 dark:text-amber-400">
                {t('buckets.settings.aliases.becomes_name', { alias: newAlias })}
              </p>
            )}
          </form>
        )}
      </div>
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={t('buckets.settings.aliases.remove_title', { alias: removing ?? '' })}
        description={t('buckets.settings.aliases.remove_description')}
        note={
          removing === aliases[0]
            ? t('buckets.settings.aliases.remove_main_note', { alias: aliases[1] ?? '' })
            : t('buckets.settings.aliases.remove_note')
        }
        confirmLabel={t('buckets.settings.aliases.remove')}
        loading={change.isPending}
        onConfirm={() => {
          if (removing) run('remove', removing);
        }}
      />
    </section>
  );
}

// The server refuses anything under an hour, see minCleanupAgeSecs in handlers/buckets.go.
const CLEANUP_AGES = [
  { secs: 3600, label: 'buckets.settings.uploads.age_hour' },
  { secs: 86400, label: 'buckets.settings.uploads.age_day' },
  { secs: 604800, label: 'buckets.settings.uploads.age_week' },
];

function IncompleteUploads({ bucket, refreshing }: { bucket: Bucket; refreshing: boolean }) {
  const { t, language } = useTranslation();
  const cleanup = useCleanupBucketUploads();
  const [ageSecs, setAgeSecs] = useState(86400);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const count = bucket.unfinishedUploads;
  const totals = {
    count: count != null ? count.toLocaleString(language) : '—',
    size: formatBytesOrDash(bucket.unfinishedUploadBytes),
  };
  const ageLabel = t(CLEANUP_AGES.find((a) => a.secs === ageSecs)!.label);

  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
      <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
        <Eraser className="h-4 w-4 text-[var(--primary)]" />
        <h2 className="text-[0.9375rem] font-semibold">{t('buckets.settings.uploads.heading')}</h2>
      </header>
      <div className="space-y-4 px-5 py-5">
        <div>
          <p className="text-[0.875rem] font-medium">{count === 0 ? t('buckets.settings.uploads.none') : t('buckets.settings.uploads.summary', totals)}</p>
          <p className="mt-1 text-[0.8125rem] text-[var(--muted-foreground)]">{t('buckets.settings.uploads.description')}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <label className="flex items-center gap-2 text-[0.875rem]">
            <span>{t('buckets.settings.uploads.older_than')}</span>
            <Select value={String(ageSecs)} onChange={(v) => setAgeSecs(Number(v))} className="w-32">
              {CLEANUP_AGES.map((a) => (
                <SelectOption key={a.secs} value={String(a.secs)}>
                  {t(a.label)}
                </SelectOption>
              ))}
            </Select>
          </label>
          {/* The saved list can be out of date, so act only on a confirmed count. */}
          <Button variant="secondary" disabled={refreshing || count === 0} onClick={() => setConfirmOpen(true)}>
            {t('buckets.settings.uploads.action')}
          </Button>
        </div>
      </div>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('buckets.settings.uploads.confirm_title')}
        description={t('buckets.settings.uploads.confirm_description', { bucket: bucket.name, age: ageLabel, ...totals })}
        confirmLabel={t('buckets.settings.uploads.action')}
        icon={<Eraser />}
        loading={cleanup.isPending}
        onConfirm={() =>
          cleanup.mutate(
            { bucketName: bucket.name, olderThanSecs: ageSecs },
            { onSettled: () => setConfirmOpen(false) }
          )
        }
      />
    </section>
  );
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-[0.6875rem] font-medium uppercase tracking-[0.08em] text-[var(--muted-foreground)]">{label}</dt>
      <dd className="mt-1 text-[0.875rem]">{value}</dd>
    </div>
  );
}
