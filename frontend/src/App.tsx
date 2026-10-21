import { lazy, Suspense, useEffect } from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router-dom';
import {QueryClientProvider} from '@tanstack/react-query';
import {ThemeProvider, useTheme} from '@/components/theme-provider';
import {Layout} from '@/components/layout/layout';
import {BucketDetailShell} from '@/components/layout/bucket-detail-shell';
import {Login} from '@/pages/Login';
import {Setup} from '@/pages/Setup';
import {Toaster} from 'sonner';
import {AppearanceController} from '@/components/appearance';
import {queryClient} from '@/lib/query-client';
import {useAuthStore} from '@/store/auth-store';
import {ProtectedRoute} from '@/components/auth/ProtectedRoute';
import {LoadingSpinner} from '@/components/auth/LoadingSpinner';
import {usePermissions} from '@/hooks/usePermissions';
import {NoAccess} from '@/pages/NoAccess';

// Route-level code splitting keeps the first load small; uPlot, Recharts and
// the object browser only download when their page opens.
const Dashboard = lazy(() => import('@/pages/Dashboard').then((m) => ({ default: m.Dashboard })));
const Buckets = lazy(() => import('@/pages/Buckets').then((m) => ({ default: m.Buckets })));
const BucketObjects = lazy(() => import('@/pages/BucketObjects').then((m) => ({ default: m.BucketObjects })));
const ObjectDetailsView = lazy(() => import('@/components/buckets/ObjectDetailsView').then((m) => ({ default: m.ObjectDetailsView })));
const BucketPermissions = lazy(() => import('@/pages/BucketPermissions').then((m) => ({ default: m.BucketPermissions })));
const BucketWebsite = lazy(() => import('@/pages/BucketWebsite').then((m) => ({ default: m.BucketWebsite })));
const BucketSettings = lazy(() => import('@/pages/BucketSettings').then((m) => ({ default: m.BucketSettings })));
const Cluster = lazy(() => import('@/pages/Cluster').then((m) => ({ default: m.Cluster })));
const AccessControl = lazy(() => import('@/pages/AccessControl').then((m) => ({ default: m.AccessControl })));
const Connections = lazy(() => import('@/pages/Connections').then((m) => ({ default: m.Connections })));
const Settings = lazy(() => import('@/pages/Settings').then((m) => ({ default: m.Settings })));
const UserSettings = lazy(() => import('@/pages/UserSettings').then((m) => ({ default: m.UserSettings })));

function PermissionRoute({ permission, children }: { permission: string; children: React.ReactNode }) {
  const { loading, hasAnyPerm, hasAnyClusterAccess } = usePermissions();
  if (loading) return <LoadingSpinner />;
  if (permission === 'cluster.access' ? !hasAnyClusterAccess : !hasAnyPerm(permission)) return <NoAccess />;
  return <>{children}</>;
}

function ThemedToaster() {
  const { mode } = useTheme();
  return (
    <Toaster
      richColors
      position="bottom-right"
      theme={mode}
      toastOptions={{
        classNames: {
          toast:
            'rounded-lg border border-[var(--border)] bg-[var(--card)] text-[var(--foreground)] font-sans shadow-lg',
          title: 'text-[0.875rem] font-medium',
          description: 'text-[0.8125rem] text-[var(--muted-foreground)]',
        },
      }}
    />
  );
}

function AppRoutes() {
  const { isLoading } = useAuthStore();

  if (isLoading) {
    return <LoadingSpinner />;
  }

  return (
    <Suspense fallback={<LoadingSpinner />}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/setup" element={<Setup />} />

        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="buckets" element={<Buckets />} />
          <Route path="buckets/:bucketName" element={<BucketDetailShell />}>
            <Route index element={<Navigate to="objects" replace />} />
            <Route path="objects" element={<BucketObjects />} />
            <Route path="objects/*" element={<ObjectDetailsView />} />
            <Route path="permissions" element={<BucketPermissions />} />
            <Route path="website" element={<BucketWebsite />} />
            <Route path="settings" element={<BucketSettings />} />
          </Route>
           <Route path="cluster" element={<PermissionRoute permission="cluster.access"><Cluster /></PermissionRoute>} />
          <Route path="connections" element={<PermissionRoute permission="cluster.manage"><Connections /></PermissionRoute>} />
           <Route path="access" element={<PermissionRoute permission="key.list"><AccessControl /></PermissionRoute>} />
          <Route path="settings" element={<Settings />} />
          <Route path="user-settings" element={<UserSettings />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </Suspense>
  );
}

function App() {
  const { initialize } = useAuthStore();

  useEffect(() => {
    initialize();
  }, [initialize]);

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultPalette="graphite" defaultMode="dark">
        <AppearanceController />
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
        <ThemedToaster />
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export default App;
