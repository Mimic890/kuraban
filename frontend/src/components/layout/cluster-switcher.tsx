import { useClusterStore } from '@/store/cluster-store';
import { Globe, Check, Plus } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from '@/lib/i18n';

export function ClusterSwitcher() {
  const { t } = useTranslation();
  const { clusters, activeClusterId, setActiveCluster } = useClusterStore();
  const navigate = useNavigate();

  const activeCluster = clusters.find((c) => c.id === activeClusterId);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex h-8 items-center gap-2 rounded-md px-2.5 text-[0.875rem] font-medium transition-colors hover:bg-[var(--accent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]">
        <Globe className="h-4 w-4 text-[var(--muted-foreground)]" />
        <span className="truncate max-w-[150px]">
          {activeCluster ? activeCluster.name : t('clusters.switcher.selectCluster')}
        </span>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="start" className="w-56 p-1">
        {clusters.map((cluster) => {
          const isActive = cluster.id === activeClusterId;
          return (
            <DropdownMenuItem
              key={cluster.id}
              onClick={() => setActiveCluster(cluster.id)}
              className="flex items-center justify-between cursor-pointer py-1.5 px-2.5 rounded-sm hover:bg-[var(--accent)]"
            >
              <div className="flex items-center gap-2">
                <Globe className="h-4 w-4 text-[var(--muted-foreground)]" />
                <span className={isActive ? 'font-medium' : 'text-[var(--muted-foreground)]'}>
                  {cluster.name}
                </span>
              </div>
              {isActive && <Check className="h-4 w-4 text-[var(--primary)]" />}
            </DropdownMenuItem>
          );
        })}

        {clusters.length > 0 && <DropdownMenuSeparator className="my-1 bg-[var(--border)]" />}

        <DropdownMenuItem
          onClick={() => navigate('/connections', { state: { addCluster: true } })}
          className="flex items-center gap-2 cursor-pointer py-1.5 px-2.5 rounded-sm hover:bg-[var(--accent)] text-[var(--muted-foreground)]"
        >
          <Plus className="h-4 w-4" />
          <span>{t('clusters.switcher.addGarage')}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
