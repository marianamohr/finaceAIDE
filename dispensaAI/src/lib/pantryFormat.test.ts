import { describe, expect, it } from "vitest";
import { formatQuantity, productDisplayName } from "./pantryFormat";
import type { PantryProduct } from "@/services/api";

function makeProduct(overrides: Partial<PantryProduct> = {}): PantryProduct {
  return {
    id: "1",
    user_id: "u1",
    name_pt: "Arroz",
    name_de: "Reis",
    category: "food",
    subcategory: "",
    default_unit: "kg",
    is_produce: false,
    track_by_unit: false,
    created_at: "",
    updated_at: "",
    quantity: 1,
    unit: "kg",
    currency: "BRL",
    status: "ok",
    ...overrides,
  };
}

describe("formatQuantity", () => {
  it("unidades contáveis inteiras não mostram casas decimais", () => {
    expect(formatQuantity(5, "un")).toBe("5 un");
    expect(formatQuantity(2, "dz")).toBe("2 dz");
    expect(formatQuantity(1, "pct")).toBe("1 pct");
    expect(formatQuantity(3, "maco")).toBe("3 maco");
  });

  it("unidades contáveis fracionárias mostram uma casa decimal", () => {
    expect(formatQuantity(1.5, "un")).toBe("1.5 un");
    expect(formatQuantity(2.25, "pct")).toBe("2.3 pct");
  });

  it("unidades de peso/volume mostram até 2 casas, sem zeros à direita", () => {
    expect(formatQuantity(2, "kg")).toBe("2 kg");
    expect(formatQuantity(1.5, "kg")).toBe("1.5 kg");
    expect(formatQuantity(1.25, "l")).toBe("1.25 l");
    expect(formatQuantity(0.1, "g")).toBe("0.1 g");
    expect(formatQuantity(1.999, "ml")).toBe("2 ml");
  });
});

describe("productDisplayName", () => {
  it("usa name_pt no idioma pt-BR", () => {
    expect(productDisplayName(makeProduct(), "pt-BR")).toBe("Arroz");
  });

  it("usa name_de no idioma de quando disponível", () => {
    expect(productDisplayName(makeProduct(), "de")).toBe("Reis");
  });

  it("cai para name_pt em alemão quando name_de está vazio", () => {
    expect(productDisplayName(makeProduct({ name_de: "" }), "de")).toBe("Arroz");
  });

  it("cai para name_pt em alemão quando name_de só tem espaços", () => {
    expect(productDisplayName(makeProduct({ name_de: "   " }), "de")).toBe("Arroz");
  });
});
