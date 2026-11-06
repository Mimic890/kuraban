import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { bucketsApi } from '@/lib/api';
import { clearListCache, readListCache, writeListCache } from '@/lib/list-cache';
import { useAuthStore } from '@/store/auth-store';
import { useClusterStore } from '@/store/cluster-store';
import { useBuckets } from './useApi';

vi.mock('@/lib/api', () => ({ bucketsApi: { list: vi.fn() } }));

const mockedList = vi.mocked(bucketsApi.list);
const cacheKey = 'alice:c1:buckets';
const bucket = (name: string) => ({ name }) as Awaited<ReturnType<typeof bucketsApi.list>>[number];

function render() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return renderHook(() => useBuckets(), { wrapper });
}

describe('cached lists', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useAuthStore.setState({ user: { username: 'alice' } });
    useClusterStore.setState({ activeClusterId: 'c1' });
  });

  it('shows the saved list at once, then replaces and saves the fresh one', async () => {
    writeListCache(cacheKey, [bucket('old')]);
    let resolve!: (list: ReturnType<typeof bucket>[]) => void;
    mockedList.mockReturnValue(new Promise((r) => (resolve = r)));

    const { result } = render();
    expect(result.current.data).toEqual([bucket('old')]);
    expect(result.current.isLoading).toBe(false);
    await waitFor(() => expect(result.current.isFetching).toBe(true));

    resolve([bucket('new')]);
    await waitFor(() => expect(result.current.data).toEqual([bucket('new')]));
    expect(readListCache(cacheKey)).toEqual([bucket('new')]);
  });

  it('keeps the saved list when the refresh fails', async () => {
    writeListCache(cacheKey, [bucket('old')]);
    mockedList.mockRejectedValue(new Error('down'));

    const { result } = render();
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.data).toEqual([bucket('old')]);
    expect(readListCache(cacheKey)).toEqual([bucket('old')]);
  });

  it('never shows another user or cluster the saved list', async () => {
    writeListCache('bob:c1:buckets', [bucket('bobs')]);
    writeListCache('alice:c2:buckets', [bucket('other-cluster')]);
    mockedList.mockReturnValue(new Promise(() => {}));

    const { result } = render();
    expect(result.current.data).toBeUndefined();
    expect(result.current.isLoading).toBe(true);
  });

  it('ignores a damaged entry and clears only its own entries', () => {
    localStorage.setItem('kuraban-list-cache:v1:broken', '{not json');
    localStorage.setItem('unrelated', 'keep');
    expect(readListCache('broken')).toBeUndefined();

    clearListCache();
    expect(localStorage.getItem('kuraban-list-cache:v1:broken')).toBeNull();
    expect(localStorage.getItem('unrelated')).toBe('keep');
  });
});
