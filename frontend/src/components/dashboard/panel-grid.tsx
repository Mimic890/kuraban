import * as React from 'react';
import { useDroppable } from '@dnd-kit/core';
import { SortableContext, rectSortingStrategy, useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Eye, EyeOff, GripVertical } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import { PanelSlotContext, type PanelSlot } from '@/components/dashboard/panels';
import { useDashboardLayout, type PanelSize } from '@/store/dashboard-layout-store';

export interface PanelDef {
  id: string;
  /** Shown on the compact stub that stands in for a hidden panel in edit mode. */
  title: string;
  /** Default width in columns of 12 (wide screens). */
  w: number;
  /** Default body height in pixels. */
  h: number;
  minH?: number;
  /** Small tiles: two per row on phones, three on tablets. */
  compact?: boolean;
  /** Receives the current size so fixed-layout panels can size their body. */
  render: (size: PanelSize) => React.ReactNode;
}

const COLS = 12;
const H_STEP = 10;
const MAX_H = 900;
const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

function ResizeHandle({ size, minH, gridRef, onPreview, onCommit }: {
  size: PanelSize;
  minH: number;
  gridRef: React.RefObject<HTMLDivElement | null>;
  onPreview: (s: PanelSize | null) => void;
  onCommit: (s: PanelSize) => void;
}) {
  const { t } = useTranslation();
  const start = React.useRef<{ x: number; y: number; size: PanelSize; col: number; last: PanelSize } | null>(null);

  const onPointerDown = (e: React.PointerEvent<HTMLButtonElement>) => {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.setPointerCapture(e.pointerId);
    const width = gridRef.current?.clientWidth ?? 1200;
    start.current = { x: e.clientX, y: e.clientY, size, col: width / COLS, last: size };
  };
  const onPointerMove = (e: React.PointerEvent) => {
    const s = start.current;
    if (!s) return;
    const next = {
      w: clamp(Math.round(s.size.w + (e.clientX - s.x) / s.col), 1, COLS),
      h: clamp(Math.round((s.size.h + e.clientY - s.y) / H_STEP) * H_STEP, minH, MAX_H),
    };
    if (next.w !== s.last.w || next.h !== s.last.h) {
      s.last = next;
      onPreview(next);
    }
  };
  const onPointerUp = () => {
    const s = start.current;
    start.current = null;
    if (!s) return;
    onPreview(null);
    if (s.last.w !== s.size.w || s.last.h !== s.size.h) onCommit(s.last);
  };
  const onKeyDown = (e: React.KeyboardEvent) => {
    const d = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -H_STEP], ArrowDown: [0, H_STEP] }[e.key];
    if (!d) return;
    e.preventDefault();
    onCommit({ w: clamp(size.w + d[0], 1, COLS), h: clamp(size.h + d[1], minH, MAX_H) });
  };

  return (
    <button
      type="button"
      aria-label={t('dashboard.edit.resize')}
      title={t('dashboard.edit.resize')}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onKeyDown={onKeyDown}
      className="absolute bottom-0 right-0 z-10 flex h-5 w-5 cursor-nwse-resize touch-none items-end justify-end p-1 text-[var(--muted-foreground)] hover:text-[var(--primary)] focus-visible:text-[var(--primary)] focus-visible:outline-none"
    >
      <svg viewBox="0 0 10 10" className="h-2.5 w-2.5" aria-hidden>
        <path d="M9 1 1 9M9 5 5 9" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" fill="none" />
      </svg>
    </button>
  );
}

