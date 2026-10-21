import { useState } from 'react';
import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { IconTile } from '@/components/ui/icon-tile';
import type { S3Object } from '@/types';
import { useTranslation } from '@/lib/i18n';

interface DeleteObjectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  object: S3Object | null;
  onDeleteObject: (key: string) => Promise<boolean>;
}

export function DeleteObjectDialog({ open, onOpenChange, object, onDeleteObject }: DeleteObjectDialogProps) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);

  const handleDelete = async () => {
    if (!object) return;

    setPending(true);
    try {
      const success = await onDeleteObject(object.key);
      if (success) {
        onOpenChange(false);
      }
    } finally {
      setPending(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange} size="destructive">
      <DialogContent>
        <DialogHeader>
          <IconTile icon={<Trash2 />} tone="destructive" size="md" />
          <div className="flex-1">
            <DialogTitle>{t('buckets.delete_object_dialog.title')}</DialogTitle>
            <DialogDescription>
              {t('buckets.delete_object_dialog.description', { key: object?.key ?? '' })}
            </DialogDescription>
          </div>
        </DialogHeader>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('buckets.actions.cancel')}
          </Button>
          <Button variant="destructive" onClick={handleDelete} disabled={pending}>
            {t('buckets.actions.delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
