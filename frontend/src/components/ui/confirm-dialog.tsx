import * as React from 'react';
import { Trash2, AlertTriangle } from 'lucide-react';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './dialog';
import { IconTile } from './icon-tile';
import { Button } from './button';
import { useTranslation } from '@/lib/i18n';

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  icon?: React.ReactNode;
  tone?: 'destructive' | 'primary';
  loading?: boolean;
  onConfirm: () => void | Promise<void>;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel,
  icon,
  tone = 'destructive',
  loading = false,
  onConfirm,
}: ConfirmDialogProps) {
  const { t } = useTranslation();
  const defaultIcon = tone === 'destructive' ? <Trash2 /> : <AlertTriangle />;
  return (
    <Dialog open={open} onOpenChange={onOpenChange} size="destructive">
      <DialogContent>
        <DialogHeader>
          <IconTile icon={icon ?? defaultIcon} tone={tone} size="md" />
          <div className="flex-1">
            <DialogTitle>{title}</DialogTitle>
            {description && <DialogDescription>{description}</DialogDescription>}
          </div>
        </DialogHeader>
        <DialogBody>
          <p className="text-[0.8438rem] text-[var(--muted-foreground)]">{t('common.confirm.irreversible')}</p>
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            {cancelLabel ?? t('common.actions.cancel')}
          </Button>
          <Button
            variant={tone === 'destructive' ? 'destructive' : 'primary'}
            onClick={() => onConfirm()}
            disabled={loading}
          >
            {loading ? t('common.status.working') : confirmLabel ?? t('common.actions.delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
