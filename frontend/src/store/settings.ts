import { create } from "zustand";
import { persist } from "zustand/middleware";

interface SettingsState {
  showGoals: boolean;
  setShowGoals: (v: boolean) => void;
  showTools: boolean;
  setShowTools: (v: boolean) => void;
  showLimitsCategories: boolean;
  setShowLimitsCategories: (v: boolean) => void;
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      showGoals: true,
      setShowGoals: (v) => set({ showGoals: v }),
      showTools: false,
      setShowTools: (v) => set({ showTools: v }),
      showLimitsCategories: false,
      setShowLimitsCategories: (v) => set({ showLimitsCategories: v }),
    }),
    { name: "fai-s" }
  )
);
