import axios from 'axios';
import {toast} from 'sonner';
import type {
  AccessKey,
  ApiResponse,
  Bucket,
  BucketDetails,
  ClusterHealth,
  ClusterStatistics,
  ClusterStatus,
  GarageCapabilities,
  MultiNodeResponse,
  MultiNodeStatisticsResponse,
  ObjectListResponse,
  ObjectMetadata,
  S3Object,
  StorageMetrics,
} from '@/types';
import type {
  Account,
  AuthConfig,
  AuthUser,
  PasskeyCeremony,
  SensitiveAccountInput,
  TotpEnrollment,
} from '@/types/auth';
import type { CreationOptionsJSON, RequestOptionsJSON } from '@/lib/webauthn';
import { translate } from '@/lib/i18n';

const api = axios.create({
  baseURL: '/api',
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Separate axios instance for auth endpoints (which are not under /api)
const authApiClient = axios.create({
  baseURL: '/auth',
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
});

function authPayload<T>(payload: T | { data: T }): T {
  return payload && typeof payload === 'object' && 'data' in payload
    ? (payload as { data: T }).data
    : payload as T;
}

declare module 'axios' {
  interface AxiosRequestConfig {
    /** Skip the global error toast; the caller shows the error inline. */
    silent?: boolean;
  }
}

api.interceptors.request.use((config) => {
  // NOTE: authentication is cookie-based (httpOnly session cookie). The token
  // is never stored in localStorage or read from it; that branch was removed
  // to avoid reintroducing an XSS-exfiltration sink. See /auth endpoints.
  let clusterId = null;
  try {
    const storageData = localStorage.getItem('cluster-storage');
    if (storageData) {
      const parsed = JSON.parse(storageData);
      clusterId = parsed.state?.activeClusterId;
    }
  } catch {
    // Ignore parsing errors
  }
  
  if (!clusterId) {
    clusterId = localStorage.getItem('cluster-id');
  }

  if (clusterId) {
    config.headers['X-Cluster-Id'] = clusterId;
  }
  
  return config;
});

api.interceptors.response.use(
  (response) => {
    // If response has success=false in data, treat it as an error
    if (response.data && response.data.success === false && response.data.error) {
      const error = response.data.error;
      const errorMessage = translate('api.errors.generic');
      const errorCode = error.code || 'UNKNOWN_ERROR';

      // Display toast with error details
      toast.error(errorMessage, {
        description: translate('api.errors.code', { code: errorCode }),
      });

      // Reject the promise so it's treated as an error
      return Promise.reject(new Error(errorMessage));
    }
    return response;
  },
  (error) => {
    // Superseded requests (React Query aborts them on key changes) are not errors.
    if (axios.isCancel(error)) {
      return Promise.reject(error);
    }

    // Callers that render the error themselves (forms) opt out of the toast.
    if (error.config?.silent && error.response?.status !== 401) {
      return Promise.reject(error);
    }

    // Handle 401 Unauthorized - redirect to login
    if (error.response?.status === 401) {
      // Only redirect if not already on login page
      if (window.location.pathname !== '/login') {
        window.location.href = '/login';
      }

      return Promise.reject(error);
    }

    // 501 Not Implemented = expected for unsupported Garage version features
    if (error.response?.status === 501) {
      return Promise.reject(error);
    }

    // Handle axios errors
    if (error.response) {
      // Server responded with error status
      const data = error.response.data;

      if (data && data.error) {
        const errorMessage = translate('api.errors.generic');
        const errorCode = data.error.code || 'UNKNOWN_ERROR';

        toast.error(errorMessage, {
          description: translate('api.errors.code', { code: errorCode }),
        });
      } else {
        // Generic HTTP error
        toast.error(translate('api.errors.requestFailed', { status: error.response.status }), {
          description: translate('api.errors.unknown'),
        });
      }
    } else if (error.request) {
      // Request made but no response received
      toast.error(translate('api.errors.networkTitle'), {
        description: translate('api.errors.networkDescription'),
      });
    } else {
      // Something else happened
      toast.error(translate('api.errors.title'), {
        description: translate('api.errors.unexpected'),
      });
    }

    return Promise.reject(error);
  }
);

// Auth API
export const authApi = {
  getConfig: async () => {
    const response = await authApiClient.get<AuthConfig>('/config');
    return response;
  },

  getSetupStatus: async () => {
    const response = await api.get<{ setup: boolean }>('/v1/panel/setup');
    return response;
  },

  setupPanel: async (data: { nickname: string; password: string }) => {
    const response = await api.post<{ success: boolean }>('/v1/panel/setup', data);
    return response;
  },

  loginAdmin: async (username: string, password: string) => {
    const response = await authApiClient.post<{
      success?: boolean;
      user?: AuthUser;
      mfa_required?: boolean;
      challenge_id?: string;
    }>('/login', {
      username,
      password,
    });
    return response;
  },

  loginMfa: async (challenge_id: string, code: string) => {
    return authApiClient.post<{ success: boolean; user: AuthUser }>('/login/mfa', { challenge_id, code });
  },

  loginToken: async (token: string) => {
    const response = await authApiClient.post<{ success: boolean; user: AuthUser }>('/login-token', {
      token,
    });
    return response;
  },

  me: async () => {
    const response = await authApiClient.get<{ success: boolean; user: AuthUser }>('/me');
    return response;
  },

  getAccount: async () => {
    const response = await authApiClient.get<Account | { data: Account }>('/account');
    return authPayload(response.data);
  },

  updateAccount: async (input: SensitiveAccountInput & { email?: string; new_password?: string }) => {
    const response = await authApiClient.patch<Account | { data: Account }>('/account', input);
    return authPayload(response.data);
  },

  beginTotp: async (current_password: string) => {
    const response = await authApiClient.post<TotpEnrollment | { data: TotpEnrollment }>('/totp/begin', { current_password });
    const enrollment = authPayload(response.data);
    return { ...enrollment, qr_code_data_url: enrollment.qr_code_data_url || enrollment.qr_png_data_url || '' };
  },

  finishTotp: async (enrollment_id: string, code: string) => {
    const response = await authApiClient.post<{ recovery_codes: string[] } | { data: { recovery_codes: string[] } }>('/totp/finish', { enrollment_id, code });
    return authPayload(response.data);
  },

  disableTotp: (input: SensitiveAccountInput) => authApiClient.delete('/totp', { data: input }),

  regenerateRecoveryCodes: async (input: SensitiveAccountInput) => {
    const response = await authApiClient.post<{ recovery_codes: string[] } | { data: { recovery_codes: string[] } }>('/recovery-codes', input);
    return authPayload(response.data);
  },

  beginPasskeyRegistration: async (input: SensitiveAccountInput & { name: string }) => {
    const response = await authApiClient.post<PasskeyCeremony<CreationOptionsJSON> | { data: PasskeyCeremony<CreationOptionsJSON> }>('/passkeys/register/begin', input);
    return authPayload(response.data);
  },

  finishPasskeyRegistration: (ceremony_id: string, credential: unknown) =>
    authApiClient.post('/passkeys/register/finish', { ceremony_id, credential }),

  removePasskey: (id: string, input: SensitiveAccountInput) =>
    authApiClient.delete(`/passkeys/${encodeURIComponent(id)}`, { data: input }),

  beginPasskeyLogin: async () => {
    const response = await authApiClient.post<PasskeyCeremony<RequestOptionsJSON> | { data: PasskeyCeremony<RequestOptionsJSON> }>('/passkeys/login/begin');
    return authPayload(response.data);
  },

  finishPasskeyLogin: async (ceremony_id: string, credential: unknown) => {
    const response = await authApiClient.post<{ success: boolean; user: AuthUser }>('/passkeys/login/finish', { ceremony_id, credential });
    return response.data;
  },

  logoutAdmin: async () => {
    return authApiClient.post('/logout');
  },

  logoutOIDC: async () => {
    const response = await authApiClient.post('/oidc/logout');
    return response;
  },

  loginOIDC: () => {
    window.location.href = '/auth/oidc/login';
  },
};


// Capabilities API
export const capabilitiesApi = {
  get: async (): Promise<GarageCapabilities> => {
    const response = await api.get('/v1/capabilities');
    return response.data.data;
  },
};

// Bucket API
export const bucketsApi = {
  list: async (): Promise<Bucket[]> => {
    const response = await api.get('/v1/buckets');
    return response.data.data.buckets || [];
  },

  get: async (name: string): Promise<BucketDetails> => {
    const response = await api.get(`/v1/buckets/${name}`);
    return response.data.data;
  },

  create: async (bucketName: string): Promise<void> => {
    await api.post('/v1/buckets', { name: bucketName });
  },

  delete: async (name: string): Promise<void> => {
    await api.delete(`/v1/buckets/${name}`);
  },

  emptyBucket: async (name: string): Promise<{ deleted: number }> => {
    const response = await api.post(`/v1/buckets/${name}/objects/empty`);
    return response.data.data;
  },

  grantPermission: async (
    bucketName: string,
    accessKeyId: string,
    permissions: { read: boolean; write: boolean; owner: boolean }
  ): Promise<void> => {
    await api.post(`/v1/buckets/${bucketName}/permissions`, {
      accessKeyId,
      permissions,
    });
  },


  updateBucketWebsite: async (
    name: string,
    payload: { enabled: boolean; indexDocument?: string; errorDocument?: string }
  ) => {
    const response = await api.put<ApiResponse<any>>(
      `/v1/buckets/${encodeURIComponent(name)}/website`,
      payload
    );
    return response.data.data;
  },

  updateBucketQuotas: async (
    name: string,
    payload: { maxSize: number | null; maxObjects: number | null }
  ) => {
    // Map nulls to undefined so they are omitted from the JSON body —
    // backend treats a missing field as "clear this quota".
    const body: { maxSize?: number; maxObjects?: number } = {};
    if (payload.maxSize !== null) body.maxSize = payload.maxSize;
    if (payload.maxObjects !== null) body.maxObjects = payload.maxObjects;
    const response = await api.put<ApiResponse<any>>(
      `/v1/buckets/${encodeURIComponent(name)}/quotas`,
      body
    );
    return response.data.data;
  },
};

// Objects API
export const objectsApi = {
  list: async (bucket: string, prefix?: string, maxKeys?: number, continuationToken?: string): Promise<ObjectListResponse> => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const params: any = {};
    if (prefix) params.prefix = prefix;
    if (maxKeys) params.max_keys = maxKeys;
    if (continuationToken) params.continuation_token = continuationToken;

    const response = await api.get(`/v1/buckets/${bucket}/objects`, { params });
    const data = response.data.data;

    // Combine objects and prefixes (folders)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const objects: S3Object[] = data.objects?.map((obj: any) => ({
      key: obj.key,
      size: obj.size,
      lastModified: obj.last_modified,
      etag: obj.etag,
      contentType: obj.content_type,
      storageClass: obj.storage_class,
      isFolder: false,
    })) || [];

    const folders: S3Object[] = data.prefixes?.map((prefix: string) => ({
      key: prefix,
      size: 0,
      lastModified: null,
      isFolder: true,
    })) || [];

    return {
      bucket: data.bucket,
      objects: [...folders, ...objects],
      prefixes: data.prefixes || [],
      count: data.count,
      isTruncated: data.is_truncated || false,
      nextContinuationToken: data.next_continuation_token,
    };
  },

  // Recursive, best-effort substring search across all objects under `prefix`.
  // The backend scans and filters (S3/Garage has no server-side substring
  // search), so this finds matches regardless of which page they'd be on.
  search: async (bucket: string, query: string, prefix?: string): Promise<ObjectListResponse> => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const params: any = { search: query };
    if (prefix) params.prefix = prefix;

    const response = await api.get(`/v1/buckets/${bucket}/objects`, { params });
    const data = response.data.data;

    // Search returns a flat list of matching objects (no folders/prefixes).
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const objects: S3Object[] = data.objects?.map((obj: any) => ({
      key: obj.key,
      size: obj.size,
      lastModified: obj.last_modified,
      etag: obj.etag,
      contentType: obj.content_type,
      storageClass: obj.storage_class,
      isFolder: false,
    })) || [];

    return {
      bucket: data.bucket,
      objects,
      prefixes: [],
      count: data.count,
      isTruncated: data.is_truncated || false,
      nextContinuationToken: data.next_continuation_token,
    };
  },

  get: async (bucket: string, key: string, signal?: AbortSignal): Promise<Blob> => {
    const response = await api.get(`/v1/buckets/${bucket}/object`, {
      responseType: 'blob',
      signal,
      params: { key },
    });
    return response.data;
  },

  getMetadata: async (bucket: string, key: string, options?: { signal?: AbortSignal }): Promise<ObjectMetadata> => {
    const response = await api.get(`/v1/buckets/${bucket}/object/metadata`, { ...options, params: { key } });
    const data = response.data.data;
    return {
      key: data.key,
      size: data.size,
      lastModified: data.last_modified,
      contentType: data.content_type,
      etag: data.etag,
      storageClass: data.storage_class,
      metadata: data.metadata,
    };
  },

  createDirectory: async (bucket: string, key: string): Promise<void> => {
    await api.post(`/v1/buckets/${bucket}/directories`, { key });
  },

  upload: async (bucket: string, key: string, file: File, onProgress?: (progress: number) => void): Promise<void> => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('key', key);
    await api.post(`/v1/buckets/${bucket}/objects`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          onProgress(progress);
        }
      },
    });
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  uploadMultiple: async (bucket: string, files: File[]): Promise<any> => {
    const formData = new FormData();
    files.forEach(file => {
      formData.append('files', file);
    });
    const response = await api.post(`/v1/buckets/${bucket}/objects/upload-multiple`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data.data;
  },

  delete: async (bucket: string, key: string): Promise<void> => {
    await api.delete(`/v1/buckets/${bucket}/object`, { params: { key } });
  },

  // Deletes the given object keys and/or recursively deletes every object under
  // each folder prefix in a single request.
  deleteMultiple: async (bucket: string, keys: string[], prefixes: string[] = []): Promise<void> => {
    const payload = { keys, ...(prefixes.length > 0 && { prefixes }) };
    await api.post(`/v1/buckets/${bucket}/objects/delete-multiple`, payload);
  },

  getPresignedUrl: async (bucket: string, key: string, expiresIn: number = 3600): Promise<string> => {
    const response = await api.get(`/v1/buckets/${bucket}/object/presign`, {
      params: { key, expires_in: expiresIn }
    });
    return response.data.data.url;
  },

  getPreviewUrl: async (bucket: string, key: string, signal?: AbortSignal): Promise<{ url: string; expiresAt: string }> => {
    const response = await api.get(`/v1/buckets/${bucket}/object/preview-url`, { signal, params: { key } });
    const data = response.data.data;
    return { url: data.url, expiresAt: data.expires_at };
  },
};

