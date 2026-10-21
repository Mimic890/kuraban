import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface PanelSize {
  /** Width in columns of a 12-column row (applies on wide screens). */
  w: number;
  /** Body height in pixels (the chart itself for time series). */
  h: number;
}

/**
 * grouped: panels live in the built-in rows (Overview, S3 API, ...), which can
 * be reordered. free: no built-in rows; panels sit in one list and can be
 * sorted into groups the user creates.
 */
export type LayoutMode = 'grouped' | 'free';

export interface FreeGroup {
  id: string;
  title: string;
}

/** The free layout's top section, which has no header. */
export const FREE_ROOT = '_';

interface DashboardLayoutStore {
  mode: LayoutMode;
  /** Built-in row ids in display order (grouped mode). */
  rowOrder: string[];
  /** Panel ids per built-in row in display order; ids not listed keep their default place. */
  order: Record<string, string[]>;
  /** User groups in display order (free mode). */
  groups: FreeGroup[];
  /** Panel ids per free section (FREE_ROOT or a group id); null until the free layout is first edited. */
  free: Record<string, string[]> | null;
  hidden: string[];
  sizes: Record<string, PanelSize>;
  setMode: (mode: LayoutMode) => void;
  setRowOrder: (ids: string[]) => void;
  setOrder: (row: string, ids: string[]) => void;
  setFree: (free: Record<string, string[]>) => void;
  setGroups: (groups: FreeGroup[]) => void;
  addGroup: (title: string) => string;
  renameGroup: (id: string, title: string) => void;
  /** Removes a group; its panels move to the end of the top section. */
  removeGroup: (id: string) => void;
  setHidden: (id: string, hidden: boolean) => void;
  setSize: (id: string, size: PanelSize) => void;
  /** Restores the default layout; the chosen mode is kept. */
  reset: () => void;
}

type LayoutData = Pick<DashboardLayoutStore, 'rowOrder' | 'order' | 'groups' | 'free' | 'hidden' | 'sizes'>;
const initial: LayoutData = { rowOrder: [], order: {}, groups: [], free: null, hidden: [], sizes: {} };

export const useDashboardLayout = create<DashboardLayoutStore>()(
  persist(
    (set) => ({
      mode: 'grouped',
      ...initial,
      setMode: (mode) => set({ mode }),
      setRowOrder: (rowOrder) => set({ rowOrder }),
      setOrder: (row, ids) => set((s) => ({ order: { ...s.order, [row]: ids } })),
      setFree: (free) => set({ free }),
      setGroups: (groups) => set({ groups }),
      addGroup: (title) => {
        const id = `g${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
        set((s) => ({ groups: [...s.groups, { id, title }] }));
        return id;
      },
      renameGroup: (id, title) => set((s) => ({ groups: s.groups.map((g) => (g.id === id ? { ...g, title } : g)) })),
      removeGroup: (id) =>
        set((s) => {
          const free = { ...(s.free ?? {}) };
          const moved = free[id] ?? [];
          delete free[id];
          free[FREE_ROOT] = [...(free[FREE_ROOT] ?? []), ...moved];
          return { groups: s.groups.filter((g) => g.id !== id), free };
        }),
      setHidden: (id, hidden) =>
        set((s) => ({ hidden: hidden ? [...s.hidden.filter((h) => h !== id), id] : s.hidden.filter((h) => h !== id) })),
      setSize: (id, size) => set((s) => ({ sizes: { ...s.sizes, [id]: size } })),
      reset: () => set(initial),
    }),
    {
      name: 'dashboard-layout',
      version: 2,
      // v1 had only order/hidden/sizes; the new fields take their defaults.
      migrate: (state) => ({ mode: 'grouped', ...initial, ...(state as Partial<LayoutData>) }) as unknown as DashboardLayoutStore,
    },
  ),
);

/**
 * Applies a saved order to the current panel ids: unknown saved ids are
 * dropped, and panels added since (or never moved) are slotted in after the
 * panel that precedes them by default.
 */
export function mergeOrder(saved: string[] | undefined, defaults: string[]): string[] {
  const known = new Set(defaults);
  const out = (saved ?? []).filter((id) => known.has(id));
  defaults.forEach((id, i) => {
    if (out.includes(id)) return;
    const prev = defaults
      .slice(0, i)
      .reverse()
      .find((p) => out.includes(p));
    out.splice(prev ? out.indexOf(prev) + 1 : 0, 0, id);
  });
  return out;
}

/**
 * Resolves the free layout against the current panels. `defaults` is every
 * panel in grouped order; panels not saved in any section (new ones, or all
 * of them before the first edit) land in the top section at their default
 * spot, so the free layout starts out as the grouped one without headers.
 */
export function resolveFree(saved: Record<string, string[]> | null, groups: FreeGroup[], defaults: string[]): Record<string, string[]> {
  const known = new Set(defaults);
  const seen = new Set<string>();
  const out: Record<string, string[]> = {};
  for (const s of [FREE_ROOT, ...groups.map((g) => g.id)]) {
    out[s] = (saved?.[s] ?? []).filter((id) => {
      if (!known.has(id) || seen.has(id)) return false;
      seen.add(id);
      return true;
    });
  }
  const inGroups = new Set(groups.flatMap((g) => out[g.id]));
  out[FREE_ROOT] = mergeOrder(out[FREE_ROOT], defaults.filter((id) => !inGroups.has(id)));
  return out;
}
