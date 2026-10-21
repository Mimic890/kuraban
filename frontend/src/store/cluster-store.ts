import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import api from '@/lib/api';
import { translate } from '@/lib/i18n';

export interface ClusterConfig {
  id: string;
  name: string;
  endpoint: string;
  region: string;
  use_ssl: boolean;
  force_path_style: boolean;
  admin_endpoint: string;
  admin_token?: string;
}

interface ClusterStore {
  clusters: ClusterConfig[];
  activeClusterId: string | null;
  isLoading: boolean;
  isInitialized: boolean;
  error: string | null;

  // Actions
  fetchClusters: () => Promise<void>;
  addCluster: (config: Omit<ClusterConfig, 'id'> & { admin_token: string }) => Promise<ClusterConfig>;
  updateCluster: (id: string, config: Omit<ClusterConfig, 'id'>) => Promise<ClusterConfig>;
  deleteCluster: (id: string) => Promise<void>;
  setActiveCluster: (id: string | null) => void;
}

export const useClusterStore = create<ClusterStore>()(
  persist(
    (set, get) => ({
      clusters: [],
      activeClusterId: null,
      isLoading: false,
      isInitialized: false,
      error: null,

      fetchClusters: async () => {
        try {
          set({ isLoading: true, error: null });
          const response = await api.get<{ clusters: ClusterConfig[] }>('/v1/panel/clusters');
          const clusters = response.data.clusters || [];
          set({ clusters, isLoading: false, isInitialized: true });
          
          // Auto-select first cluster if none is active
          if (clusters.length > 0 && !get().activeClusterId) {
            get().setActiveCluster(clusters[0].id);
          } else if (clusters.length === 0 && get().activeClusterId) {
            get().setActiveCluster(null);
          }
        } catch (err: any) {
          set({ error: err.message || translate('clusters.errors.fetchFailed'), isLoading: false, isInitialized: true });
        }
      },

      // Errors are rendered by the form, so these calls are silent and do not
      // touch isLoading (that would swap the page for a spinner and lose input).
      addCluster: async (config) => {
        const response = await api.post<{ cluster: ClusterConfig }>('/v1/panel/clusters', config, { silent: true });
        const newCluster = response.data.cluster;
        set({ clusters: [...get().clusters, newCluster] });
        if (!get().activeClusterId) {
          get().setActiveCluster(newCluster.id);
        }
        return newCluster;
      },

      updateCluster: async (id, config) => {
        const response = await api.put<{ cluster: ClusterConfig }>(`/v1/panel/clusters/${id}`, config, { silent: true });
        const updated = response.data.cluster;
        set({ clusters: get().clusters.map((c) => (c.id === id ? updated : c)) });
        return updated;
      },

      deleteCluster: async (id) => {
        try {
          set({ isLoading: true, error: null });
          await api.delete(`/v1/panel/clusters/${id}`);
          const clusters = get().clusters.filter((c) => c.id !== id);
          set({ clusters, isLoading: false });
          
          if (get().activeClusterId === id) {
            get().setActiveCluster(clusters.length > 0 ? clusters[0].id : null);
          }
        } catch (err: any) {
          set({ error: err.message || translate('clusters.errors.deleteFailed'), isLoading: false });
          throw err;
        }
      },

      setActiveCluster: (id) => {
        const previousId = get().activeClusterId;
        if (id) {
          localStorage.setItem('cluster-id', id);
        } else {
          localStorage.removeItem('cluster-id');
        }
        set({ activeClusterId: id });
        
        if (previousId !== id) set({ error: null });
      },
    }),
    {
      name: 'cluster-storage',
      partialize: (state) => ({
        activeClusterId: state.activeClusterId,
      }),
    }
  )
);
