import { describe, expect, it } from "vitest";
import { intlLocale, translate } from "./index";
import { ptBR, type Key } from "./pt-BR";
import { de } from "./de";

describe("i18n", () => {
  it("todas as chaves de pt-BR existem em de", () => {
    const missing = (Object.keys(ptBR) as Key[]).filter((k) => !(k in de) || de[k].trim() === "");
    expect(missing).toEqual([]);
  });

  it("traduz no idioma pedido", () => {
    expect(translate("pt-BR", "nav.pantry")).toBe("Dispensa");
    expect(translate("de", "nav.pantry")).toBe("Vorrat");
  });

  it("substitui variáveis", () => {
    expect(translate("pt-BR", "settings.interval.days", { n: 7 })).toBe("dias");
  });

  it("locale do Intl segue o idioma", () => {
    expect(intlLocale("pt-BR")).toBe("pt-BR");
    expect(intlLocale("de")).toBe("de-DE");
  });
});
