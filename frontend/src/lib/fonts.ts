import '@fontsource-variable/jetbrains-mono';
import '@fontsource-variable/jetbrains-mono/wght-italic.css';
import '@fontsource-variable/fira-code';
import '@fontsource-variable/inter';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/500.css';
import '@fontsource/ibm-plex-mono/600.css';
import '@fontsource/ibm-plex-sans/400.css';
import '@fontsource/ibm-plex-sans/500.css';
import '@fontsource/ibm-plex-sans/600.css';

export interface FontOption {
  id: string;
  label: string;
  stack: string;
  monospace: boolean;
}

const MONO_FALLBACK = "ui-monospace, 'SF Mono', Menlo, Consolas, monospace";
const SANS_FALLBACK = "-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif";

export const FONTS: FontOption[] = [
  { id: 'jetbrains-mono', label: 'JetBrains Mono', stack: `'JetBrains Mono Variable', 'JetBrains Mono', ${MONO_FALLBACK}`, monospace: true },
  { id: 'fira-code', label: 'Fira Code', stack: `'Fira Code Variable', 'Fira Code', ${MONO_FALLBACK}`, monospace: true },
  { id: 'ibm-plex-mono', label: 'IBM Plex Mono', stack: `'IBM Plex Mono', ${MONO_FALLBACK}`, monospace: true },
  { id: 'geist-mono', label: 'Geist Mono', stack: `'Geist Mono', ${MONO_FALLBACK}`, monospace: true },
  { id: 'system-mono', label: 'System monospace', stack: MONO_FALLBACK, monospace: true },
  { id: 'inter', label: 'Inter', stack: `'Inter Variable', 'Inter', ${SANS_FALLBACK}`, monospace: false },
  { id: 'geist-sans', label: 'Geist Sans', stack: `'Geist Sans', ${SANS_FALLBACK}`, monospace: false },
  { id: 'ibm-plex-sans', label: 'IBM Plex Sans', stack: `'IBM Plex Sans', ${SANS_FALLBACK}`, monospace: false },
  { id: 'system', label: 'System UI', stack: SANS_FALLBACK, monospace: false },
];

export function fontStack(id: string, fallbackId = 'jetbrains-mono'): string {
  return (FONTS.find((f) => f.id === id) ?? FONTS.find((f) => f.id === fallbackId)!).stack;
}
