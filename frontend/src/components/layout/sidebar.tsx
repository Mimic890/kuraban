import { Link, useLocation } from 'react-router-dom';
import { PanelLeftClose, PanelLeftOpen, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { usePermissions } from '@/hooks/usePermissions';
import { useClusterStore } from '@/store/cluster-store';
import { useSettingsStore } from '@/store/settings-store';
import { useTranslation } from '@/lib/i18n';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { isActivePath, navGroups, settingsNavItem, type NavItem } from './nav';

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
}

function NavLink({ item, collapsed, active, onClick }: { item: NavItem; collapsed: boolean; active: boolean; onClick: () => void }) {
  const { t } = useTranslation();
  const Icon = item.icon;
  const link = (
    <Link
      to={item.href}
      onClick={onClick}
      aria-current={active ? 'page' : undefined}
      className={cn(
        'group relative flex h-8 items-center gap-2.5 rounded-md text-[0.8125rem] transition-colors',
        collapsed ? 'w-8 justify-center' : 'px-2.5',
        active
          ? 'bg-[var(--accent)] font-medium text-[var(--foreground)]'
          : 'text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)]',
      )}
    >
      {active && <span className="absolute -left-2 top-1.5 bottom-1.5 w-[3px] rounded-r bg-[var(--primary)]" />}
      <Icon className={cn('h-4 w-4 shrink-0', active && 'text-[var(--primary)]')} />
      {!collapsed && <span className="truncate">{t(item.title)}</span>}
    </Link>
  );
  if (!collapsed) return link;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{link}</TooltipTrigger>
      <TooltipContent side="right" className="border-[var(--border)] bg-[var(--popover)] text-[var(--popover-foreground)] text-[0.75rem] px-2 py-1">
        {t(item.title)}
      </TooltipContent>
    </Tooltip>
  );
}

export function Sidebar({ isOpen, onClose }: SidebarProps) {
  const { t } = useTranslation();
  const location = useLocation();
  const perms = usePermissions();
  const { clusters } = useClusterStore();
  const { sidebarCollapsed, setSidebarCollapsed } = useSettingsStore();
  const hasClusters = clusters.length > 0;
  // The mobile drawer always shows labels.
  const collapsed = sidebarCollapsed && !isOpen;

  return (
    <TooltipProvider delayDuration={200}>
      {isOpen && <div className="fixed inset-0 z-40 bg-black/50 md:hidden" onClick={onClose} />}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex h-full flex-col border-r border-[var(--border)] bg-[var(--surface-sunken)] transition-[width,transform] duration-200 ease-out md:static md:translate-x-0',
          collapsed ? 'md:w-[52px]' : 'md:w-56',
          'w-64',
          isOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className={cn('flex h-12 shrink-0 items-center border-b border-[var(--border)]', collapsed ? 'justify-center px-0' : 'gap-2.5 px-4')}>
          <Link to="/" onClick={onClose} className="flex items-center gap-2.5 min-w-0">
            <img src="/garage.png" alt="" className="h-5 w-5 shrink-0 object-contain" />
            {!collapsed && (
              <span className="truncate text-[0.875rem] font-semibold tracking-tight text-[var(--foreground)]">
                Kuraban<span className="text-[var(--muted-foreground)] font-normal"> 蔵番</span>
              </span>
            )}
          </Link>
          <button type="button" onClick={onClose} className="ml-auto rounded p-1 text-[var(--muted-foreground)] hover:text-[var(--foreground)] md:hidden" aria-label={t('layout.sidebar.close')}>
            <X className="h-4 w-4" />
          </button>
        </div>

        <nav className={cn('flex-1 overflow-y-auto py-3 space-y-4 scrollbar-thin', collapsed ? 'px-2.5' : 'px-3')}>
          {navGroups.map((group, gi) => {
            const items = group.items.filter((item) => !item.visible || item.visible(perms, hasClusters));
            if (items.length === 0) return null;
            return (
              <div key={gi}>
                {group.label && !collapsed && (
                  <div className="px-2.5 pb-1 text-[0.6875rem] font-medium uppercase tracking-[0.08em] text-[var(--muted-foreground)]/80">
                    {t(group.label)}
                  </div>
                )}
                {group.label && collapsed && <div className="mx-1.5 mb-2 border-t border-[var(--border)]" />}
                <ul className="space-y-0.5">
                  {items.map((item) => (
                    <li key={item.href}>
                      <NavLink item={item} collapsed={collapsed} active={isActivePath(location.pathname, item.href)} onClick={onClose} />
                    </li>
                  ))}
                </ul>
              </div>
            );
          })}
        </nav>

        <div className={cn('shrink-0 border-t border-[var(--border)] py-2 space-y-0.5', collapsed ? 'px-2.5' : 'px-3')}>
          <NavLink item={settingsNavItem} collapsed={collapsed} active={isActivePath(location.pathname, settingsNavItem.href)} onClick={onClose} />
          <button
            type="button"
            onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
            className={cn(
              'hidden md:flex h-8 items-center gap-2.5 rounded-md text-[0.8125rem] text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)] transition-colors',
              collapsed ? 'w-8 justify-center' : 'w-full px-2.5',
            )}
            aria-label={sidebarCollapsed ? t('layout.sidebar.expand') : t('layout.sidebar.collapse')}
            title={sidebarCollapsed ? t('layout.sidebar.expand') : t('layout.sidebar.collapse')}
          >
            {collapsed ? <PanelLeftOpen className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
            {!collapsed && <span>{t('layout.sidebar.collapse')}</span>}
          </button>
        </div>
      </aside>
    </TooltipProvider>
  );
}
