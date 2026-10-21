import { useEffect } from 'react';
import { FONT_SIZE_PX, useSettingsStore } from '@/store/settings-store';
import { fontStack } from '@/lib/fonts';

/** Applies font, size, density and motion preferences to <html>. */
export function useApplyAppearance() {
  const { fontSans, fontMono, fontSize, density, reduceMotion, language } = useSettingsStore();

  useEffect(() => {
    const root = document.documentElement;
    root.style.setProperty('--app-font-sans', fontStack(fontSans));
    root.style.setProperty('--app-font-mono', fontStack(fontMono));
    root.style.setProperty('--app-font-size', `${FONT_SIZE_PX[fontSize] ?? 15}px`);
    root.dataset.density = density;
    root.classList.toggle('reduce-motion', reduceMotion);
    root.lang = language;
  }, [fontSans, fontMono, fontSize, density, reduceMotion, language]);
}

export function AppearanceController() {
  useApplyAppearance();
  return null;
}
