import * as React from 'react';
import {
  DndContext,
  KeyboardSensor,
  MeasuringStrategy,
  PointerSensor,
  closestCenter,
  pointerWithin,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { SortableContext, arrayMove, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { ChevronRight, GripVertical, Plus, Trash2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import { useSettingsStore } from '@/store/settings-store';
import { FREE_ROOT, mergeOrder, resolveFree, useDashboardLayout } from '@/store/dashboard-layout-store';
import { HiddenZone, PanelContainer, type PanelDef } from '@/components/dashboard/panel-grid';

export interface RowDef {
  id: string;
  title: string;
  extra?: React.ReactNode;
  panels: PanelDef[];
}

// Drag ids: panels use their own ids, sections are "s:<id>" and panel lists
// (drop targets) are "c:<...>"; panel ids contain neither prefix.
const SECTION = 's:';
const LIST = 'c:';
const HIDDEN = ':hidden';
const isSection = (id: string) => id.startsWith(SECTION);
const isList = (id: string) => id.startsWith(LIST);

type Lists = Record<string, string[]>;

// Custom widths apply from this width up; below it panels use a responsive
// default so a 2-column tile never gets squeezed on a laptop.
const WIDE_QUERY = '(min-width: 1280px)';

function useWide() {
  const [wide, setWide] = React.useState(() => typeof window !== 'undefined' && window.matchMedia(WIDE_QUERY).matches);
  React.useEffect(() => {
    const mq = window.matchMedia(WIDE_QUERY);
    const on = () => setWide(mq.matches);
    mq.addEventListener('change', on);
    return () => mq.removeEventListener('change', on);
  }, []);
  return wide;
}

// Sections only collide with sections; panels prefer the panel under the
// pointer, then the list under it, then the nearest panel.
const collision: CollisionDetection = (args) => {
  const section = isSection(String(args.active.id));
  const a = { ...args, droppableContainers: args.droppableContainers.filter((d) => isSection(String(d.id)) === section) };
  if (section) return closestCenter(a);
  const hits = pointerWithin(a);
  if (hits.length > 0) return [hits.find((h) => !isList(String(h.id))) ?? hits[0]];
  return closestCenter(a);
};

function Section({ id, collapseKey = id, title, extra, editing, sortable, onRename, onRemove, children }: {
  id: string;
  /** Key the collapsed state is stored under (settings-store collapsedRows). */
  collapseKey?: string;
  title: string;
  extra?: React.ReactNode;
  editing: boolean;
  sortable: boolean;
  onRename?: (title: string) => void;
  onRemove?: () => void;
  children: React.ReactNode;
}) {
  const { t } = useTranslation();
  const collapsed = useSettingsStore((s) => s.collapsedRows.includes(collapseKey));
  const toggleRow = useSettingsStore((s) => s.toggleRow);
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: SECTION + id,
    disabled: !editing || !sortable,
  });

  return (
    <section
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition, zIndex: isDragging ? 40 : undefined }}
      // Outside edit mode a section whose panels are all hidden disappears.
      className={cn('relative space-y-2', !editing && !collapsed && '[&:not(:has([data-panel]))]:hidden', isDragging && 'opacity-80')}
    >
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => toggleRow(collapseKey)}
          className="flex shrink-0 items-center gap-1.5 rounded px-1 py-0.5 text-[0.8125rem] font-semibold text-[var(--foreground)] hover:bg-[var(--accent)]"
          aria-expanded={!collapsed}
          aria-label={onRename && editing ? title : undefined}
        >
          <ChevronRight className={cn('h-4 w-4 text-[var(--muted-foreground)] transition-transform', !collapsed && 'rotate-90')} />
          {!(onRename && editing) && title}
        </button>
        {onRename && editing && (
          <input
            value={title}
            onChange={(e) => onRename(e.target.value)}
            placeholder={t('dashboard.edit.groupName')}
            className="min-w-0 max-w-64 rounded border border-[var(--input)] bg-transparent px-1.5 py-0.5 text-[0.8125rem] font-semibold text-[var(--foreground)] focus:border-[var(--primary)] focus:outline-none"
          />
        )}
        <div className="h-px flex-1 bg-[var(--border)]" />
        {extra}
        {editing && onRemove && (
          <button
            type="button"
            onClick={onRemove}
            className="rounded p-1 text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--destructive)]"
            aria-label={t('dashboard.edit.removeGroup')}
            title={t('dashboard.edit.removeGroup')}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        )}
        {editing && sortable && (
          <button
            type="button"
            ref={setActivatorNodeRef}
            {...attributes}
            {...listeners}
            className="cursor-grab touch-none rounded p-1 text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)] active:cursor-grabbing"
            aria-label={t('dashboard.edit.moveGroup')}
            title={t('dashboard.edit.moveGroup')}
          >
            <GripVertical className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      {!collapsed && children}
    </section>
  );
}

