import * as React from 'react';
import { Trash2 } from 'lucide-react';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { IconTile } from '@/components/ui/icon-tile';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { useTranslation } from '@/lib/i18n';

interface DeleteBucketDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bucketName: string;
  objectCount: number;
  onEmptyBucket: () => Promise<void>;
  onDeleteBucket: () => Promise<void>;
}

export function DeleteBucketDialog({
  open,
  onOpenChange,
  bucketName,
  objectCount,
  onEmptyBucket,
  onDeleteBucket,
}: DeleteBucketDialogProps) {
  const { t, language } = useTranslation();
  const [value, setValue] = React.useState('');
  const [confirmed, setConfirmed] = React.useState(false);
  const [highlight, setHighlight] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [status, setStatus] = React.useState('');
  const [error, setError] = React.useState('');

  const matches = value === bucketName;

  React.useEffect(() => {
    if (!open) {
      setValue('');
      setConfirmed(false);
      setHighlight(false);
      setLoading(false);
      setStatus('');
      setError('');
    }
  }, [open]);

  const submit = async () => {
    if (!matches || loading) return;

    if (!confirmed) {
      setHighlight(true);
      return;
    }

    setLoading(true);
    setError('');

    setStatus(t('buckets.delete_dialog.status.deleting_objects'));
    try {
      await onEmptyBucket();
    } catch {
      setError(t('buckets.delete_dialog.errors.delete_objects_failed'));
      setLoading(false);
      setStatus('');
      return;
    }

    setStatus(t('buckets.delete_dialog.status.deleting_bucket'));
    try {
      await onDeleteBucket();
    } catch {
      setError(t('buckets.delete_dialog.errors.delete_bucket_failed'));
      setLoading(false);
      setStatus('');
      return;
    }

    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange} size="destructive">
      <DialogContent>
        <DialogHeader>
          <IconTile icon={<Trash2 />} tone="destructive" size="md" />
          <div className="flex-1">
            <DialogTitle>{t('buckets.delete_dialog.title', { bucket: bucketName })}</DialogTitle>
            <DialogDescription>{t('buckets.delete_dialog.description')}</DialogDescription>
          </div>
        </DialogHeader>
        <DialogBody className="space-y-4">
          {/* Data confirmation checkbox */}
          <label
            className={`flex items-start gap-2.5 rounded-lg border px-3 py-2.5 transition-colors ${
              highlight && !confirmed
                  ? 'border-[var(--destructive)] bg-[var(--destructive)]/5'
                  : 'border-[var(--border)] cursor-pointer'
            }`}
          >
            <Checkbox
              checked={confirmed}
              onCheckedChange={(c) => {
                setConfirmed(c);
                if (c) setHighlight(false);
              }}
              className="mt-0.5"
            />
            <span className="text-[0.8438rem] select-none">
              {t('buckets.delete_dialog.confirm_data')}
              <span className="text-[var(--muted-foreground)]">
                {' '}{t('buckets.delete_dialog.reported_objects', { count: objectCount.toLocaleString(language) })}
              </span>
            </span>
          </label>
          {highlight && !confirmed && (
            <p className="text-[0.8125rem] text-[var(--destructive)]">
              {t('buckets.delete_dialog.errors.confirm_data_first')}
            </p>
          )}

          {/* Confirmation text input */}
          <div className="space-y-2">
            <p className="text-[0.8438rem] text-[var(--muted-foreground)]">
              {t('buckets.delete_dialog.type_to_confirm')}{' '}
              <code className="rounded bg-[var(--surface-sunken)] px-1 py-0.5 font-mono text-[0.8125rem] text-[var(--foreground)]">
                {bucketName}
              </code>{' '}
              {t('buckets.delete_dialog.below')}
            </p>
            <Input
              autoFocus
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && submit()}
              placeholder={bucketName}
              aria-label={t('buckets.delete_dialog.input_aria', { bucket: bucketName })}
            />
          </div>

          {error && (
            <p className="text-[0.8125rem] text-[var(--destructive)]">{error}</p>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            {t('buckets.actions.cancel')}
          </Button>
          <Button
            variant="destructive"
            onClick={submit}
            disabled={!matches || loading}
          >
            {loading ? status || t('buckets.common.working') : t('buckets.actions.delete_bucket')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
