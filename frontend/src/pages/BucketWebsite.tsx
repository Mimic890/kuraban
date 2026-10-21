import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Globe } from 'lucide-react';
import { useBuckets } from '@/hooks/useApi';
import { bucketsApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { EmptyState } from '@/components/ui/empty-state';
import { toast } from 'sonner';
import { useTranslation } from '@/lib/i18n';

export function BucketWebsite() {
  const { t } = useTranslation();
  const { bucketName = '' } = useParams<{ bucketName: string }>();
  const queryClient = useQueryClient();
  const { data: buckets = [], isLoading } = useBuckets();
  const bucket = buckets.find((b) => b.name === bucketName);

  const [enabled, setEnabled] = useState(false);
  const [indexDocument, setIndexDocument] = useState('index.html');
  const [errorDocument, setErrorDocument] = useState('');
  const [saving, setSaving] = useState(false);

  // Sync local form state whenever the underlying bucket changes.
  useEffect(() => {
    if (!bucket) return;
    setEnabled(bucket.websiteAccess);
    setIndexDocument(bucket.websiteConfig?.indexDocument ?? 'index.html');
    setErrorDocument(bucket.websiteConfig?.errorDocument ?? '');
  }, [bucket?.name, bucket?.websiteAccess, bucket?.websiteConfig?.indexDocument, bucket?.websiteConfig?.errorDocument]);

  if (isLoading) {
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

  const wasEnabled = bucket.websiteAccess;
  const disabling = wasEnabled && !enabled;

  const handleReset = () => {
    setEnabled(bucket.websiteAccess);
    setIndexDocument(bucket.websiteConfig?.indexDocument ?? 'index.html');
    setErrorDocument(bucket.websiteConfig?.errorDocument ?? '');
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await bucketsApi.updateBucketWebsite(bucketName, {
        enabled,
        indexDocument: enabled ? indexDocument : undefined,
        errorDocument: enabled && errorDocument ? errorDocument : undefined,
      });
      await queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      toast.success(disabling ? t('buckets.website.toast.disabled') : t('buckets.website.toast.updated'));
    } catch {
      // error toast handled by axios interceptor
    } finally {
      setSaving(false);
    }
  };

  const saveDisabled = saving || (enabled && !indexDocument);

  return (
    <div className="px-7 py-6">
      <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
          <Globe className="h-4 w-4 text-[var(--primary)]" />
          <h2 className="text-[0.9375rem] font-semibold">{t('buckets.website.heading')}</h2>
        </header>

        <div className="space-y-6 px-5 py-5">
          <div className="flex items-center justify-between gap-4">
            <div>
              <p className="text-[0.875rem] font-medium">{t('buckets.website.access.label')}</p>
              <p className="mt-0.5 text-[0.7812rem] text-[var(--muted-foreground)]">
                {t('buckets.website.access.description')}
              </p>
            </div>
            <div className="flex items-center gap-3">
              <Badge variant={enabled ? 'primary' : 'neutral'}>
                {enabled ? t('buckets.common.enabled') : t('buckets.common.disabled')}
              </Badge>
              <Switch checked={enabled} onCheckedChange={setEnabled} />
            </div>
          </div>

          {enabled && (
            <div className="space-y-4">
              <div className="space-y-2">
                <label className="text-[0.8438rem] font-medium">
                  {t('buckets.website.index_document.label')} <span className="text-[var(--destructive)]">*</span>
                </label>
                <Input
                  value={indexDocument}
                  onChange={(e) => setIndexDocument(e.target.value)}
                  placeholder={t('buckets.website.index_document.placeholder')}
                />
                <p className="text-[0.7812rem] text-[var(--muted-foreground)]">
                  {t('buckets.website.index_document.description')}
                </p>
              </div>

              <div className="space-y-2">
                <label className="text-[0.8438rem] font-medium">{t('buckets.website.error_document.label')}</label>
                <Input
                  value={errorDocument}
                  onChange={(e) => setErrorDocument(e.target.value)}
                  placeholder={t('buckets.website.error_document.placeholder')}
                />
                <p className="text-[0.7812rem] text-[var(--muted-foreground)]">
                  {t('buckets.website.error_document.description')}
                </p>
              </div>
            </div>
          )}
        </div>

        <footer className="flex justify-end gap-2 border-t border-[var(--border)] bg-[var(--surface-sunken)] px-5 py-3">
          <Button variant="secondary" onClick={handleReset} disabled={saving}>{t('buckets.actions.reset')}</Button>
          <Button
            onClick={handleSave}
            variant={disabling ? 'destructive' : 'primary'}
            disabled={saveDisabled}
          >
            {saving ? t('buckets.actions.saving') : disabling ? t('buckets.website.actions.disable') : t('buckets.actions.save_changes')}
          </Button>
        </footer>
      </section>
    </div>
  );
}