/**
 * The dashboard's panels, laid out by the chosen mode. Grouped: the built-in
 * rows, reorderable by their grip, each with its own hidden panels. Free: one
 * list of panels (in grouped order until edited) plus groups the user adds;
 * panels move freely between them and hidden ones share one zone at the end.
 * Hidden and collapsed panels are never rendered, so they fetch nothing.
 */
export function DashboardBoard({ rows, editing }: { rows: RowDef[]; editing: boolean }) {
  const { t } = useTranslation();
  const layout = useDashboardLayout();
  const { mode, groups } = layout;
  const wide = useWide();

  const defs = React.useMemo(() => new Map(rows.flatMap((r) => r.panels.map((p) => [p.id, p] as const))), [rows]);
  const rowIds = React.useMemo(() => mergeOrder(layout.rowOrder, rows.map((r) => r.id)), [layout.rowOrder, rows]);
  const rowsById = React.useMemo(() => new Map(rows.map((r) => [r.id, r])), [rows]);
  const rowOrder = React.useMemo(
    () => Object.fromEntries(rows.map((r) => [r.id, mergeOrder(layout.order[r.id], r.panels.map((p) => p.id))])),
    [rows, layout.order],
  );
  const free = React.useMemo(
    () => (mode === 'free' ? resolveFree(layout.free, groups, rowIds.flatMap((r) => rowOrder[r])) : null),
    [mode, layout.free, groups, rowIds, rowOrder],
  );
  const hiddenSet = React.useMemo(() => new Set(layout.hidden), [layout.hidden]);
  const sections = React.useMemo(() => (free ? [FREE_ROOT, ...groups.map((g) => g.id)] : rowIds), [free, groups, rowIds]);

  // The panel lists as stored; while a panel is dragged, a draft copy follows
  // it between lists and is committed on drop.
  const stored = React.useMemo<Lists>(() => {
    const out: Lists = {};
    if (free) {
      for (const s of sections) out[LIST + s] = free[s].filter((id) => !hiddenSet.has(id));
      out[LIST + HIDDEN] = sections.flatMap((s) => free[s].filter((id) => hiddenSet.has(id)));
    } else {
      for (const r of rowIds) {
        out[LIST + r] = rowOrder[r].filter((id) => !hiddenSet.has(id));
        out[LIST + r + HIDDEN] = rowOrder[r].filter((id) => hiddenSet.has(id));
      }
    }
    return out;
  }, [free, sections, rowIds, rowOrder, hiddenSet]);
  const [draft, setDraft] = React.useState<Lists | null>(null);
  const lists = draft ?? stored;

  const listOf = (id: string, l: Lists) => (isList(id) ? id : Object.keys(l).find((k) => l[k].includes(id)));
  // Grouped rows keep their panels: a panel may only go to its row's hidden zone and back.
  const canMove = (from: string, to: string) => !!free || from.replace(HIDDEN, '') === to.replace(HIDDEN, '');

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const onDragStart = ({ active }: DragStartEvent) => {
    if (!isSection(String(active.id))) setDraft(stored);
  };

  const onDragOver = ({ active, over }: DragOverEvent) => {
    const id = String(active.id);
    if (!over || isSection(id)) return;
    setDraft((cur) => {
      const l = cur ?? stored;
      const overId = String(over.id);
      const from = listOf(id, l);
      const to = listOf(overId, l);
      if (!from || !to || from === to || !canMove(from, to)) return cur;
      const target = l[to].filter((p) => p !== id);
      const at = isList(overId) ? target.length : Math.max(0, target.indexOf(overId));
      target.splice(at, 0, id);
      return { ...l, [from]: l[from].filter((p) => p !== id), [to]: target };
    });
  };

  const commit = (l: Lists, id: string) => {
    const list = listOf(id, l)!;
    const nowHidden = list.endsWith(HIDDEN);
    if (free) {
      const next: Lists = {};
      for (const s of sections) {
        // Hidden panels keep their section, so they come back where they were.
        const keep = free[s].filter((p) => hiddenSet.has(p) && p !== id);
        next[s] = [...l[LIST + s], ...keep];
      }
      if (nowHidden) {
        const home = sections.find((s) => free[s].includes(id)) ?? FREE_ROOT;
        next[home] = [...next[home], id];
      }
      layout.setFree(next);
    } else {
      const row = list.slice(LIST.length).replace(HIDDEN, '');
      layout.setOrder(row, [...l[LIST + row], ...l[LIST + row + HIDDEN]]);
    }
    if (nowHidden !== hiddenSet.has(id)) layout.setHidden(id, nowHidden);
  };

  const onDragEnd = ({ active, over }: DragEndEvent) => {
    const id = String(active.id);
    const l = draft;
    setDraft(null);
    if (!over) return;
    const overId = String(over.id);
    if (isSection(id)) {
      if (!isSection(overId) || id === overId) return;
      const ids = sections.map((s) => SECTION + s);
      const moved = arrayMove(ids, ids.indexOf(id), ids.indexOf(overId)).map((s) => s.slice(SECTION.length));
      if (free) {
        const byId = new Map(groups.map((g) => [g.id, g]));
        layout.setGroups(moved.filter((s) => s !== FREE_ROOT).map((s) => byId.get(s)!));
      } else {
        layout.setRowOrder(moved);
      }
      return;
    }
    if (!l) return;
    const list = listOf(id, l);
    let next = l;
    if (list && !isList(overId) && l[list].includes(overId) && overId !== id) {
      next = { ...l, [list]: arrayMove(l[list], l[list].indexOf(id), l[list].indexOf(overId)) };
    }
    if (next !== stored) commit(next, id);
  };

  // Sections sort among themselves; in free mode the top section stays first.
  const sortableSections = (free ? groups.map((g) => g.id) : rowIds).map((s) => SECTION + s);

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={collision}
      measuring={{ droppable: { strategy: MeasuringStrategy.Always } }}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDragEnd={onDragEnd}
      onDragCancel={() => setDraft(null)}
    >
      <div className="space-y-5">
        {free && (
          <PanelContainer
            id={LIST + FREE_ROOT}
            ids={lists[LIST + FREE_ROOT]}
            defs={defs}
            editing={editing}
            wide={wide}
            placeholder={groups.length > 0 ? t('dashboard.edit.dropHere') : undefined}
          />
        )}
        <SortableContext items={sortableSections} strategy={verticalListSortingStrategy}>
          {free
            ? groups.map((g) => (
                <Section
                  key={g.id}
                  id={g.id}
                  collapseKey={`free:${g.id}`}
                  title={g.title}
                  editing={editing}
                  sortable
                  onRename={(title) => layout.renameGroup(g.id, title)}
                  onRemove={() => layout.removeGroup(g.id)}
                >
                  <PanelContainer id={LIST + g.id} ids={lists[LIST + g.id]} defs={defs} editing={editing} wide={wide} placeholder={t('dashboard.edit.dropHere')} />
                </Section>
              ))
            : rowIds.map((r) => {
                const row = rowsById.get(r)!;
                return (
                  <Section key={r} id={r} title={row.title} extra={row.extra} editing={editing} sortable>
                    <PanelContainer id={LIST + r} ids={lists[LIST + r]} defs={defs} editing={editing} wide={wide} />
                    {editing && <HiddenZone id={LIST + r + HIDDEN} ids={lists[LIST + r + HIDDEN]} defs={defs} wide={wide} />}
                  </Section>
                );
              })}
        </SortableContext>
        {free && editing && (
          <>
            <button
              type="button"
              onClick={() => layout.addGroup(t('dashboard.edit.newGroup'))}
              className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-[var(--input)] py-2 text-[0.8125rem] text-[var(--muted-foreground)] hover:border-[var(--primary)] hover:text-[var(--foreground)]"
            >
              <Plus className="h-3.5 w-3.5" />
              {t('dashboard.edit.addGroup')}
            </button>
            <HiddenZone id={LIST + HIDDEN} ids={lists[LIST + HIDDEN]} defs={defs} wide={wide} />
          </>
        )}
      </div>
    </DndContext>
  );
}
