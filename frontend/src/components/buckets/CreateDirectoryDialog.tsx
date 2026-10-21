import { useEffect, useState } from 'react';
import { FolderPlus } from 'lucide-react';
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

interface CreateDirectoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentPath: string;
  onCreateDirectory: (name: string) => Promise<boolean>;
}

export function CreateDirectoryDialog({ open, onOpenChange, currentPath, onCreateDirectory }: CreateDirectoryDialogProps) {
  const { t } = useTranslation();
  const [dirName, setDirName] = useState('');
  const [pending, setPending] = useState(false);

  useEffect(() => { if (!open) { setDirName(''); setPending(false); } }, [open]);

  const handleCreate = async () => {
    if (!dirName) {
      toast.error(t('buckets.directory_dialog.errors.name_required'));
      return;
    }

    setPending(true);
    try {
      const success = await onCreateDirectory(dirName);
      if (success) {
        setDirName('');
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
          <IconTile icon={<FolderPlus />} tone="primary" size="md" />
          <div className="flex-1">
            <DialogTitle>{t('buckets.directory_dialog.title')}</DialogTitle>
            <DialogDescription>
              {t('buckets.directory_dialog.description', { location: currentPath || t('buckets.common.root') })}
            </DialogDescription>
          </div>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">{t('buckets.directory_dialog.name.label')}</label>
            <Input
              autoFocus
              placeholder={t('buckets.directory_dialog.name.placeholder')}
              value={dirName}
              onChange={(e) => setDirName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  handleCreate();
                }
              }}
            />
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('buckets.actions.cancel')}
          </Button>
          <Button onClick={handleCreate} disabled={!dirName || pending}>
            {t('buckets.actions.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
