import { AlertTriangle, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';

export function RefreshButton({ query }: { query: { isFetching: boolean; isError: boolean; refetch: () => unknown } }) {
  const { t } = useTranslation();
  const failed = query.isError && !query.isFetching;
  return (
    <Button variant="secondary" className="disabled:opacity-100" disabled={query.isFetching} aria-busy={query.isFetching} onClick={() => void query.refetch()}>
      {failed ? <AlertTriangle className="text-[var(--destructive)]" /> : <RefreshCw className={cn(query.isFetching && 'animate-spin')} />}
      {query.isFetching ? t('common.refreshing') : failed ? t('common.refresh_failed') : t('common.refresh')}
    </Button>
  );
}
