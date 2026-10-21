import { Database, Key, LayoutDashboard, Plug, Server, Settings } from 'lucide-react';
import type { usePermissions } from '@/hooks/usePermissions';

export interface NavItem {
  title: string;
  href: string;
  icon: React.ComponentType<{ className?: string }>;
  shortcut?: string;
  visible?: (p: ReturnType<typeof usePermissions>, hasClusters: boolean) => boolean;
}

export interface NavGroup {
  label?: string;
  items: NavItem[];
}

export const navGroups: NavGroup[] = [
  {
    items: [{ title: 'nav.dashboard', href: '/', icon: LayoutDashboard, shortcut: 'G D' }],
  },
  {
    label: 'nav.storage',
    items: [
      { title: 'nav.buckets', href: '/buckets', icon: Database, shortcut: 'G B', visible: (p, hasClusters) => hasClusters && p.hasAnyPerm('bucket.list') },
      { title: 'nav.access', href: '/access', icon: Key, shortcut: 'G K', visible: (p, hasClusters) => hasClusters && p.hasClusterPerm('key.list') },
    ],
  },
  {
    label: 'nav.cluster',
    items: [
      { title: 'nav.status', href: '/cluster', icon: Server, shortcut: 'G C', visible: (p, hasClusters) => hasClusters && p.hasAnyClusterAccess },
      { title: 'nav.connections', href: '/connections', icon: Plug, visible: () => true },
    ],
  },
];

export const settingsNavItem: NavItem = { title: 'nav.settings', href: '/settings', icon: Settings, shortcut: 'G S' };

export const SETTINGS_SECTIONS = ['appearance', 'region', 'dashboard', 'monitoring', 'security', 'about'] as const;

export function isActivePath(pathname: string, href: string) {
  return href === '/' ? pathname === '/' : pathname === href || pathname.startsWith(href + '/');
}
