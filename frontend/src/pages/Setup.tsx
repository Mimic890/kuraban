import { useState } from 'react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Box } from 'lucide-react';
import { authApi } from '@/lib/api';
import { toast } from 'sonner';
import { useTranslation } from '@/lib/i18n';

export function Setup() {
  const { t } = useTranslation();
  const [nickname, setNickname] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSetup = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!nickname.trim()) {
      toast.error(t('setup.errors.nicknameRequired'));
      return;
    }

    setIsLoading(true);
    try {
      await authApi.setupPanel({ nickname, password });
      toast.success(t('setup.success.completed'));
      window.location.href = '/login';
    } catch (error) {
      const message = (error as { response?: { data?: { error?: { message?: string } } } }).response?.data?.error?.message;
      toast.error(message || t('setup.errors.failed'));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-[400px] space-y-6">
        <div className="flex flex-col items-center space-y-2 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <Box className="h-6 w-6" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">{t('setup.title')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('setup.welcome')}
          </p>
        </div>

        <Card className="border-border shadow-sm">
          <CardHeader>
            <CardTitle>{t('setup.form.title')}</CardTitle>
            <CardDescription>
              {t('setup.form.description')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSetup} className="space-y-4">
              <div className="space-y-2">
                <label htmlFor="nickname" className="text-sm font-medium">{t('setup.form.nicknameLabel')}</label>
                <Input
                  id="nickname"
                  placeholder={t('setup.form.nicknamePlaceholder')}
                  value={nickname}
                  onChange={(e) => setNickname(e.target.value)}
                  disabled={isLoading}
                  autoFocus
                />
              </div>

              <div className="space-y-2">
                  <label htmlFor="password" className="text-sm font-medium">{t('setup.form.passwordLabel')}</label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={isLoading}
                />
                <p className="text-xs text-muted-foreground">
                  {t('setup.form.passwordHelp')}
                </p>
              </div>

              <Button type="submit" className="w-full" disabled={isLoading || !nickname.trim() || password.length < 12}>
                {isLoading ? t('setup.form.submitting') : t('setup.form.submitAction')}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
