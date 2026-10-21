import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { bucketsApi, accessApi, garageApi, analyticsApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { toast } from 'sonner';
import { useClusterStore } from '@/store/cluster-store';
import { useTranslation } from '@/lib/i18n';


export function useBuckets(enabled = true) {
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  return useQuery({
    queryKey: queryKeys.buckets.list(activeClusterId),
    queryFn: () => bucketsApi.list(),
    enabled: enabled && !!activeClusterId,
  });
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


export function useAccessKeys(enabled = true) {
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  return useQuery({
    queryKey: queryKeys.accessKeys.list(activeClusterId),
    queryFn: () => accessApi.listKeys(),
    enabled: enabled && !!activeClusterId,
  });
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