// Access Control API (Users/Keys)
export const accessApi = {
  listKeys: async (): Promise<AccessKey[]> => {
    const response = await api.get('/v1/users');
    return response.data.data.users || [];
  },

  getKey: async (accessKey: string): Promise<AccessKey> => {
    const response = await api.get(`/v1/users/${accessKey}`);
    return response.data.data;
  },

  getSecretKey: async (accessKey: string): Promise<string> => {
    const response = await api.get(`/v1/users/${accessKey}/secret`);
    return response.data.data.secretKey;
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  createKey: async (name: string, permissions?: any[]): Promise<AccessKey> => {
    const response = await api.post('/v1/users', { name, permissions });
    return response.data.data;
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  updateKey: async (accessKey: string, updates: any): Promise<void> => {
    await api.patch(`/v1/users/${accessKey}`, updates);
  },

  deleteKey: async (accessKey: string): Promise<void> => {
    await api.delete(`/v1/users/${accessKey}`);
  },
};

// Analytics API
export const analyticsApi = {
  getMetrics: async (): Promise<StorageMetrics> => {
    const response = await api.get('/v1/monitoring/dashboard');
    return response.data.data;
  },
};

// Garage Cluster & Monitoring API
export const garageApi = {
  getClusterHealth: async (): Promise<ClusterHealth> => {
    const response = await api.get('/v1/cluster/health');
    return response.data.data;
  },

  getClusterStatus: async (): Promise<ClusterStatus> => {
    const response = await api.get('/v1/cluster/status');
    return response.data.data;
  },

  getClusterStatistics: async (): Promise<ClusterStatistics> => {
    const response = await api.get('/v1/cluster/statistics');
    return response.data.data;
  },

  getNodeInfo: async (nodeId: string = 'self'): Promise<MultiNodeResponse> => {
    const response = await api.get(`/v1/cluster/nodes/${nodeId}`);
    return response.data.data;
  },

  getNodeStatistics: async (nodeId: string): Promise<MultiNodeStatisticsResponse> => {
    const response = await api.get(`/v1/cluster/nodes/${nodeId}/statistics`);
    return response.data.data;
  },

};

export default api;
