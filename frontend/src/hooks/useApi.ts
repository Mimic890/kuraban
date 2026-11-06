import { useQuery, useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query';
import { bucketsApi, accessApi, garageApi, analyticsApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { readListCache, writeListCache } from '@/lib/list-cache';
import { toast } from 'sonner';
import { useClusterStore } from '@/store/cluster-store';
import { useAuthStore } from '@/store/auth-store';
import { useTranslation } from '@/lib/i18n';


// Shows the list saved on this device at once and refreshes it in the background.
function useCachedList<T>(name: string, queryKey: (clusterId: string | null) => QueryKey, fetchList: () => Promise<T[]>, enabled: boolean) {
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  const username = useAuthStore((state) => state.user?.username);
  const cacheKey = username && activeClusterId ? `${username}:${activeClusterId}:${name}` : null;
  return useQuery({
    queryKey: queryKey(activeClusterId),
    queryFn: async () => {
      const list = await fetchList();
      // shortcut: a superseded slow request can still save an older list, the next page open corrects it.
      if (cacheKey) writeListCache(cacheKey, list);
      return list;
    },
    initialData: () => (cacheKey ? readListCache<T>(cacheKey) : undefined),
    // Saved lists always count as stale, so opening a page checks for new data.
    initialDataUpdatedAt: 0,
    // The default keeps the previous key's data, which here is another cluster's list.
    placeholderData: undefined,
    staleTime: 30 * 1000,
    refetchOnMount: true,
    enabled: enabled && !!activeClusterId,
  });
}

export function useBuckets(enabled = true) {
  return useCachedList('buckets', queryKeys.buckets.list, bucketsApi.list, enabled);
}

export function useCreateBucket() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: (name: string) => bucketsApi.create(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success(t('api.bucket_created_success'));
    },
  });
}

export function useDeleteBucket() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: (name: string) => bucketsApi.delete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success(t('api.bucket_deleted_success'));
    },
  });
}

export function useGrantBucketPermission() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: ({ bucketName, accessKeyId, permissions }: {
      bucketName: string;
      accessKeyId: string;
      permissions: { read: boolean; write: boolean; owner: boolean };
    }) => bucketsApi.grantPermission(bucketName, accessKeyId, permissions),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      toast.success(t('api.permissions_granted_success'));
    },
  });
}

export function useUpdateBucketQuotas() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: ({
      bucketName,
      maxSize,
      maxObjects,
    }: {
      bucketName: string;
      maxSize: number | null;
      maxObjects: number | null;
    }) => bucketsApi.updateBucketQuotas(bucketName, { maxSize, maxObjects }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      toast.success(t('api.quotas_updated_success'));
    },
  });
}

export function useBucketAliasChange() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: ({ kind, bucketName, alias }: { kind: 'add' | 'remove'; bucketName: string; alias: string }) =>
      kind === 'add' ? bucketsApi.addAlias(bucketName, alias) : bucketsApi.removeAlias(bucketName, alias),
    onSuccess: (_, { kind }) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      toast.success(t(kind === 'add' ? 'api.alias_added_success' : 'api.alias_removed_success'));
    },
    // Unlike most errors, the reason here is for the user to act on: a bad name, or one that is taken.
    onError: (error: unknown) => {
      const message = (error as { response?: { data?: { error?: { message?: string } } } }).response?.data?.error?.message;
      toast.error(t('api.errors.generic'), { description: message ?? t('api.errors.unknown') });
    },
  });
}

export function useCleanupBucketUploads() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  return useMutation({
    mutationFn: ({ bucketName, olderThanSecs }: { bucketName: string; olderThanSecs: number }) =>
      bucketsApi.cleanupUploads(bucketName, olderThanSecs),
    onSuccess: ({ uploadsDeleted }) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      if (uploadsDeleted > 0) {
        toast.success(t('api.uploads_cleaned_success', { count: uploadsDeleted }));
      } else {
        toast.info(t('api.uploads_cleaned_none'));
      }
    },
  });
}


export function useAccessKeys(enabled = true) {
  return useCachedList('keys', queryKeys.accessKeys.list, accessApi.listKeys, enabled);
}

export function useClusterHealth(enabled = true) {
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  return useQuery({
    queryKey: queryKeys.cluster.health(activeClusterId),
    queryFn: () => garageApi.getClusterHealth(),
    staleTime: 30 * 1000,
    enabled: enabled && !!activeClusterId,
  });
}

export function useDashboardMetrics(enabled = true) {
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  return useQuery({
    queryKey: queryKeys.dashboard.metrics(activeClusterId),
    queryFn: () => analyticsApi.getMetrics(),
    staleTime: 2 * 60 * 1000,
    enabled: enabled && !!activeClusterId,
  });
}

// Combined hook for dashboard data
export function useDashboardData() {
  const metrics = useDashboardMetrics();
  const buckets = useBuckets();
  const health = useClusterHealth();

  return {
    metrics,
    buckets,
    health,
    isLoading: metrics.isLoading || buckets.isLoading || health.isLoading,
    isError: metrics.isError || buckets.isError || health.isError,
  };
}
