import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type Language = 'en' | 'ru';
export type FontSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';
export type Density = 'compact' | 'comfortable';

export const FONT_SIZE_PX: Record<FontSize, number> = { xs: 13, sm: 14, md: 15, lg: 16, xl: 17 };

interface SettingsStore {
  timezone: string;
  language: Language;
  fontSans: string;
  fontMono: string;
  fontSize: FontSize;
  density: Density;
  reduceMotion: boolean;
  hour12: boolean;
  sidebarCollapsed: boolean;
  dashboardRange: string;
  dashboardRefresh: string;
  collapsedRows: string[];
  setTimezone: (timezone: string) => void;
  setLanguage: (language: Language) => void;
  setFontSans: (id: string) => void;
  setFontMono: (id: string) => void;
  setFontSize: (size: FontSize) => void;
  setDensity: (density: Density) => void;
  setReduceMotion: (value: boolean) => void;
  setHour12: (value: boolean) => void;
  setSidebarCollapsed: (value: boolean) => void;
  setDashboardRange: (value: string) => void;
  setDashboardRefresh: (value: string) => void;
  toggleRow: (id: string) => void;
  resetAppearance: () => void;
}

export const APPEARANCE_DEFAULTS = {
  fontSans: 'jetbrains-mono',
  fontMono: 'jetbrains-mono',
  fontSize: 'md' as FontSize,
  density: 'comfortable' as Density,
  reduceMotion: false,
};

export const useSettingsStore = create<SettingsStore>()(
  persist(
    (set) => ({
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      language: 'en',
      ...APPEARANCE_DEFAULTS,
      hour12: false,
      sidebarCollapsed: false,
      dashboardRange: '1h',
      dashboardRefresh: 'auto',
      collapsedRows: [],
      setTimezone: (timezone) => set({ timezone }),
      setLanguage: (language) => set({ language }),
      setFontSans: (fontSans) => set({ fontSans }),
      setFontMono: (fontMono) => set({ fontMono }),
      setFontSize: (fontSize) => set({ fontSize }),
      setDensity: (density) => set({ density }),
      setReduceMotion: (reduceMotion) => set({ reduceMotion }),
      setHour12: (hour12) => set({ hour12 }),
      setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
      setDashboardRange: (dashboardRange) => set({ dashboardRange }),
      setDashboardRefresh: (dashboardRefresh) => set({ dashboardRefresh }),
      toggleRow: (id) =>
        set((state) => ({
          collapsedRows: state.collapsedRows.includes(id)
            ? state.collapsedRows.filter((r) => r !== id)
            : [...state.collapsedRows, id],
        })),
      resetAppearance: () => set({ ...APPEARANCE_DEFAULTS }),
    }),
    {
      name: 'settings-storage',
    }
  )
);
