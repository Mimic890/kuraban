import { useEffect, useState } from 'react';

/** Grafana's classic series order, mapped to theme tokens. */
export const SERIES_COLORS = [
  'var(--chart-green)',
  'var(--chart-olive)',
  'var(--chart-blue)',
  'var(--chart-orange)',
  'var(--chart-red)',
  'var(--chart-purple)',
  'var(--chart-cyan)',
  'var(--chart-pink)',
  'var(--chart-brown)',
  'var(--chart-gray)',
];

export function seriesColor(i: number): string {
  return SERIES_COLORS[i % SERIES_COLORS.length];
}

/** Resolves `var(--x)` (possibly nested) to a concrete color for canvas. */
export function resolveCssColor(color: string, fallback = '#888888'): string {
  if (typeof document === 'undefined') return fallback;
  const m = color.match(/^var\((--[\w-]+)\)$/);
  if (!m) return color;
  const v = getComputedStyle(document.documentElement).getPropertyValue(m[1]).trim();
  if (!v) return fallback;
  return v.startsWith('var(') ? resolveCssColor(v, fallback) : v;
}

export function withAlpha(color: string, alpha: number): string {
  const hex = color.trim();
  const m = hex.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (m) {
    let h = m[1];
    if (h.length === 3) h = h.split('').map((c) => c + c).join('');
    const n = parseInt(h, 16);
    return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
  }
  const rgb = hex.match(/^rgba?\(([^)]+)\)$/i);
  if (rgb) {
    const [r, g, b] = rgb[1].split(',').map((x) => x.trim());
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  return hex;
}

/** Bumps whenever the root theme/font changes so canvases can repaint. */
export function useThemeVersion(): number {
  const [version, setVersion] = useState(0);
  useEffect(() => {
    const observer = new MutationObserver(() => setVersion((v) => v + 1));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'style'] });
    return () => observer.disconnect();
  }, []);
  return version;
}
