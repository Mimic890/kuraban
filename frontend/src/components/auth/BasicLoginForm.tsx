import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/store/auth-store';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { safeReturnUrl } from '@/lib/auth';
import { useTranslation } from '@/lib/i18n';
export function BasicLoginForm({ tokenMode = false }: { tokenMode?: boolean }) {
  const { t } = useTranslation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [mfaRequired, setMfaRequired] = useState(false);
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { loginAdmin, loginMfa, loginToken } = useAuthStore();

  const returnUrl = safeReturnUrl(searchParams.get('returnUrl'));
  const prefix = tokenMode ? 'token-' : '';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isLoading) return;
    setIsLoading(true);
    setError('');

    try {
      if (mfaRequired) {
        await loginMfa(code);
      } else if (tokenMode) {
        await loginToken(password);
      } else {
        const result = await loginAdmin(username, password);
        if (result.status === 'mfa-required') {
          setMfaRequired(true);
          setPassword('');
          return;
        }
      }
      // Navigate to return URL on success
      navigate(returnUrl);
    } catch {
      setError(useAuthStore.getState().error || t('auth.login.errors.failedRetry'));
      if (mfaRequired && !useAuthStore.getState().mfaChallengeId) {
        setMfaRequired(false);
        setCode('');
      }
    } finally {
      setPassword('');
      setIsLoading(false);
    }
  };

  return (
    <Card className="w-full">
      <CardHeader className="space-y-1">
        <div className="flex items-center justify-center mb-4">
          <img
            src="/garage.png"
            alt={t('auth.login.logoAlt')}
            className="h-16 w-16 object-contain"
          />
        </div>
        <CardTitle className="text-2xl text-center">
          {t('auth.login.title')}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          {!mfaRequired && <div className="space-y-2">
            {!tokenMode && <label htmlFor={`${prefix}username`} className="text-sm font-medium">{t('auth.login.usernameLabel')}</label>}
            <Input
              id={`${prefix}username`}
              type="text"
              placeholder={tokenMode ? t('auth.login.tokenUsernamePlaceholder') : t('auth.login.usernamePlaceholder')}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required={!tokenMode}
              disabled={isLoading}
              autoComplete="username"
            />
          </div>}
          {!mfaRequired && <div className="space-y-2">
            <label htmlFor={`${prefix}password`} className="text-sm font-medium">{tokenMode ? t('auth.login.tokenLabel') : t('auth.login.passwordLabel')}</label>
            <Input
              id={`${prefix}password`}
              type="password"
              placeholder={tokenMode ? t('auth.login.tokenPlaceholder') : t('auth.login.passwordPlaceholder')}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={isLoading}
              autoComplete={tokenMode ? 'off' : 'current-password'}
              required
            />
          </div>}
          {mfaRequired && <div className="space-y-2">
            <p className="text-sm text-[var(--muted-foreground)]">{t('auth.login.mfaInstructions')}</p>
            <label htmlFor="mfa-code" className="text-sm font-medium">{t('auth.login.mfaCodeLabel')}</label>
            <Input
              id="mfa-code"
              value={code}
              onChange={(event) => setCode(event.target.value.trim())}
              autoComplete="one-time-code"
              disabled={isLoading}
              required
              autoFocus
            />
          </div>}
          <p role="alert" aria-live="polite" className="min-h-5 text-sm text-[var(--destructive)]">{error}</p>
          <Button
            type="submit"
            className="w-full"
            disabled={isLoading || (mfaRequired ? !code : tokenMode ? !password : !username || !password)}
          >
            {isLoading ? t('auth.login.signingIn') : mfaRequired ? t('auth.login.verifyAction') : t('auth.login.signInAction')}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
