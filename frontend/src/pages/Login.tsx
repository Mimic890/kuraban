import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams, Navigate } from 'react-router-dom';
import { useAuthStore } from '@/store/auth-store';
import { BasicLoginForm } from '@/components/auth/BasicLoginForm';
import { LoadingSpinner } from '@/components/auth/LoadingSpinner';
import { Button } from '@/components/ui/button';
import { authApi } from '@/lib/api';
import { safeReturnUrl } from '@/lib/auth';
import {
  credentialToJSON,
  isWebAuthnCancellation,
  isWebAuthnSupported,
  requestOptionsFromJSON,
  type RequestOptionsJSON,
} from '@/lib/webauthn';
import { useTranslation } from '@/lib/i18n';

export function Login() {
  const { t } = useTranslation();
  const { config, isLoading, initialize, isAuthenticated, isSetup } = useAuthStore();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [passkeyPending, setPasskeyPending] = useState(false);
  const [passkeyError, setPasskeyError] = useState('');

  const loginSuccess = searchParams.get('login');
  const returnUrl = safeReturnUrl(searchParams.get('returnUrl') || (loginSuccess ? sessionStorage.getItem('auth-return-url') : null));

  const signInWithPasskey = async () => {
    if (passkeyPending) return;
    setPasskeyPending(true);
    setPasskeyError('');
    try {
      const begin = await authApi.beginPasskeyLogin();
      const options = begin.publicKey || begin.public_key || begin.options?.publicKey || begin as unknown as RequestOptionsJSON;
      const credential = await navigator.credentials.get({ publicKey: requestOptionsFromJSON(options) });
      if (!credential) return;
      const result = await authApi.finishPasskeyLogin(begin.ceremony_id, credentialToJSON(credential as PublicKeyCredential));
      useAuthStore.getState().setUser(result.user);
      navigate(returnUrl);
    } catch (error) {
      if (!isWebAuthnCancellation(error)) {
        setPasskeyError(t('auth.passkey.errors.signInFailed'));
      }
    } finally {
      setPasskeyPending(false);
    }
  };

  useEffect(() => {
    if (!isSetup) {
      navigate('/setup', { replace: true });
    }
  }, [isSetup, navigate]);

  useEffect(() => {
    if (loginSuccess === 'success') {
      initialize().then(() => {
        sessionStorage.removeItem('auth-return-url');
        navigate(returnUrl);
      });
    }
  }, [loginSuccess, initialize, navigate, returnUrl]);

  useEffect(() => {
    if (isAuthenticated && !loginSuccess) {
      navigate(returnUrl);
    }
  }, [isAuthenticated, navigate, returnUrl, loginSuccess]);

  if (isLoading || loginSuccess === 'success') {
    return <LoadingSpinner />;
  }

  // No auth enabled, redirect to dashboard immediately
  if (config && !config.admin.enabled && !config.oidc.enabled && !config.token.enabled) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-md">
          {config?.admin.enabled && <BasicLoginForm />}
          {config?.token.enabled && <BasicLoginForm tokenMode />}
          {config?.passkey?.enabled && isWebAuthnSupported() && (
            <Button type="button" variant="secondary" className="mt-3 w-full" disabled={passkeyPending} onClick={signInWithPasskey}>
              {passkeyPending ? t('auth.passkey.waiting') : t('auth.passkey.signInAction')}
            </Button>
          )}
          <p role="alert" aria-live="polite" className="mt-2 text-sm text-[var(--destructive)]">{passkeyError}</p>
          {config?.oidc.enabled && (
            <button type="button" className="mt-3 w-full rounded-md border p-2" onClick={() => useAuthStore.getState().loginOIDC(returnUrl)}>
              {t('auth.oidc.signInWith', { provider: config.oidc.provider || 'OIDC' })}
            </button>
          )}
      </div>
    </div>
  );
}
