import { ShieldOff } from 'lucide-react';
import { EmptyState } from '@/components/ui/empty-state';
import { useTranslation } from '@/lib/i18n';

/** Shown to authenticated users whose identity matches no team. */
export function NoAccess() {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-[60vh] items-center justify-center px-6 py-6">
      <EmptyState
        icon={<ShieldOff />}
        tone="neutral"
        title={t('auth.noAccess.title')}
        description={t('auth.noAccess.description')}
      />
    </div>
  );
}
