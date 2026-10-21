import axios from 'axios';
import api from '@/lib/api';

export interface EndpointCheck {
  ok: boolean;
  status?: number;
  message?: string;
}

export interface ConnectionCheck {
  ok: boolean;
  api_version?: string;
  admin: EndpointCheck;
  s3: EndpointCheck;
}

/** A private address refused by the SSRF policy; it can be allowlisted. */
export interface BlockedEndpoint {
  host: string;
  ip: string;
  private: boolean;
}

export interface ClusterDraft {
  name: string;
  region: string;
  endpoint: string;
  admin_endpoint: string;
  admin_token: string;
  use_ssl: boolean;
  force_path_style: boolean;
}

export const clusterApi = {
  /** Probe a cluster without saving it. With `id` and no token, the stored token is used. */
  test: async (draft: ClusterDraft & { id?: string }): Promise<ConnectionCheck> => {
    const response = await api.post('/v1/panel/clusters/test', draft, { silent: true });
    return response.data.data;
  },
};

/** Pulls the server's message and the blocked address (if any) out of an API error. */
export function describeApiError(err: unknown): { message: string; blocked?: BlockedEndpoint } {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { error?: { message?: string } | string; blocked?: BlockedEndpoint } | undefined;
    const message = typeof data?.error === 'string' ? data.error : data?.error?.message;
    return { message: message || err.message, blocked: data?.blocked };
  }
  return { message: err instanceof Error ? err.message : String(err) };
}

/** Keeps use_ssl in step with an explicit scheme typed into the S3 endpoint. */
export function normalizeDraft(d: ClusterDraft): ClusterDraft {
  const e = d.endpoint.trim();
  if (/^https:\/\//i.test(e)) return { ...d, endpoint: e, use_ssl: true };
  if (/^http:\/\//i.test(e)) return { ...d, endpoint: e, use_ssl: false };
  return { ...d, endpoint: e };
}
