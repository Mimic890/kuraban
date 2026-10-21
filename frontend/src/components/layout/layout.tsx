import { Outlet, useLocation } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Sidebar } from './sidebar';
import { TopBar } from './top-bar';
import { CommandPalette, useGlobalShortcuts } from './command-palette';
import { usePermissions } from '@/hooks/usePermissions';
import { NoAccess } from '@/pages/NoAccess';

export function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const { noAccess } = usePermissions();
  const location = useLocation();
  useGlobalShortcuts();

  useEffect(() => setSidebarOpen(false), [location.pathname]);

  return (
    <div className="flex h-screen overflow-hidden bg-[var(--background)]">
      <Sidebar isOpen={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar onMenuClick={() => setSidebarOpen(!sidebarOpen)} />
        <main className="min-h-0 flex-1 overflow-y-auto scrollbar-thin">
          {noAccess ? <NoAccess /> : <Outlet />}
        </main>
      </div>
      <CommandPalette />
    </div>
  );
}
