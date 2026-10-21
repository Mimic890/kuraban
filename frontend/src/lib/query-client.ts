import {QueryClient} from '@tanstack/react-query';

// Create a query client with default options
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // Data is fresh for 5 minutes
      gcTime: 10 * 60 * 1000, // Cache data for 10 minutes (formerly cacheTime)
      retry: 1, // Retry failed requests once
      refetchOnWindowFocus: false, // Don't refetch when window regains focus
      refetchOnMount: false, // Don't refetch on component mount if data exists
      placeholderData: (previousData: unknown) => previousData, // Keep previous data while fetching new data
    },
  },
});

// Query keys for consistent cache management
export const queryKeys = {
  buckets: {
    all: ['buckets'] as const,
    list: (clusterId?: string | null) => [...queryKeys.buckets.all, 'list', clusterId ?? 'none'] as const,
    detail: (name: string, clusterId?: string | null) => [...queryKeys.buckets.all, 'detail', clusterId ?? 'none', name] as const,
  },
  objects: {
    all: ['objects'] as const,
    list: (bucket: string, prefix?: string, clusterId?: string | null) => [...queryKeys.objects.all, 'list', clusterId ?? 'none', bucket, prefix] as const,
  },
  accessKeys: {
    all: ['accessKeys'] as const,
    list: (clusterId?: string | null) => [...queryKeys.accessKeys.all, 'list', clusterId ?? 'none'] as const,
    detail: (keyId: string, clusterId?: string | null) => [...queryKeys.accessKeys.all, 'detail', clusterId ?? 'none', keyId] as const,
  },
  cluster: {
    all: ['cluster'] as const,
    health: (clusterId?: string | null) => [...queryKeys.cluster.all, 'health', clusterId ?? 'none'] as const,
    status: (clusterId?: string | null) => [...queryKeys.cluster.all, 'status', clusterId ?? 'none'] as const,
    statistics: (clusterId?: string | null) => [...queryKeys.cluster.all, 'statistics', clusterId ?? 'none'] as const,
  },
  dashboard: {
    all: ['dashboard'] as const,
    metrics: (clusterId?: string | null) => [...queryKeys.dashboard.all, 'metrics', clusterId ?? 'none'] as const,
  },
  capabilities: {
    all: ['capabilities'] as const,
    get: (clusterId?: string | null) => [...queryKeys.capabilities.all, 'get', clusterId ?? 'none'] as const,
  },
};
