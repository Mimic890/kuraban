import * as React from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';

type DialogSize = 'standard' | 'form' | 'destructive';

interface DialogContextValue {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  size: DialogSize;
  titleId: string;
  descriptionId: string;
}

const DialogContext = React.createContext<DialogContextValue | undefined>(undefined);

function useDialog() {
  const ctx = React.useContext(DialogContext);
  if (!ctx) throw new Error('useDialog must be used within a Dialog');
  return ctx;
}

interface DialogProps {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  size?: DialogSize;
  children: React.ReactNode;
}

const Dialog: React.FC<DialogProps> = ({ open = false, onOpenChange, size = 'standard', children }) => {
  const id = React.useId();
  return (
    <DialogContext.Provider value={{ open, onOpenChange: onOpenChange || (() => {}), size, titleId: `${id}-title`, descriptionId: `${id}-description` }}>
      {children}
    </DialogContext.Provider>
  );
};

const DialogTrigger = React.forwardRef<HTMLButtonElement, React.ButtonHTMLAttributes<HTMLButtonElement>>(
  ({ onClick, ...props }, ref) => {
    const { onOpenChange } = useDialog();
    return (
      <button
        ref={ref}
        onClick={(e) => { onOpenChange(true); onClick?.(e); }}
        {...props}
      />
    );
  }
);
DialogTrigger.displayName = 'DialogTrigger';

// Mobile: full-screen sheet. sm+: viewport-relative box, capped so a wide screen
// doesn't turn the dialog into mostly empty space.
const widthClass: Record<DialogSize, string> = {
  standard: 'sm:max-w-[480px]',
  form: 'sm:max-w-[600px]',
  destructive: 'sm:max-w-[440px]',
};

const DialogOverlay: React.FC = () => {
  const { onOpenChange } = useDialog();
  return (
    <div
      className="fixed inset-0 z-50 bg-black/55 backdrop-blur-[8px]"
      onClick={() => onOpenChange(false)}
    />
  );
};

const DialogContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => {
    const { t } = useTranslation();
    const { open, onOpenChange, size, titleId, descriptionId } = useDialog();
    const containerRef = React.useRef<HTMLDivElement>(null);

    React.useEffect(() => {
      if (!open) return;
      const previouslyFocused = document.activeElement as HTMLElement | null;

      const keyHandler = (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
          e.stopPropagation();
          onOpenChange(false);
          return;
        }
        if (e.key !== 'Tab' || !containerRef.current) return;
        const focusables = containerRef.current.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );
        if (focusables.length === 0) return;
        const first = focusables[0];
        const last = focusables[focusables.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      };
      document.addEventListener('keydown', keyHandler);

      setTimeout(() => {
        const first = containerRef.current?.querySelector<HTMLElement>(
          'input:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );
        first?.focus();
      }, 0);

      return () => {
        document.removeEventListener('keydown', keyHandler);
        previouslyFocused?.focus?.();
      };
    }, [open, onOpenChange]);

    if (!open) return null;

    return createPortal(
      <>
        <DialogOverlay />
        <div
          ref={containerRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          aria-describedby={descriptionId}
          className={cn(
            'fixed inset-0 z-50',
            'sm:inset-auto sm:left-1/2 sm:top-1/2 sm:w-[calc(100%-2rem)] sm:-translate-x-1/2 sm:-translate-y-1/2',
            widthClass[size],
          )}
        >
          <div
            ref={ref}
            className={cn(
              'relative flex h-full flex-col overflow-hidden border border-[var(--border)]',
              'sm:h-auto sm:max-h-[min(88vh,760px)] sm:rounded-xl',
              'bg-[var(--card)] text-[var(--card-foreground)]',
              'shadow-[0_20px_40px_rgba(0,0,0,0.3)]',
              className,
            )}
            {...props}
          >
            {children}
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              aria-label={t('common.dialog.close')}
              className="absolute right-3 top-3 inline-flex h-7 w-7 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--accent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>
      </>,
      document.body,
    );
  }
);
DialogContent.displayName = 'DialogContent';

const DialogHeader: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ className, ...props }) => (
  <div
    className={cn(
      'flex shrink-0 items-start gap-3 border-b border-[var(--border)] px-6 py-5',
      className,
    )}
    {...props}
  />
);
DialogHeader.displayName = 'DialogHeader';

const DialogBody: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ className, ...props }) => (
  <div className={cn('min-h-0 flex-1 overflow-y-auto px-6 py-5', className)} {...props} />
);
DialogBody.displayName = 'DialogBody';

const DialogFooter: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ className, ...props }) => (
  <div
    className={cn(
      'mt-auto flex shrink-0 justify-end gap-2 border-t border-[var(--border)] bg-[var(--surface-sunken)] px-6 py-3.5',
      'pb-[max(0.875rem,env(safe-area-inset-bottom))] sm:pb-3.5',
      className,
    )}
    {...props}
  />
);
DialogFooter.displayName = 'DialogFooter';

const DialogTitleText = React.forwardRef<HTMLHeadingElement, React.HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => {
    const { titleId } = useDialog();
    return (
    <h2
      ref={ref}
      id={titleId}
      className={cn('text-[1.25rem] font-semibold tracking-[-0.015em] leading-tight', className)}
      {...props}
    />
    );
  }
);
DialogTitleText.displayName = 'DialogTitle';

const DialogDescription = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => {
    const { descriptionId } = useDialog();
    return (
    <p
      ref={ref}
      id={descriptionId}
      className={cn('mt-1 text-[0.8438rem] leading-[1.45] text-[var(--muted-foreground)]', className)}
      {...props}
    />
    );
  }
);
DialogDescription.displayName = 'DialogDescription';

export {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogBody,
  DialogFooter,
  DialogTitleText as DialogTitle,
  DialogDescription,
};
