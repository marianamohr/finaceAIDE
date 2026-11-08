import type { PantryLanguage } from "@/services/api";

// Unidades "contáveis": exibe inteiro sem casas decimais quando for inteiro,
// senão com uma casa decimal.
const COUNTABLE_UNITS = new Set(["un", "dz", "pct", "maco"]);

/** Remove zeros à direita de um número já fixado em N casas decimais. */
function trimTrailingZeros(value: string): string {
  if (!value.includes(".")) return value;
  return value.replace(/0+$/, "").replace(/\.$/, "");
}

/** Formata `quantity` com a unidade, seguindo a convenção de cada tipo de unidade. */
export function formatQuantity(quantity: number, unit: string): string {
  let formatted: string;
  if (COUNTABLE_UNITS.has(unit)) {
    formatted = Number.isInteger(quantity) ? String(quantity) : quantity.toFixed(1);
  } else {
    formatted = trimTrailingZeros(quantity.toFixed(2));
  }
  return `${formatted} ${unit}`;
}

/** Nome do produto no idioma da interface, com fallback para pt-BR quando faltar name_de. */
export function productDisplayName(
  product: { name_pt: string; name_de: string },
  language: PantryLanguage,
): string {
  if (language === "de" && product.name_de.trim() !== "") return product.name_de;
  return product.name_pt;
}
