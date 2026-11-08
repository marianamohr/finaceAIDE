import { create } from "zustand";

interface PrivacyState {
  valuesHidden: boolean;
  toggleValuesHidden: () => void;
  setValuesHidden: (v: boolean) => void;
}

export const usePrivacyStore = create<PrivacyState>((set) => ({
  valuesHidden: true,
  toggleValuesHidden: () => set((s) => ({ valuesHidden: !s.valuesHidden })),
  setValuesHidden: (v) => set({ valuesHidden: v }),
}));
