import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Currency = "BRL" | "EUR" | "USD" | "GBP";

interface CurrencyState {
  currency: Currency;
  setCurrency: (c: Currency) => void;
}

export const useCurrencyStore = create<CurrencyState>()(
  persist(
    (set) => ({
      currency: "BRL",
      setCurrency: (c) => set({ currency: c }),
    }),
    { name: "fai-c" }
  )
);

const SYMBOLS: Record<Currency, string> = { BRL: "R$", EUR: "€", USD: "$", GBP: "£" };
const LOCALES: Record<Currency, string> = { BRL: "pt-BR", EUR: "de-DE", USD: "en-US", GBP: "en-GB" };

export function formatCurrency(value: number, currency: Currency): string {
  return Math.abs(value).toLocaleString(LOCALES[currency], {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
  });
}

export function currencySymbol(currency: Currency): string {
  return SYMBOLS[currency];
}

export function maskCurrency(value: number, currency: Currency): string {
  const symbol = SYMBOLS[currency];
  const digits = Math.max(4, Math.floor(Math.abs(value)).toString().length);
  return `${symbol} ${"•".repeat(digits)}`;
}
