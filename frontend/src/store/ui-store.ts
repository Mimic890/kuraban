import { create } from 'zustand';

interface UIStore {
  paletteOpen: boolean;
  setPaletteOpen: (open: boolean) => void;
}

export const useUIStore = create<UIStore>()((set) => ({
  paletteOpen: false,
  setPaletteOpen: (paletteOpen) => set({ paletteOpen }),
}));
