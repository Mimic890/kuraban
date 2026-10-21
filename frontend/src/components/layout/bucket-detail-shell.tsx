import { NavLink, Outlet, useParams, useLocation } from 'react-router-dom';
import { Database, Copy, Upload } from 'lucide-react';
import { IconTile } from '@/components/ui/icon-tile';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useBuckets } from '@/hooks/useApi';
import { useBucketCan } from '@/hooks/usePermissions';
import { useTranslation } from '@/lib/i18n';
import { formatBytes } from '@/lib/file-utils';
import { copyText } from '@/lib/clipboard';

interface TabSpec {
  to: string;
  label: string;
  end?: boolean;
  perms?: string[];
}

const tabs: TabSpec[] = [
  { to: 'objects', label: 'buckets.tabs.objects', perms: ['object.list'] },
  { to: 'permissions', label: 'buckets.tabs.permissions', perms: ['permission.allow_bucket_key', 'permission.deny_bucket_key'] },
  { to: 'website', label: 'buckets.tabs.website', perms: ['bucket.update'] },
  { to: 'settings', label: 'buckets.tabs.settings', perms: ['bucket.update'] },
];

export function BucketDetailShell() {
  const { t, language } = useTranslation();
  const { bucketName = '' } = useParams<{ bucketName: string }>();
  const location = useLocation();
  const { data: buckets = [], isFetched } = useBuckets();
  const bucket = buckets.find((b) => b.name === bucketName);
  const canBucket = useBucketCan();
  const visibleTabs = tabs.filter((t) => !t.perms || t.perms.every((p) => canBucket(bucket, p)));
  const isObjectsTab = location.pathname.endsWith('/objects') || location.pathname.includes('/objects/');

  // Show not-found state when buckets have been loaded but no match was found.
  if (isFetched && !bucket) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[400px] gap-4 px-7 py-12 text-center">
        <Database className="h-12 w-12 text-muted-foreground opacity-50" />
        <h2 className="text-xl font-semibold">{t('buckets.errors.not_found')}</h2>
        <p className="text-sm text-muted-foreground max-w-md">
          {t('buckets.errors.named_not_found_description', { bucket: bucketName })}
        </p>
      </div>
    );
  }

  const s3Url = `s3://${bucketName}`;
  const copyUrl = () => {
    copyText(s3Url, t('buckets.toast.url_copied'));
  };

  return (
    <div className="flex flex-col">
      {/* Hero */}
      <section className="px-7 pt-6 pb-5">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <IconTile icon={<Database />} tone="primary" size="lg" />
            <div className="min-w-0">
              <h1 className="truncate text-[1.625rem] font-semibold tracking-[-0.02em]">{bucketName}</h1>
              <p className="mt-1 truncate font-mono text-[0.8438rem] text-[var(--muted-foreground)]">{s3Url}</p>
              <div className="mt-2 flex flex-wrap gap-1.5">
                <Badge variant="success">{t('buckets.common.active')}</Badge>
                {bucket?.objectCount != null && <Badge>{t('buckets.summary.objects', { count: bucket.objectCount.toLocaleString(language) })}</Badge>}
                {bucket?.size != null && <Badge>{formatBytes(bucket.size)}</Badge>}
              </div>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="secondary" onClick={copyUrl}>
              <Copy /> {t('buckets.actions.copy_url')}
            </Button>
            {canBucket(bucket, 'object.write') && isObjectsTab && (
              <Button variant="primary" onClick={() => document.dispatchEvent(new CustomEvent('bucket:upload'))}>
                <Upload /> {t('buckets.actions.upload')}
              </Button>
            )}
          </div>
        </div>
      </section>

      {/* Tabs */}
      <nav className="flex h-12 items-center gap-0 border-b border-[var(--border)] px-7">
        {visibleTabs.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.end}
            className={({ isActive }) =>
              cn(
                'relative -mb-px inline-flex h-12 items-center px-3.5 text-[0.875rem] font-medium transition-colors',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)] rounded-sm',
                isActive
                  ? 'text-[var(--primary)] border-b-2 border-[var(--primary)]'
                  : 'text-[var(--muted-foreground)] border-b-2 border-transparent hover:text-[var(--foreground)]',
              )
            }
          >
            {t(tab.label)}
          </NavLink>
        ))}
      </nav>

      <div className="min-w-0">
        <Outlet />
      </div>
    </div>
  );
}
