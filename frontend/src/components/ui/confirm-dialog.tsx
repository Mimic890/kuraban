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
  /** Replaces the default "cannot be undone" line, for actions that can be. */
  note?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  icon?: React.ReactNode;
  tone?: 'destructive' | 'primary';
  loading?: boolean;
  onConfirm: () => void | Promise<void>;
}

type GuardedDialogProps = React.ComponentProps<typeof Dialog> & {
  /** Something would be lost: an overlay click, Esc or the X ask first. Cancel buttons still close at once. */
  dirty: boolean;
  /** Replaces the default "changes won't be saved" line. */
  warning?: string;
};

export function GuardedDialog({ dirty, warning, onOpenChange, ...props }: GuardedDialogProps) {
  const { t } = useTranslation();
  const [asking, setAsking] = React.useState(false);
  return (
    <>
      <Dialog
        {...props}
        onOpenChange={(next) => {
          if (!next && dirty) setAsking(true);
          else onOpenChange?.(next);
        }}
      />
      <ConfirmDialog
        open={asking}
        onOpenChange={setAsking}
        title={t('common.confirm.close_title')}
        note={warning ?? t('common.confirm.close_note')}
        icon={<AlertTriangle />}
        cancelLabel={t('common.confirm.close_stay')}
        confirmLabel={t('common.confirm.close_leave')}
        onConfirm={() => {
          setAsking(false);
          onOpenChange?.(false);
        }}
      />
    </>
  );
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  note,
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
          <div className="min-w-0 flex-1 pr-6">
            <DialogTitle>{title}</DialogTitle>
            {description && <DialogDescription>{description}</DialogDescription>}
          </div>
        </DialogHeader>
        <DialogBody>
          <p className="text-[0.8438rem] text-[var(--muted-foreground)]">{note ?? t('common.confirm.irreversible')}</p>
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
