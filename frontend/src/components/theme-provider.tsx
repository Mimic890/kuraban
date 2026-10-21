import { createContext, useContext, useEffect, useState } from 'react';

/**
 * Palette ids. Catppuccin ships all four official flavors as separate palettes
 * (Latte is light; Frappé / Macchiato / Mocha are dark). GitHub and Kanagawa
 * pair with the light/dark mode toggle; Warm is the brand default.
 */
export type Palette =
  | 'graphite'
  | 'warm'
  | 'catppuccin-latte'
  | 'catppuccin-frappe'
  | 'catppuccin-macchiato'
  | 'catppuccin-mocha'
  | 'github'
  | 'kanagawa';

export type Mode = 'dark' | 'light';

export const ALL_PALETTES: Palette[] = [
  'graphite',
  'warm',
  'catppuccin-latte',
  'catppuccin-frappe',
  'catppuccin-macchiato',
  'catppuccin-mocha',
  'github',
  'kanagawa',
];

/** Preferred brightness for palettes that are authentically one-sided. */
export const PALETTE_PREFERRED_MODE: Partial<Record<Palette, Mode>> = {
  'catppuccin-latte': 'light',
  'catppuccin-frappe': 'dark',
  'catppuccin-macchiato': 'dark',
  'catppuccin-mocha': 'dark',
};

function isPalette(value: string | null): value is Palette {
  return value !== null && (ALL_PALETTES as string[]).includes(value);
}

interface ThemeProviderProps {
  children: React.ReactNode;
  defaultPalette?: Palette;
  defaultMode?: Mode;
  storageKeyPrefix?: string;
}

interface ThemeProviderState {
  palette: Palette;
  mode: Mode;
  setPalette: (palette: Palette) => void;
  setMode: (mode: Mode) => void;
  theme: Mode;
  setTheme: (mode: Mode) => void;
}

const initialState: ThemeProviderState = {
  palette: 'graphite',
  mode: 'dark',
  setPalette: () => null,
  setMode: () => null,
  theme: 'dark',
  setTheme: () => null,
};

const ThemeProviderContext = createContext<ThemeProviderState>(initialState);

export function ThemeProvider({
  children,
  defaultPalette = 'graphite',
  defaultMode = 'dark',
  storageKeyPrefix = 'kuraban',
  ...props
}: ThemeProviderProps) {
  const [palette, setPaletteState] = useState<Palette>(() => {
    const stored = localStorage.getItem(`${storageKeyPrefix}-palette`);
    return isPalette(stored) ? stored : defaultPalette;
  });

  const [mode, setModeState] = useState<Mode>(() => {
    const stored = localStorage.getItem(`${storageKeyPrefix}-mode`) as Mode | null;
    if (stored === 'dark' || stored === 'light') return stored;
    return defaultMode;
  });

  useEffect(() => {
    const root = window.document.documentElement;

    // Clear current + legacy palette classes.
    root.classList.remove(
      'graphite',
      'warm',
      'catppuccin-latte',
      'catppuccin-frappe',
      'catppuccin-macchiato',
      'catppuccin-mocha',
      'github',
      'kanagawa',
      'sage',
      'cobalt',
      'lavender',
      'moss',
      'berry'
    );

    if (mode === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }

    root.classList.add(palette);
  }, [palette, mode]);

  const setMode = (m: Mode) => {
    // On Catppuccin, light/dark maps across flavors so the toggle still works:
    // light → Latte, dark → Mocha (pick Frappé / Macchiato from the list).
    localStorage.setItem(`${storageKeyPrefix}-mode`, m);
    setModeState(m);
    if (palette.startsWith('catppuccin-')) {
      const next: Palette = m === 'light' ? 'catppuccin-latte' : 'catppuccin-mocha';
      localStorage.setItem(`${storageKeyPrefix}-palette`, next);
      setPaletteState(next);
    }
  };

  const setPalette = (p: Palette) => {
    localStorage.setItem(`${storageKeyPrefix}-palette`, p);
    setPaletteState(p);
    // Catppuccin flavors are fixed brightness — keep mode in sync so charts
    // and any .dark-dependent UI stay correct.
    const preferred = PALETTE_PREFERRED_MODE[p];
    if (preferred) {
      localStorage.setItem(`${storageKeyPrefix}-mode`, preferred);
      setModeState(preferred);
    }
  };

  const value: ThemeProviderState = {
    palette,
    mode,
    setPalette,
    setMode,
    theme: mode,
    setTheme: setMode,
  };

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  );
}

export const useTheme = () => {
  const context = useContext(ThemeProviderContext);

  if (context === undefined) throw new Error('useTheme must be used within a ThemeProvider');

  return context;
};