function SortablePanel({ def, size, hidden, editing, wide, gridRef }: {
  def: PanelDef;
  size: PanelSize;
  hidden: boolean;
  editing: boolean;
  wide: boolean;
  gridRef: React.RefObject<HTMLDivElement | null>;
}) {
  const { t } = useTranslation();
  const setHidden = useDashboardLayout((s) => s.setHidden);
  const setSize = useDashboardLayout((s) => s.setSize);
  const [preview, setPreview] = React.useState<PanelSize | null>(null);
  const [expanded, setExpanded] = React.useState(false);
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: def.id, disabled: !editing });

  React.useEffect(() => {
    if (editing) setExpanded(false);
  }, [editing]);

  const cur = preview ?? size;
  const span = expanded ? COLS : cur.w;

  const slot = React.useMemo<PanelSlot>(
    () => ({
      height: cur.h,
      editing,
      inactive: hidden,
      expanded,
      toggleExpanded: () => setExpanded((e) => !e),
      controls: (
        <div className="flex items-center gap-0.5">
          <button
            type="button"
            onClick={() => setHidden(def.id, !hidden)}
            className="rounded p-1 text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)]"
            aria-label={hidden ? t('dashboard.edit.show') : t('dashboard.edit.hide')}
            title={hidden ? t('dashboard.edit.show') : t('dashboard.edit.hide')}
          >
            {hidden ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </button>
          <button
            type="button"
            ref={setActivatorNodeRef}
            {...attributes}
            {...listeners}
            className="cursor-grab touch-none rounded p-1 text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)] active:cursor-grabbing"
            aria-label={t('dashboard.edit.move')}
            title={t('dashboard.edit.move')}
          >
            <GripVertical className="h-3.5 w-3.5" />
          </button>
        </div>
      ),
      resizeHandle: (
        <ResizeHandle size={size} minH={def.minH ?? 60} gridRef={gridRef} onPreview={setPreview} onCommit={(s) => setSize(def.id, s)} />
      ),
    }),
    [cur.h, editing, hidden, expanded, def.id, def.minH, size, gridRef, setHidden, setSize, setActivatorNodeRef, attributes, listeners, t],
  );

  const style: React.CSSProperties = {
    transform: CSS.Translate.toString(transform),
    transition,
    zIndex: isDragging ? 30 : undefined,
    gridColumn: wide || expanded ? `span ${span} / span ${span}` : undefined,
  };

  // A hidden panel is never rendered: in edit mode a title-only stub stands
  // in for it, so it neither fetches nor re-renders on refresh.
  if (hidden) {
    return (
      <div
        ref={setNodeRef}
        style={{ transform: CSS.Translate.toString(transform), transition, zIndex: isDragging ? 30 : undefined }}
        data-panel={def.id}
        className={cn('col-span-6 min-w-0 sm:col-span-4 xl:col-span-3', isDragging && 'opacity-80 shadow-lg')}
      >
        <div className="flex h-9 items-center gap-1.5 rounded-lg border border-dashed border-[var(--input)] bg-[var(--card)]/60 pl-3 pr-1.5 text-[var(--muted-foreground)]">
          <span className="min-w-0 flex-1 truncate text-[0.8125rem]">{def.title}</span>
          {slot.controls}
        </div>
      </div>
    );
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      data-panel={def.id}
      className={cn(
        'relative min-w-0',
        !wide && !expanded && (def.compact ? 'col-span-6 sm:col-span-4' : def.w >= COLS ? 'col-span-12' : 'col-span-12 lg:col-span-6'),
        !wide && expanded && 'col-span-12',
        isDragging && 'opacity-80 shadow-lg',
      )}
    >
      <PanelSlotContext.Provider value={slot}>{def.render(cur)}</PanelSlotContext.Provider>
    </div>
  );
}

/**
 * One sortable list of panels on a 12-column grid, and a drop target for
 * panels dragged in from another list. The DndContext lives in the board.
 */
export function PanelContainer({ id, ids, defs, editing, hidden = false, wide, placeholder }: {
  id: string;
  ids: string[];
  defs: Map<string, PanelDef>;
  editing: boolean;
  hidden?: boolean;
  wide: boolean;
  /** Shown in edit mode while the list is empty. */
  placeholder?: string;
}) {
  const sizes = useDashboardLayout((s) => s.sizes);
  const gridRef = React.useRef<HTMLDivElement>(null);
  const { setNodeRef, isOver } = useDroppable({ id, disabled: !editing });
  const setRefs = React.useCallback(
    (el: HTMLDivElement | null) => {
      gridRef.current = el;
      setNodeRef(el);
    },
    [setNodeRef],
  );

  if (!editing && ids.length === 0) return null;
  return (
    <SortableContext id={id} items={ids} strategy={rectSortingStrategy}>
      <div ref={setRefs} className="grid grid-cols-12 gap-2">
        {ids.map((pid) => {
          const def = defs.get(pid)!;
          const size = { w: sizes[pid]?.w ?? def.w, h: sizes[pid]?.h ?? def.h };
          return <SortablePanel key={pid} def={def} size={size} hidden={hidden} editing={editing} wide={wide} gridRef={gridRef} />;
        })}
        {editing && ids.length === 0 && placeholder && (
          <div
            className={cn(
              'col-span-12 rounded-md border border-dashed border-[var(--border)] py-4 text-center text-[0.75rem] text-[var(--muted-foreground)]',
              isOver && 'border-[var(--primary)] text-[var(--foreground)]',
            )}
          >
            {placeholder}
          </div>
        )}
      </div>
    </SortableContext>
  );
}

/** Hidden panels, shown only in edit mode under a dashed line. */
export function HiddenZone({ id, ids, defs, wide }: { id: string; ids: string[]; defs: Map<string, PanelDef>; wide: boolean }) {
  const { t } = useTranslation();
  return (
    <div className="mt-3 border-t border-dashed border-[var(--input)] pt-3">
      <div className="mb-2 text-[0.6875rem] uppercase tracking-wide text-[var(--muted-foreground)]">{t('dashboard.edit.hiddenPanels')}</div>
      <PanelContainer id={id} ids={ids} defs={defs} editing hidden wide={wide} placeholder={t('dashboard.edit.dropToHide')} />
    </div>
  );
}
