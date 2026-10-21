import * as React from 'react';
import {createPortal} from 'react-dom';
import {cn} from '@/lib/utils';

interface DropdownMenuContextValue {
  open: boolean;
  setOpen: (open: boolean) => void;
  triggerRef: React.RefObject<HTMLButtonElement | null>;
}

const DropdownMenuContext = React.createContext<DropdownMenuContextValue | undefined>(undefined);

function useDropdownMenu() {
  const context = React.useContext(DropdownMenuContext);
  if (!context) {
    throw new Error('useDropdownMenu must be used within a DropdownMenu');
  }
  return context;
}

interface DropdownMenuProps {
  children: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

const DropdownMenu: React.FC<DropdownMenuProps> = ({ children, open: controlledOpen, onOpenChange }) => {
  const [internalOpen, setInternalOpen] = React.useState(false);
  const open = controlledOpen ?? internalOpen;
  const setOpen = React.useCallback((next: boolean) => {
    if (controlledOpen === undefined) setInternalOpen(next);
    onOpenChange?.(next);
  }, [controlledOpen, onOpenChange]);
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  return (
    <DropdownMenuContext.Provider value={{ open, setOpen, triggerRef }}>
      <div className="relative inline-block text-left">{children}</div>
    </DropdownMenuContext.Provider>
  );
};

const DropdownMenuTrigger = React.forwardRef<
  HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement> & { asChild?: boolean }
>(({ onClick, asChild, children, ...props }, ref) => {
  const { open, setOpen, triggerRef } = useDropdownMenu();

  // Merge the forwarded ref with the context triggerRef
  React.useImperativeHandle(ref, () => triggerRef.current as HTMLButtonElement);

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
    setOpen(!open);
    onClick?.(e);
  };

  // asChild wraps an existing button in a non-interactive span so we never
  // render an invalid <button> inside <button>.
  if (asChild) {
    return (
      <span
        ref={triggerRef as unknown as React.Ref<HTMLSpanElement>}
        className="inline-flex"
        onClick={handleClick as unknown as React.MouseEventHandler<HTMLSpanElement>}
      >
        {children}
      </span>
    );
  }

  return (
    <button ref={triggerRef} onClick={handleClick} {...props}>
      {children}
    </button>
  );
});
DropdownMenuTrigger.displayName = 'DropdownMenuTrigger';

interface DropdownMenuContentProps extends React.HTMLAttributes<HTMLDivElement> {
  align?: 'start' | 'end' | 'center';
}

const DropdownMenuContent = React.forwardRef<HTMLDivElement, DropdownMenuContentProps>(
  ({ className, children, align = 'start', ...props }, ref) => {
    const { open, setOpen, triggerRef } = useDropdownMenu();
    const contentRef = React.useRef<HTMLDivElement>(null);
    React.useImperativeHandle(ref, () => contentRef.current as HTMLDivElement);
    const [position, setPosition] = React.useState({ top: 0, left: 0 });

    // Fixed positioning relative to the trigger, measured after render so
    // wide menus align correctly and never leave the viewport.
    React.useLayoutEffect(() => {
      const updatePosition = () => {
        if (!open || !triggerRef.current) return;
        const rect = triggerRef.current.getBoundingClientRect();
        const width = contentRef.current?.offsetWidth ?? 224;
        const height = contentRef.current?.offsetHeight ?? 0;
        let left = rect.left;
        if (align === 'end') left = rect.right - width;
        else if (align === 'center') left = rect.left + rect.width / 2 - width / 2;
        left = Math.max(8, Math.min(left, window.innerWidth - width - 8));
        let top = rect.bottom + 6;
        if (height && top + height > window.innerHeight - 8 && rect.top - height - 6 > 8) top = rect.top - height - 6;
        setPosition({ top, left });
      };

      updatePosition();

      if (open) {
        window.addEventListener('scroll', updatePosition, true);
        window.addEventListener('resize', updatePosition);
      }

      return () => {
        window.removeEventListener('scroll', updatePosition, true);
        window.removeEventListener('resize', updatePosition);
      };
    }, [open, align, triggerRef]);

    React.useEffect(() => {
      if (!open) return;
      const onKey = (e: KeyboardEvent) => {
        if (e.key === 'Escape') setOpen(false);
      };
      document.addEventListener('keydown', onKey);
      return () => document.removeEventListener('keydown', onKey);
    }, [open, setOpen]);

    React.useEffect(() => {
      const handleClickOutside = (event: MouseEvent) => {
        const isClickOnTrigger = triggerRef.current?.contains(event.target as Node);
        const isClickOnContent = contentRef.current?.contains(event.target as Node);

        if (!isClickOnContent && !isClickOnTrigger) {
          setOpen(false);
        }
      };

      if (open) {
        document.addEventListener('mousedown', handleClickOutside);
      }

      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
      };
    }, [open, setOpen, triggerRef]);

    if (!open) return null;

    const content = (
      <div
        ref={contentRef}
        style={{
          backgroundColor: 'var(--popover)',
          position: 'fixed',
          top: `${position.top}px`,
          left: `${position.left}px`,
        }}
        className={cn(
          'z-50 w-56 origin-top-right rounded-lg text-popover-foreground shadow-xl border border-border focus:outline-none',
          className
        )}
        {...props}
      >
        <div className="py-1">{children}</div>
      </div>
    );

    return createPortal(content, document.body);
  }
);
DropdownMenuContent.displayName = 'DropdownMenuContent';

interface DropdownMenuItemProps extends React.HTMLAttributes<HTMLDivElement> {
  closeOnClick?: boolean;
}

const DropdownMenuItem = React.forwardRef<HTMLDivElement, DropdownMenuItemProps>(
  ({ className, onClick, closeOnClick = true, ...props }, ref) => {
    const { setOpen } = useDropdownMenu();
    return (
      <div
        ref={ref}
        className={cn(
          'relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none [&_svg]:h-4 [&_svg]:w-4 [&_svg]:shrink-0',
          className
        )}
        onClick={(e) => {
          onClick?.(e);
          if (closeOnClick) {
            setOpen(false);
          }
        }}
        {...props}
      />
    );
  }
);
DropdownMenuItem.displayName = 'DropdownMenuItem';

const DropdownMenuSeparator = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('-mx-1 my-1 h-px bg-muted', className)} {...props} />
  )
);
DropdownMenuSeparator.displayName = 'DropdownMenuSeparator';

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
};
