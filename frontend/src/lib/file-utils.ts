import { objectsApi } from './api';
import { toast } from 'sonner';
import { useSettingsStore } from '@/store/settings-store';
import { translate } from '@/lib/i18n';

/**
 * Use the presigned URL so large downloads stream from object storage.
 */
export async function downloadObject(bucket: string, key: string): Promise<void> {
  try {
    const url = await objectsApi.getPresignedUrl(bucket, key);
    const a = document.createElement('a');
    a.href = url;
    a.download = key.split('/').pop() || 'download';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    toast.success(translate('buckets.toast.download_started'));
  } catch {
    // error toast handled by axios interceptor
  }
}


/**
 * Generate breadcrumbs from a file path
 */
export function getBreadcrumbs(currentPath: string): Array<{ label: string; path: string }> {
  const rootLabel = translate('buckets.common.root');
  if (!currentPath) return [{ label: rootLabel, path: '' }];

  const parts = currentPath.split('/').filter(Boolean);
  const breadcrumbs = [{ label: rootLabel, path: '' }];

  parts.forEach((part, index) => {
    const path = parts.slice(0, index + 1).join('/') + '/';
    breadcrumbs.push({ label: part, path });
  });

  return breadcrumbs;
}

/**
 * Format relative time from a date
 */
export function formatRelativeTime(date: Date): string {
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  const relative = new Intl.RelativeTimeFormat(useSettingsStore.getState().language, { numeric: 'auto' });
  if (diffMins < 1) return relative.format(0, 'minute');
  if (diffMins < 60) return relative.format(-diffMins, 'minute');
  if (diffHours < 24) return relative.format(-diffHours, 'hour');
  if (diffDays < 7) return relative.format(-diffDays, 'day');
  if (diffDays < 30) return relative.format(-Math.floor(diffDays / 7), 'week');
  return relative.format(-Math.floor(diffDays / 30), 'month');
}

/**
 * Format bytes to human-readable size
 */
export function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return '0 B';

  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return `${new Intl.NumberFormat(useSettingsStore.getState().language, {
    maximumFractionDigits: dm,
  }).format(bytes / Math.pow(k, i))} ${sizes[i]}`;
}
