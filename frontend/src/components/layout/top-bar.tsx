import * as React from 'react';
import { LogOut, Menu, Search, Settings, User } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { ClusterSwitcher } from '@/components/layout/cluster-switcher';
import { ThemeToggle } from '@/components/layout/theme-toggle';
import { useAuthStore } from '@/store/auth-store';
import { useUIStore } from '@/store/ui-store';
import { useTranslation } from '@/lib/i18n';

interface TopBarProps {
  onMenuClick?: () => void;
}

export function TopBar({ onMenuClick }: TopBarProps) {
  const { user, config, logout } = useAuthStore();
  const setPaletteOpen = useUIStore((s) => s.setPaletteOpen);
  const [menuOpen, setMenuOpen] = React.useState(false);
  const menuRef = React.useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const { t } = useTranslation();

  React.useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [menuOpen]);

  const hasUser = !!(config && (config.admin.enabled || config.oidc.enabled) && user);
  const displayName = user?.name || user?.username || '';

  return (
    <header className="sticky top-0 z-30 flex h-12 w-full shrink-0 items-center gap-2 border-b border-[var(--border)] bg-[var(--background)]/85 px-3 backdrop-blur md:px-4">
      {onMenuClick && (
        <Button variant="ghost" size="icon-sm" className="md:hidden" onClick={onMenuClick} aria-label={t('layout.topBar.toggleNavigation')}>
          <Menu className="h-4 w-4" />
        </Button>
      )}
      <div className="min-w-0 flex items-center">
        <ClusterSwitcher />
      </div>
      <div className="flex-1" />
      <button
        type="button"
        onClick={() => setPaletteOpen(true)}
        className="hidden sm:flex h-8 w-56 items-center gap-2 rounded-md border border-[var(--border)] bg-[var(--card)] px-2.5 text-[0.8125rem] text-[var(--muted-foreground)] transition-colors hover:border-[var(--input)] hover:text-[var(--foreground)]"
      >
        <Search className="h-3.5 w-3.5" />
        <span className="flex-1 text-left">{t('palette.searchButton')}</span>
        <kbd className="rounded border border-[var(--border)] px-1 text-[0.625rem]">Ctrl K</kbd>
      </button>
      <Button variant="ghost" size="icon-sm" className="sm:hidden" onClick={() => setPaletteOpen(true)} aria-label={t('palette.title')}>
        <Search className="h-4 w-4" />
      </Button>
      <ThemeToggle />
      {hasUser && (
        <div ref={menuRef} className="relative">
          <button
            type="button"
            onClick={() => setMenuOpen((o) => !o)}
            className="flex h-8 items-center gap-2 rounded-md px-1.5 text-[0.8125rem] text-[var(--foreground)] hover:bg-[var(--accent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
          >
            <span className="flex h-6 w-6 items-center justify-center rounded-full border border-[var(--accent-primary-border)] bg-[var(--accent-primary-soft)] text-[0.6875rem] font-semibold uppercase text-[var(--primary)]">
              {displayName ? displayName.slice(0, 1) : <User className="h-3.5 w-3.5" />}
            </span>
            <span className="hidden max-w-[140px] truncate lg:inline">{displayName}</span>
          </button>
          {menuOpen && (
            <div role="menu" className="absolute right-0 mt-1 w-56 overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--popover)] py-1 shadow-xl">
              <div className="border-b border-[var(--border)] px-3 py-2">
                <div className="truncate text-[0.8125rem] font-medium">{displayName}</div>
                {user?.email && <div className="truncate text-[0.75rem] text-[var(--muted-foreground)]">{user.email}</div>}
              </div>
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); navigate('/user-settings'); }} className="flex w-full items-center gap-2 px-3 py-2 text-left text-[0.8125rem] hover:bg-[var(--accent)]">
                <User className="h-3.5 w-3.5" /> {t('settings.user')}
              </button>
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); navigate('/settings'); }} className="flex w-full items-center gap-2 px-3 py-2 text-left text-[0.8125rem] hover:bg-[var(--accent)]">
                <Settings className="h-3.5 w-3.5" /> {t('nav.settings')}
              </button>
              <div className="my-1 border-t border-[var(--border)]" />
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); logout(); }} className="flex w-full items-center gap-2 px-3 py-2 text-left text-[0.8125rem] text-[var(--destructive)] hover:bg-[var(--accent)]">
                <LogOut className="h-3.5 w-3.5" /> {t('auth.logoutAction')}
              </button>
            </div>
          )}
        </div>
      )}
    </header>
  );
}
