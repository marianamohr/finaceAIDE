import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useT } from "@/i18n";
import type { PantryLanguage, PantryProductCategory, PantryProductUnit, PantryReceiptItem } from "@/services/api";

const CATEGORIES: PantryProductCategory[] = ["food", "cleaning", "hygiene", "general"];
const UNITS: PantryProductUnit[] = ["un", "kg", "g", "l", "ml", "pct", "maco", "dz"];

interface ReceiptItemRowProps {
  item: PantryReceiptItem;
  language: PantryLanguage;
  onChange: (patch: Partial<PantryReceiptItem>) => void;
}

/** Nome do item no idioma da interface, com fallback para pt-BR (ou o texto cru da nota). */
export function receiptItemDisplayName(item: PantryReceiptItem, language: PantryLanguage): string {
  if (language === "de" && item.name_de.trim() !== "") return item.name_de;
  if (item.name_pt.trim() !== "") return item.name_pt;
  return item.raw_name;
}

export function ReceiptItemRow({ item, language, onChange }: ReceiptItemRowProps) {
  const t = useT();
  const name = language === "de" ? item.name_de : item.name_pt;

  return (
    <div className="px-4 py-3 border-b border-border space-y-2">
      <div className="flex items-center gap-2">
        <Input
          value={name}
          onChange={(e) =>
            onChange(language === "de" ? { name_de: e.target.value } : { name_pt: e.target.value })
          }
          className="h-10 rounded-lg flex-1"
        />
        {!item.product_id && (
          <Badge variant="secondary" className="shrink-0">
            {t("receipt.review.newProduct")}
          </Badge>
        )}
      </div>

      <div className="flex items-center gap-2">
        <Select
          value={item.category || undefined}
          onValueChange={(v) => onChange({ category: v as PantryProductCategory })}
        >
          <SelectTrigger className="h-10 rounded-lg flex-1">
            <SelectValue placeholder={t("receipt.review.category")} />
          </SelectTrigger>
          <SelectContent>
            {CATEGORIES.map((c) => (
              <SelectItem key={c} value={c}>
                {t(`pantry.category.${c}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Input
          type="number"
          inputMode="decimal"
          min={0}
          value={item.quantity}
          onChange={(e) => onChange({ quantity: Number(e.target.value) })}
          className="h-10 rounded-lg w-20 shrink-0"
        />

        <Select value={item.unit} onValueChange={(v) => onChange({ unit: v as PantryProductUnit })}>
          <SelectTrigger className="h-10 rounded-lg w-20 shrink-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {UNITS.map((u) => (
              <SelectItem key={u} value={u}>
                {u}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}
