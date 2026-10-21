import { useQuery } from '@tanstack/react-query';
import { capabilitiesApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { useClusterStore } from '@/store/cluster-store';

export function useCapabilities() {
  const activeClusterId = useClusterStore((state) => state.activeClusterId);
  return useQuery({
    queryKey: queryKeys.capabilities.get(activeClusterId),
    queryFn: () => capabilitiesApi.get(),
    staleTime: Infinity,
    gcTime: Infinity,
  });
}
