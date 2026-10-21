import * as React from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { CornerDownLeft, Database, Globe, LogOut, Moon, Search, SlidersHorizontal, Sun, type LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useUIStore } from '@/store/ui-store';
import { useClusterStore } from '@/store/cluster-store';
import { useAuthStore } from '@/store/auth-store';
import { usePermissions } from '@/hooks/usePermissions';
import { useBuckets } from '@/hooks/useApi';
import { useTheme } from '@/components/theme-provider';
import { useTranslation } from '@/lib/i18n';
import { navGroups, settingsNavItem, SETTINGS_SECTIONS } from './nav';

interface Command {
  id: string;
  group: string;
  label: string;
  hint?: string;
  icon: LucideIcon | React.ComponentType<{ className?: string }>;
  keywords?: string;
  run: () => void;
}

function fuzzyScore(text: string, query: string): number {
  const t = text.toLowerCase();
  const q = query.toLowerCase().trim();
  if (!q) return 1;
  const idx = t.indexOf(q);
  if (idx >= 0) return 100 - idx;
  let ti = 0;
  for (const ch of q) {
    ti = t.indexOf(ch, ti);
    if (ti < 0) return 0;
    ti++;
  }
  return 10;
}

export function CommandPalette() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { paletteOpen: open, setPaletteOpen: setOpen } = useUIStore();
  const { clusters, activeClusterId, setActiveCluster } = useClusterStore();
  const { logout, user } = useAuthStore();
  const perms = usePermissions();
  const { mode, setMode } = useTheme();
  const hasClusters = clusters.length > 0;
  const { data: buckets = [] } = useBuckets(open && hasClusters && perms.hasAnyPerm('bucket.list'));
  const [query, setQuery] = React.useState('');
  const [active, setActive] = React.useState(0);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const listRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setOpen(!useUIStore.getState().paletteOpen);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [setOpen]);

  React.useEffect(() => {
    if (open) {
      setQuery('');
      setActive(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  const commands = React.useMemo<Command[]>(() => {
    const go = (href: string) => () => navigate(href);
    const list: Command[] = [];
    for (const group of navGroups) {
      for (const item of group.items) {
        if (item.visible && !item.visible(perms, hasClusters)) continue;
        list.push({ id: `nav:${item.href}`, group: t('palette.group.navigation'), label: t(item.title), hint: item.shortcut, icon: item.icon, run: go(item.href) });
      }
    }
    list.push({ id: 'nav:settings', group: t('palette.group.navigation'), label: t(settingsNavItem.title), hint: settingsNavItem.shortcut, icon: settingsNavItem.icon, run: go('/settings') });
    for (const section of SETTINGS_SECTIONS) {
      list.push({ id: `settings:${section}`, group: t('palette.group.settings'), label: t(`settings.section.${section}`), keywords: 'settings настройки', icon: SlidersHorizontal, run: go(`/settings?section=${section}`) });
    }
    for (const b of buckets) {
      list.push({ id: `bucket:${b.name}`, group: t('palette.group.buckets'), label: b.name, keywords: 'bucket бакет', icon: Database, run: go(`/buckets/${encodeURIComponent(b.name)}/objects`) });
    }
    for (const c of clusters) {
      if (c.id === activeClusterId) continue;
      list.push({ id: `cluster:${c.id}`, group: t('palette.group.clusters'), label: t('palette.switchCluster', { name: c.name }), keywords: 'cluster кластер', icon: Globe, run: () => setActiveCluster(c.id) });
    }
    list.push({
      id: 'theme',
      group: t('palette.group.actions'),
      label: mode === 'dark' ? t('palette.lightMode') : t('palette.darkMode'),
      keywords: 'theme тема dark light',
      icon: mode === 'dark' ? Sun : Moon,
      run: () => setMode(mode === 'dark' ? 'light' : 'dark'),
    });
    if (user) {
      list.push({ id: 'logout', group: t('palette.group.actions'), label: t('auth.logoutAction'), icon: LogOut, run: () => logout() });
    }
    return list;
  }, [t, perms, hasClusters, buckets, clusters, activeClusterId, mode, user, navigate, setActiveCluster, setMode, logout]);

  const results = React.useMemo(() => {
    return commands
      .map((c) => ({ c, score: Math.max(fuzzyScore(c.label, query), fuzzyScore(`${c.group} ${c.keywords ?? ''}`, query) / 2) }))
      .filter((r) => r.score > 0)
      .sort((a, b) => (query ? b.score - a.score : 0))
      .slice(0, 40)
      .map((r) => r.c);
  }, [commands, query]);

  React.useEffect(() => setActive(0), [query]);
  React.useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)?.scrollIntoView({ block: 'nearest' });
  }, [active]);

  if (!open) return null;

  const run = (c: Command | undefined) => {
    if (!c) return;
    setOpen(false);
    c.run();
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, results.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      run(results[active]);
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  };

  return createPortal(
    <div className="fixed inset-0 z-[60] flex items-start justify-center px-4 pt-[12vh]" role="dialog" aria-modal="true" aria-label={t('palette.title')}>
      <div className="absolute inset-0 bg-black/50 backdrop-blur-[2px]" onClick={() => setOpen(false)} />
      <div className="relative w-full max-w-xl overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--popover)] shadow-2xl" onKeyDown={onKeyDown}>
        <div className="flex items-center gap-2.5 border-b border-[var(--border)] px-4">
          <Search className="h-4 w-4 shrink-0 text-[var(--muted-foreground)]" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t('palette.placeholder')}
            className="h-12 w-full bg-transparent text-[0.875rem] outline-none placeholder:text-[var(--muted-foreground)]"
          />
          <kbd className="rounded border border-[var(--border)] px-1.5 py-0.5 text-[0.6875rem] text-[var(--muted-foreground)]">Esc</kbd>
        </div>
        <div ref={listRef} className="max-h-[min(60vh,420px)] overflow-y-auto p-1.5 scrollbar-thin">
          {results.length === 0 && (
            <div className="px-3 py-8 text-center text-[0.8125rem] text-[var(--muted-foreground)]">{t('palette.empty')}</div>
          )}
          {results.map((c, i) => {
            const Icon = c.icon;
            const header = i === 0 || results[i - 1].group !== c.group ? c.group : null;
            return (
              <React.Fragment key={c.id}>
                {header && <div className="px-2.5 pb-1 pt-2 text-[0.6875rem] font-medium uppercase tracking-[0.08em] text-[var(--muted-foreground)]">{header}</div>}
                <button
                  type="button"
                  data-index={i}
                  onMouseMove={() => setActive(i)}
                  onClick={() => run(c)}
                  className={cn(
                    'flex w-full items-center gap-3 rounded-md px-2.5 py-2 text-left text-[0.8125rem]',
                    i === active ? 'bg-[var(--accent)] text-[var(--foreground)]' : 'text-[var(--muted-foreground)]',
                  )}
                >
                  <Icon className={cn('h-4 w-4 shrink-0', i === active && 'text-[var(--primary)]')} />
                  <span className="flex-1 truncate">{c.label}</span>
                  {c.hint && <span className="text-[0.6875rem] text-[var(--muted-foreground)]">{c.hint}</span>}
                  {i === active && <CornerDownLeft className="h-3.5 w-3.5 text-[var(--muted-foreground)]" />}
                </button>
              </React.Fragment>
            );
          })}
        </div>
        <div className="flex items-center gap-4 border-t border-[var(--border)] px-4 py-2 text-[0.6875rem] text-[var(--muted-foreground)]">
          <span><kbd className="font-mono">↑↓</kbd> {t('palette.navigate')}</span>
          <span><kbd className="font-mono">↵</kbd> {t('palette.open')}</span>
          <span className="ml-auto"><kbd className="font-mono">Ctrl K</kbd></span>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/** Two-key "G then X" navigation shortcuts, ignored while typing. */
export function useGlobalShortcuts() {
  const navigate = useNavigate();
  React.useEffect(() => {
    let pendingG = 0;
    const map: Record<string, string> = { d: '/', b: '/buckets', k: '/access', c: '/cluster', s: '/settings' };
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      if (target && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))) return;
      const key = e.key.toLowerCase();
      if (key === 'g') {
        pendingG = Date.now();
        return;
      }
      if (Date.now() - pendingG < 1200 && map[key]) {
        pendingG = 0;
        navigate(map[key]);
      } else if (key === '/' ) {
        e.preventDefault();
        useUIStore.getState().setPaletteOpen(true);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [navigate]);
}
