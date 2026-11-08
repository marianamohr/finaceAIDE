import { useCallback } from "react";
import { useSettingsStore } from "@/store/settings";
import { ptBR, type Key } from "./pt-BR";
import { de } from "./de";
import type { PantryLanguage } from "@/services/api";

export type { Key };

const dictionaries: Record<PantryLanguage, Record<Key, string>> = {
  "pt-BR": ptBR,
  de,
};

export const LANGUAGES: { value: PantryLanguage; label: string }[] = [
  { value: "pt-BR", label: "Português (Brasil)" },
  { value: "de", label: "Deutsch" },
];

/** Traduz `key` no idioma dado, substituindo `{nome}` pelos valores de `vars`. Cai para pt-BR se faltar. */
export function translate(
  lang: PantryLanguage,
  key: Key,
  vars?: Record<string, string | number>,
): string {
  const dict = dictionaries[lang] ?? ptBR;
  let text = dict[key] ?? ptBR[key] ?? key;
  if (vars) {
    for (const [name, value] of Object.entries(vars)) {
      text = text.split(`{${name}}`).join(String(value));
    }
  }
  return text;
}

/** Hook: devolve `t(key, vars)` já ligado ao idioma escolhido nas configurações. */
export function useT() {
  const lang = useSettingsStore((s) => s.language);
  return useCallback(
    (key: Key, vars?: Record<string, string | number>) => translate(lang, key, vars),
    [lang],
  );
}

/** Locale para Intl (números, datas) a partir do idioma da interface. */
export function intlLocale(lang: PantryLanguage): string {
  return lang === "de" ? "de-DE" : "pt-BR";
}
