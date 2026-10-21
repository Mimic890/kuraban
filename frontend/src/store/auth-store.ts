import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { AuthConfig, AuthUser, AuthState, LoginResult } from '@/types/auth';
import { authApi } from '@/lib/api';
import { safeReturnUrl } from '@/lib/auth';
import { translate } from '@/lib/i18n';

interface AuthStore extends AuthState {
  config: AuthConfig | null;
  isSetup: boolean;
  mfaChallengeId: string | null;

  // Actions
  setUser: (user: AuthUser | null) => void;
  setConfig: (config: AuthConfig) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  setAuthenticated: (authenticated: boolean) => void;

  // Async actions
  initialize: () => Promise<void>;
  loginAdmin: (username: string, password: string) => Promise<LoginResult>;
  loginMfa: (code: string) => Promise<void>;
  loginToken: (token: string) => Promise<void>;
  loginOIDC: (returnUrl?: string) => void;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthStore>()(
  persist(
    (set) => ({
      user: null,
      config: null,
      isSetup: true,
      isAuthenticated: false,
      isLoading: true,
      error: null,
      mfaChallengeId: null,

      setUser: (user) => set({ user, isAuthenticated: !!user }),
      setConfig: (config) => set({ config }),
      setLoading: (isLoading) => set({ isLoading }),
      setError: (error) => set({ error }),
      setAuthenticated: (isAuthenticated) => set({ isAuthenticated }),

      initialize: async () => {
        try {
          set({ isLoading: true, error: null, mfaChallengeId: null });

          // Fetch auth configuration and setup status
          const [configResponse, setupResponse] = await Promise.all([
            authApi.getConfig(),
            authApi.getSetupStatus(),
          ]);
          const config = configResponse.data as AuthConfig;
          const isSetup = setupResponse.data.setup as boolean;
          set({ config, isSetup });

          if (!isSetup) {
            set({ user: null, isAuthenticated: false, isLoading: false });
            return;
          }

          // If no auth is enabled, mark as authenticated immediately
          if (!config.admin.enabled && !config.oidc.enabled && !config.token.enabled) {
            set({
              isAuthenticated: true,
              isLoading: false,
              user: { username: 'guest' }
            });
            return;
          }

          // Try to get current user (check if already authenticated)
          try {
            const userResponse = await authApi.me();
            const user = userResponse.data.user;
            set({
              user,
              isAuthenticated: true,
              isLoading: false
            });
          } catch {
            // Not authenticated - this is okay, but don't clobber a login
            // that happened while we were still checking.
            if (!useAuthStore.getState().isAuthenticated) {
              set({
                user: null,
                isAuthenticated: false,
                isLoading: false
              });
            } else {
              set({ isLoading: false });
            }
          }
        } catch (error) {
          console.error('Failed to initialize auth:', error);
          if (!useAuthStore.getState().isAuthenticated) {
            set({
              error: translate('auth.errors.initializeFailed'),
              isLoading: false,
              isAuthenticated: false
            });
          } else {
            set({ error: translate('auth.errors.initializeFailed'), isLoading: false });
          }
        }
      },

      loginAdmin: async (username, password) => {
        try {
          set({ isLoading: true, error: null, mfaChallengeId: null });

          const response = await authApi.loginAdmin(username, password);
          const { user, mfa_required, challenge_id } = response.data;

          if (mfa_required && challenge_id) {
            set({ mfaChallengeId: challenge_id, isLoading: false, error: null });
            return { status: 'mfa-required' };
          }

          if (!user) throw new Error(translate('auth.errors.missingUser'));

          // Update state
          set({
            user: { ...user, auth_method: user.auth_method || 'admin' },
            isAuthenticated: true,
            isLoading: false,
            error: null,
            mfaChallengeId: null,
          });
          return { status: 'authenticated' };
        } catch (error) {
          const errorMessage = translate('auth.errors.loginFailed');
          set({
            error: errorMessage,
            isLoading: false,
            isAuthenticated: false,
            user: null,
            mfaChallengeId: null,
          });
          throw error; // Re-throw for form handling
        }
      },

      loginMfa: async (code) => {
        const challengeId = useAuthStore.getState().mfaChallengeId;
        if (!challengeId) throw new Error(translate('auth.errors.challengeExpired'));
        try {
          set({ isLoading: true, error: null });
          const response = await authApi.loginMfa(challengeId, code);
          set({
            user: { ...response.data.user, auth_method: response.data.user.auth_method || 'admin' },
            isAuthenticated: true,
            isLoading: false,
            error: null,
            mfaChallengeId: null,
          });
        } catch (error) {
          const errorMessage = translate('auth.errors.verificationFailed');
          set({ error: errorMessage, isLoading: false, isAuthenticated: false, mfaChallengeId: null });
          throw error;
        }
      },

      loginToken: async (token) => {
        try {
          set({ isLoading: true, error: null });

          const response = await authApi.loginToken(token);
          const { user } = response.data;

          set({
            user: { ...user, auth_method: user.auth_method || 'token' },
            isAuthenticated: true,
            isLoading: false,
            error: null,
          });
        } catch (error) {
          const errorMessage = translate('auth.errors.loginFailed');
          set({
            error: errorMessage,
            isLoading: false,
            isAuthenticated: false,
            user: null,
          });
          throw error;
        }
      },

      loginOIDC: (returnUrl = '/') => {
        // Redirect to OIDC login endpoint
        sessionStorage.setItem('auth-return-url', safeReturnUrl(returnUrl));
        window.location.href = '/auth/oidc/login';
      },

      logout: async () => {
        try {
          // Call logout endpoint for OIDC mode
          await authApi.logoutAdmin();
        } catch (error) {
          console.error('Logout API call failed:', error);
        }

        // Clear state
        set({
          user: null,
          isAuthenticated: false,
          error: null,
          mfaChallengeId: null,
        });

        // Redirect to login page
        window.location.href = '/login';
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        user: state.user,
        // Don't persist config, isLoading, or error
      }),
    }
  )
);
