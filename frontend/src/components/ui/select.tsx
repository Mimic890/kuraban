import * as React from 'react';
import { createPortal } from 'react-dom';
import {cn} from '@/lib/utils';
import {ChevronDown, Check} from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps {
  value?: string;
  onChange?: (value: string) => void;
  children?: React.ReactNode;
  className?: string;
  disabled?: boolean;
  placeholder?: string;
}

const SelectContext = React.createContext<{
  value?: string;
  onChange?: (value: string) => void;
  open: boolean;
  setOpen: (open: boolean) => void;
} | null>(null);

const useSelectContext = () => {
  const context = React.useContext(SelectContext);
  if (!context) {
    throw new Error('Select components must be used within a Select');
  }
  return context;
};

// Menu geometry for a fixed-position list, so a scrolling/clipping ancestor
// (Dialog is overflow-y-auto) cannot cut the options off. Exported for tests.
export function computeMenuPosition(
  rect: { top: number; bottom: number; left: number; width: number },
  viewportHeight: number,
  margin = 8
) {
  const below = viewportHeight - rect.bottom - margin;
  const above = rect.top - margin;
  const flip = below < 160 && above > below;
  return {
    left: rect.left,
    width: rect.width,
    maxHeight: Math.max(flip ? above : below, 120),
    ...(flip ? { bottom: viewportHeight - rect.top + 4 } : { top: rect.bottom + 4 }),
  };
}

const Select = React.forwardRef<HTMLButtonElement, SelectProps>(
  ({ className, children, value, onChange, disabled, placeholder, ...props }, _ref) => {
    const { t } = useTranslation();
    const resolvedPlaceholder = placeholder ?? t('common.select.placeholder');
    const [open, setOpen] = React.useState(false);
    const [internalValue, setInternalValue] = React.useState(value);
    const [menuStyle, setMenuStyle] = React.useState<React.CSSProperties>({});
    const buttonRef = React.useRef<HTMLButtonElement>(null);
    const listRef = React.useRef<HTMLDivElement>(null);
    const listboxId = React.useId();

    const displayValue = React.useMemo(() => {
      const currentValue = value ?? internalValue;
       if (!currentValue) return resolvedPlaceholder;

      // Extract label from children
      const options = React.Children.toArray(children);
      const selectedOption = options.find((child) => {
        if (React.isValidElement<SelectOptionProps>(child) && child.type === SelectOption) {
          return child.props.value === currentValue;
        }
        return false;
      });

      if (React.isValidElement<SelectOptionProps>(selectedOption)) {
        return selectedOption.props.children;
      }

      return currentValue;
    }, [value, internalValue, children, resolvedPlaceholder]);

    React.useEffect(() => {
      setInternalValue(value);
    }, [value]);

    React.useLayoutEffect(() => {
      if (!open) return;

      const update = () => {
        const trigger = buttonRef.current;
        if (!trigger) return;
        setMenuStyle({
          position: 'fixed',
          backgroundColor: 'var(--popover)',
          ...computeMenuPosition(trigger.getBoundingClientRect(), window.innerHeight),
        });
      };

      update();
      window.addEventListener('scroll', update, true);
      window.addEventListener('resize', update);

      return () => {
        window.removeEventListener('scroll', update, true);
        window.removeEventListener('resize', update);
      };
    }, [open]);

    React.useEffect(() => {
      const handleClickOutside = (event: MouseEvent) => {
        const target = event.target as Node;
        if (buttonRef.current?.contains(target) || listRef.current?.contains(target)) return;
        setOpen(false);
      };

      if (open) {
        document.addEventListener('mousedown', handleClickOutside);
      }

      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
      };
    }, [open]);

    const handleChange = (newValue: string) => {
      setInternalValue(newValue);
      onChange?.(newValue);
      setOpen(false);
    };

    return (
      <SelectContext.Provider value={{ value: value ?? internalValue, onChange: handleChange, open, setOpen }}>
        <div className="relative">
          <button
            ref={(node) => {
              buttonRef.current = node;
              if (typeof _ref === 'function') _ref(node);
              else if (_ref) (_ref as React.MutableRefObject<HTMLButtonElement | null>).current = node;
            }}
            type="button"
            className={cn(
              'w-full h-10 px-3 py-2 text-sm rounded-md border border-input bg-background text-foreground',
              'flex items-center justify-between',
              'ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
              'disabled:cursor-not-allowed disabled:opacity-50',
              !internalValue && !value && 'text-muted-foreground',
              className
            )}
            onClick={() => !disabled && setOpen(!open)}
            aria-haspopup="listbox"
            aria-expanded={open}
            aria-controls={open ? listboxId : undefined}
            disabled={disabled}
            {...props}
          >
            <span className="truncate">{displayValue}</span>
            <ChevronDown className={cn('h-4 w-4 opacity-50 transition-transform', open && 'transform rotate-180')} />
          </button>

          {open &&
            createPortal(
              <div
                role="listbox"
                id={listboxId}
                ref={listRef}
                aria-label={resolvedPlaceholder}
                className="z-[60] text-popover-foreground rounded-md border border-border shadow-lg overflow-y-auto overscroll-contain"
                style={menuStyle}
              >
                {children}
              </div>,
              document.body
            )}
        </div>
      </SelectContext.Provider>
    );
  }
);
Select.displayName = 'Select';

export interface SelectOptionProps {
  value: string;
  children: React.ReactNode;
  className?: string;
  disabled?: boolean;
}

const SelectOption = React.forwardRef<HTMLDivElement, SelectOptionProps>(
  ({ className, children, value: optionValue, disabled, ...props }, ref) => {
    const { value, onChange } = useSelectContext();
    const isSelected = value === optionValue;

    return (
      <div
        role="option"
        aria-selected={isSelected}
        tabIndex={disabled ? -1 : 0}
        ref={ref}
        className={cn(
          'relative flex items-center w-full px-3 py-2 text-sm cursor-pointer select-none bg-transparent',
          'transition-colors',
          'hover:bg-accent hover:text-accent-foreground',
          'focus:bg-accent focus:text-accent-foreground',
          isSelected && 'bg-accent text-accent-foreground',
          disabled && 'pointer-events-none opacity-50',
          className
        )}
        onClick={() => {
          if (!disabled) {
            onChange?.(optionValue);
          }
        }}
        onKeyDown={(event) => {
          if (!disabled && (event.key === 'Enter' || event.key === ' ')) {
            event.preventDefault();
            onChange?.(optionValue);
          }
        }}
        {...props}
      >
        <span className="flex-1">{children}</span>
        {isSelected && <Check className="h-4 w-4 ml-2" />}
      </div>
    );
  }
);
SelectOption.displayName = 'SelectOption';

export { Select, SelectOption };
