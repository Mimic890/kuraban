import { useEffect, useState } from 'react';
import { useNavigate, useParams, Link } from 'react-router-dom';
import { objectsApi } from '@/lib/api';
import { useBuckets } from '@/hooks/useApi';
import { useBucketCan } from '@/hooks/usePermissions';
import type { ObjectMetadata } from '@/types';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { IconTile } from '@/components/ui/icon-tile';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { ObjectPreview } from '@/components/buckets/ObjectPreview';
import { ArrowLeft, ChevronRight, Copy, Download, File, Loader2, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { downloadObject, formatBytes } from '@/lib/file-utils';
import { formatDate } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import { copyText } from '@/lib/clipboard';

function CardSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)]">
      <div className="border-b border-[var(--border)] px-5 py-3.5">
        <h3 className="text-[0.875rem] font-semibold tracking-[-0.01em]">{title}</h3>
      </div>
      {children}
    </section>
  );
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-1 gap-1 px-5 py-3.5 sm:grid-cols-[200px_1fr] sm:gap-4">
      <dt className="text-[0.7812rem] font-medium text-[var(--muted-foreground)]">{label}</dt>
      <dd className="text-[0.8438rem] text-[var(--foreground)] break-words">{children}</dd>
    </div>
  );
}

export function ObjectDetailsView() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { bucketName, '*': objectKey } = useParams();

  const { data: buckets = [] } = useBuckets();
  const bucket = buckets.find((b) => b.name === bucketName);
  const canBucket = useBucketCan();
  const canDelete = canBucket(bucket, 'object.delete');
  const canRead = canBucket(bucket, 'object.read');

  const [metadata, setMetadata] = useState<ObjectMetadata | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!bucketName || !objectKey) {
      setError('buckets.object_details.errors.identifiers_required');
      setIsLoading(false);
      return;
    }
    let cancelled = false;
    const ctrl = new AbortController();
    const fetchMetadata = async () => {
      try {
        setIsLoading(true);
        setError(null);
        const data = await objectsApi.getMetadata(bucketName, objectKey, { signal: ctrl.signal });
        if (cancelled) return;
        setMetadata(data);
      } catch {
        if (cancelled) return;
        setError('buckets.object_details.errors.metadata_failed');
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    };
    fetchMetadata();
    return () => { cancelled = true; ctrl.abort(); };
  }, [bucketName, objectKey]);

  const parentPath = objectKey?.split('/').slice(0, -1).join('/') ?? '';
  const fileName = objectKey?.split('/').pop() || objectKey || '';
  const backHref = `/buckets/${bucketName}/objects${parentPath ? `?prefix=${encodeURIComponent(parentPath + '/')}` : ''}`;
  const pathSegments = parentPath ? parentPath.split('/').filter(Boolean) : [];

  const copy = (text: string, label: string) => { copyText(text, label); };

  const handleDownload = () => {
    if (!bucketName || !objectKey) return;
    downloadObject(bucketName, objectKey);
  };

  const handleDelete = async () => {
    if (!bucketName || !objectKey) return;
    try {
      setDeleting(true);
      await objectsApi.delete(bucketName, objectKey);
      toast.success(t('buckets.toast.object_deleted'));
      navigate(backHref);
    } catch {
      // error toast handled by axios interceptor
    } finally {
      setDeleting(false);
      setDeleteOpen(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center gap-2 text-[var(--muted-foreground)]">
        <Loader2 className="h-4 w-4 animate-spin" /> {t('buckets.object_details.loading')}
      </div>
    );
  }

  if (error || !metadata) {
    return (
      <div className="px-7 py-6">
        <Button variant="secondary" onClick={() => navigate(backHref)} className="mb-4">
          <ArrowLeft className="h-4 w-4" /> {t('buckets.actions.back')}
        </Button>
        <div className="rounded-xl border border-[var(--danger-border)] bg-[var(--danger-soft)] px-5 py-4 text-[0.8438rem] text-[var(--destructive)]">
          {error ? t(error) : t('buckets.object_details.errors.not_found')}
        </div>
      </div>
    );
  }

  return (
    <div className="px-7 py-6 space-y-6">
      {/* Back + breadcrumb */}
      <div className="flex items-center gap-2 text-[0.8125rem] text-[var(--muted-foreground)]">
        <Link
          to={backHref}
          className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 hover:bg-[var(--accent)] hover:text-[var(--foreground)]"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          {t('buckets.tabs.objects')}
        </Link>
        {pathSegments.map((seg, i) => (
          <span key={i} className="inline-flex items-center gap-1">
            <ChevronRight className="h-3.5 w-3.5 opacity-50" />
            <span className="font-mono">{seg}</span>
          </span>
        ))}
        <ChevronRight className="h-3.5 w-3.5 opacity-50" />
        <span className="truncate font-mono text-[var(--foreground)]">{fileName}</span>
      </div>

      {/* Hero */}
      <section className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <IconTile icon={<File />} tone="primary" size="lg" />
          <div className="min-w-0">
            <h1 className="truncate text-[1.375rem] font-semibold tracking-[-0.02em]">{fileName}</h1>
            <button
              type="button"
              onClick={() => copy(metadata.key, t('buckets.toast.object_key_copied'))}
              title={t('buckets.actions.copy_key')}
              className="group mt-1 inline-flex max-w-full items-center gap-1.5 truncate font-mono text-[0.8125rem] text-[var(--muted-foreground)] hover:text-[var(--foreground)]"
            >
              <span className="truncate">{metadata.key}</span>
              <Copy className="h-3 w-3 flex-shrink-0 opacity-60 group-hover:opacity-100" />
            </button>
            <div className="mt-2 flex flex-wrap gap-1.5">
              <Badge>{formatBytes(metadata.size)}</Badge>
              <Badge>{metadata.contentType || 'application/octet-stream'}</Badge>
              {metadata.storageClass && <Badge>{metadata.storageClass}</Badge>}
            </div>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="secondary" onClick={handleDownload}>
            <Download className="h-4 w-4" /> {t('buckets.actions.download')}
          </Button>
          {canDelete && (
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
              <Trash2 className="h-4 w-4" /> {t('buckets.actions.delete')}
            </Button>
          )}
        </div>
      </section>

      {/* Details */}
      <CardSection title={t('buckets.object_details.sections.details')}>
        <dl className="divide-y divide-[var(--border)]">
          <DetailRow label={t('buckets.fields.size')}>{formatBytes(metadata.size)}</DetailRow>
          <DetailRow label={t('buckets.fields.content_type')}>{metadata.contentType || 'application/octet-stream'}</DetailRow>
          <DetailRow label={t('buckets.fields.storage_class')}>{metadata.storageClass || t('buckets.storage_class.standard')}</DetailRow>
          <DetailRow label={t('buckets.fields.last_modified')}>{formatDate(metadata.lastModified)}</DetailRow>
          <DetailRow label={t('buckets.fields.etag')}>
            <button
              type="button"
              onClick={() => copy(metadata.etag, t('buckets.toast.etag_copied'))}
              className="inline-flex max-w-full items-center gap-1.5 truncate rounded-md bg-[var(--surface-sunken)] px-2 py-0.5 font-mono text-[0.7812rem] hover:bg-[var(--accent)]"
            >
              <span className="truncate">{metadata.etag}</span>
              <Copy className="h-3 w-3 flex-shrink-0 opacity-60" />
            </button>
          </DetailRow>
          {metadata.versionId && (
            <DetailRow label={t('buckets.fields.version_id')}>
              <span className="font-mono text-[0.7812rem]">{metadata.versionId}</span>
            </DetailRow>
          )}
        </dl>
      </CardSection>

      {/* Custom metadata */}
      {metadata.metadata && Object.keys(metadata.metadata).length > 0 && (
        <CardSection title={t('buckets.object_details.sections.custom_metadata')}>
          <dl className="divide-y divide-[var(--border)]">
            {Object.entries(metadata.metadata).map(([key, value]) => (
              <DetailRow key={key} label={key}>
                <span className="font-mono text-[0.7812rem]">{value}</span>
              </DetailRow>
            ))}
          </dl>
        </CardSection>
      )}

      {/* Preview */}
      <CardSection title={t('buckets.object_details.sections.preview')}>
        {canRead && bucketName && objectKey ? (
          <ObjectPreview
            bucket={bucketName}
            objectKey={objectKey}
            size={metadata.size}
            contentType={metadata.contentType}
            onDownload={handleDownload}
          />
        ) : (
          <div className="px-5 py-10 text-center text-[0.8125rem] text-[var(--muted-foreground)]">
            {t('buckets.preview.unavailable')}
          </div>
        )}
      </CardSection>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('buckets.object_details.delete.title', { name: fileName })}
        description={t('buckets.object_details.delete.description')}
        confirmLabel={t('buckets.actions.delete_object')}
        loading={deleting}
        onConfirm={handleDelete}
      />
    </div>
  );
}
