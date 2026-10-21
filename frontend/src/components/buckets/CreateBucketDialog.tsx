import { useEffect, useState } from 'react';
import { Database } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
import { toast } from 'sonner';
import { useTranslation } from '@/lib/i18n';

interface CreateBucketDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreateBucket: (name: string) => Promise<boolean>;
}

export function CreateBucketDialog({ open, onOpenChange, onCreateBucket }: CreateBucketDialogProps) {
  const { t } = useTranslation();
  const [bucketName, setBucketName] = useState('');
  const [pending, setPending] = useState(false);

  useEffect(() => { if (!open) { setBucketName(''); setPending(false); } }, [open]);

  const handleCreate = async () => {
    if (!bucketName || pending) {
      toast.error(t('buckets.create_dialog.errors.name_required'));
      return;
    }

    setPending(true);
    try {
      const success = await onCreateBucket(bucketName);
      if (success) {
        setBucketName('');
        onOpenChange(false);
      }
    } finally {
      setPending(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <IconTile icon={<Database />} tone="primary" size="md" />
          <div className="flex-1">
            <DialogTitle>{t('buckets.create_dialog.title')}</DialogTitle>
            <DialogDescription>
              {t('buckets.create_dialog.description')}
            </DialogDescription>
          </div>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t('buckets.create_dialog.name.label')}</label>
            <Input
              autoFocus
              placeholder={t('buckets.create_dialog.name.placeholder')}
              value={bucketName}
              onChange={(e) => setBucketName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  handleCreate();
                }
              }}
            />
            <p className="text-xs text-muted-foreground">
              {t('buckets.create_dialog.name.help')}
            </p>
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('buckets.actions.cancel')}
          </Button>
          <Button
            variant="primary"
            onClick={handleCreate}
            disabled={!bucketName || pending}
          >
            {t('buckets.actions.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
