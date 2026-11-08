import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { PantryCountry, PantryCurrency, PantryLanguage, PantrySettings } from "@/services/api";

interface SettingsState {
  language: PantryLanguage;
  country: PantryCountry;
  currency: PantryCurrency;
  shoppingIntervalDays: number;
  askProduceUnits: boolean;
  setLanguage: (language: PantryLanguage) => void;
  setFromServer: (s: PantrySettings) => void;
  reset: () => void;
}

const defaults = {
  language: "pt-BR" as PantryLanguage,
  country: "BR" as PantryCountry,
  currency: "BRL" as PantryCurrency,
  shoppingIntervalDays: 7,
  askProduceUnits: true,
};

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      ...defaults,
      setLanguage: (language) => set({ language }),
      setFromServer: (s) =>
        set({
          language: s.language,
          country: s.country,
          currency: s.currency,
          shoppingIntervalDays: s.shopping_interval_days,
          askProduceUnits: s.ask_produce_units,
        }),
      reset: () => set({ ...defaults }),
    }),
    { name: "dai-s" },
  ),
);
