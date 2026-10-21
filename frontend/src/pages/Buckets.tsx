import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertCircle, Plus } from 'lucide-react';
import { useBuckets, useCreateBucket, useDeleteBucket } from '@/hooks/useApi';
import { usePermissions } from '@/hooks/usePermissions';
import { BucketListView } from '@/components/buckets/BucketListView';
import { CreateBucketDialog } from '@/components/buckets/CreateBucketDialog';
import { DeleteBucketDialog } from '@/components/buckets/DeleteBucketDialog';
import { PageHeader } from '@/components/ui/page-header';
import { Button } from '@/components/ui/button';
import { bucketsApi } from '@/lib/api';
import type { Bucket } from '@/types';
import { useClusterStore } from '@/store/cluster-store';
import { Navigate } from 'react-router-dom';
import { useTranslation } from '@/lib/i18n';
import { EmptyState } from '@/components/ui/empty-state';

export function Buckets() {
  const navigate = useNavigate();
  const { t, language } = useTranslation();
  const [searchQuery, setSearchQuery] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Bucket | null>(null);

  const { hasAnyPerm } = usePermissions();
  const { data: buckets = [], isLoading, isError, refetch } = useBuckets();
  const createMutation = useCreateBucket();
  const deleteMutation = useDeleteBucket();
  const { clusters } = useClusterStore();

  const createBucket = async (name: string) => {
    try {
      await createMutation.mutateAsync(name);
      return true;
    } catch {
      return false;
    }
  };

  if (clusters.length === 0) {
    return <Navigate to="/" replace />;
  }

  if (isError) {
    return (
      <div>
        <PageHeader title={t('buckets.title')} subtitle={t('buckets.errors.load_failed')} />
        <div className="p-6">
          <EmptyState icon={<AlertCircle />} title={t('buckets.errors.unavailable')} description={t('buckets.errors.request_failed')} tone="destructive" action={<Button onClick={() => void refetch()}>{t('buckets.actions.retry')}</Button>} />
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title={t('buckets.title')}
        subtitle={t('buckets.summary.count', { count: buckets.length.toLocaleString(language) })}
        actions={
          hasAnyPerm('bucket.create') && (
            <Button onClick={() => setCreateOpen(true)}>
              <Plus /> {t('buckets.create')}
            </Button>
          )
        }
      />
      <div className="p-4 sm:p-6">
        <BucketListView
          buckets={buckets}
          searchQuery={searchQuery}
          isLoading={isLoading}
          onSearchChange={setSearchQuery}
          onViewBucket={(name) => navigate(`/buckets/${name}/objects`)}
          onOpenSettings={(b) => navigate(`/buckets/${b.name}/settings`)}
          onWebsiteSettings={(b) => navigate(`/buckets/${b.name}/website`)}
          onDeleteBucket={(b) => setDeleteTarget(b)}
        />
      </div>

      <CreateBucketDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreateBucket={createBucket}
      />

      {deleteTarget && (
        <DeleteBucketDialog
          open={!!deleteTarget}
          onOpenChange={(o) => !o && setDeleteTarget(null)}
          bucketName={deleteTarget.name}
          objectCount={deleteTarget.objectCount ?? 0}
          onEmptyBucket={() => bucketsApi.emptyBucket(deleteTarget.name).then(() => {})}
          onDeleteBucket={async () => {
            await deleteMutation.mutateAsync(deleteTarget.name);
            setDeleteTarget(null);
          }}
        />
      )}
    </div>
  );
}
