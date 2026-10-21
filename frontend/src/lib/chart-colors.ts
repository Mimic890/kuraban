export function hexToHsl(hex: string): { h: number; s: number; l: number } {
  let c = hex.replace('#', '').trim();
  if (c.length === 3) c = c.split('').map((x) => x + x).join('');
  const num = parseInt(c, 16);
  if (isNaN(num)) return { h: 210, s: 60, l: 50 };

  const r = ((num >> 16) & 255) / 255;
  const g = ((num >> 8) & 255) / 255;
  const b = (num & 255) / 255;

  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  let h = 0;
  let s = 0;
  const l = (max + min) / 2;

  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch (max) {
      case r:
        h = (g - b) / d + (g < b ? 6 : 0);
        break;
      case g:
        h = (b - r) / d + 2;
        break;
      case b:
        h = (r - g) / d + 4;
        break;
    }
    h /= 6;
  }

  return { h: Math.round(h * 360), s: Math.round(s * 100), l: Math.round(l * 100) };
}

/**
 * Generates `count` unique, non-repeating colors tailored to the active theme.
 * Uses uniform hue distribution across 360 degrees starting from theme primary color,
 * ensuring NO duplicate colors even if count > theme palette size.
 */
export function getUniqueThemeColors(count: number): string[] {
  if (count <= 0) return [];

  let primaryHex = '#3f68c0';
  let isDark = false;

  if (typeof document !== 'undefined') {
    const rootStyle = getComputedStyle(document.documentElement);
    const p = rootStyle.getPropertyValue('--primary').trim();
    if (p) primaryHex = p;
    isDark = document.documentElement.classList.contains('dark');
  }

  const primaryHsl = hexToHsl(primaryHex);

  if (count === 1) {
    return [primaryHex];
  }

  const result: string[] = [];
  const angleStep = 360 / count;

  for (let i = 0; i < count; i++) {
    const hue = (primaryHsl.h + i * angleStep) % 360;
    const saturation = isDark ? 65 + (i % 3) * 8 : 60 + (i % 3) * 8;
    const lightness = isDark ? 52 + ((i * 5) % 16) - 8 : 45 + ((i * 5) % 16) - 8;

    result.push(`hsl(${Math.round(hue)}, ${Math.round(saturation)}%, ${Math.round(lightness)}%)`);
  }

  return result;
}

export const getTextColor = () => {
  if (typeof document === 'undefined') return '#e8eaed';
  return getComputedStyle(document.documentElement).getPropertyValue('--popover-foreground').trim() || '#e8eaed';
};

export const getTooltipStyle = () => {
  return {
    backgroundColor: 'var(--popover)',
    border: '1px solid var(--border)',
    borderRadius: '8px',
    color: 'var(--popover-foreground)',
    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
  };
};
