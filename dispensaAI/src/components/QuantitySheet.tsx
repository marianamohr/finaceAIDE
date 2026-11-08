import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import {
  useAdjustPantryProduct,
  useChangePantryProductUnit,
  useDeletePantryProduct,
  useUpdatePantryProduct,
} from "@/hooks/usePantryProducts";
import { intlLocale, useT } from "@/i18n";
import { useSettingsStore } from "@/store/settings";
import type { PantryProduct, PantryProductUnit } from "@/services/api";

const UNITS: PantryProductUnit[] = ["un", "kg", "g", "l", "ml", "pct", "maco", "dz"];

// Espelha pantry.UnitConversionFactor no backend: só esses dois pares
// convertem exatamente. Usado só para decidir a mensagem de confirmação —
// a conversão de verdade é sempre feita no backend.
const CONVERTIBLE_UNIT_GROUPS: PantryProductUnit[][] = [
  ["kg", "g"],
  ["l", "ml"],
];

function unitsConvertExactly(from: PantryProductUnit, to: PantryProductUnit): boolean {
  if (from === to) return true;
  return CONVERTIBLE_UNIT_GROUPS.some((group) => group.includes(from) && group.includes(to));
}

interface QuantitySheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  product: PantryProduct | null;
}

export function QuantitySheet({ open, onOpenChange, product }: QuantitySheetProps) {
  const t = useT();
  const language = useSettingsStore((s) => s.language);
  const [quantity, setQuantity] = useState("");
  const [minLevel, setMinLevel] = useState("");
  const [shelfLifeDays, setShelfLifeDays] = useState("");

  const adjust = useAdjustPantryProduct();
  const update = useUpdatePantryProduct();
  const changeUnit = useChangePantryProductUnit();
  const del = useDeletePantryProduct();

  useEffect(() => {
    if (product) {
      setQuantity(String(product.quantity));
      // min_level já vem como o valor EFETIVO (manual ou sugerido pelo ritmo de
      // consumo) — só preenche o campo quando a origem é manual, senão a
      // usuária estaria "confirmando" um número que nunca digitou.
      setMinLevel(product.min_level_source === "manual" && product.min_level != null ? String(product.min_level) : "");
      setShelfLifeDays(product.shelf_life_days != null ? String(product.shelf_life_days) : "");
    }
  }, [product]);

  if (!product) return null;

  const handleSaveQuantity = () => {
    const value = Number(quantity);
    if (Number.isNaN(value) || value < 0) return;
    adjust.mutate(
      { id: product.id, quantity: value },
      {
        onSuccess: () => toast.success(t("pantry.sheet.quantityUpdated")),
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  };

  const handleSaveMinLevel = () => {
    const minLevelValue = minLevel.trim() === "" ? undefined : Number(minLevel);
    const shelfLifeValue = shelfLifeDays.trim() === "" ? undefined : Number(shelfLifeDays);
    if (minLevelValue !== undefined && (Number.isNaN(minLevelValue) || minLevelValue < 0)) return;
    if (shelfLifeValue !== undefined && (Number.isNaN(shelfLifeValue) || shelfLifeValue < 0)) return;
    update.mutate(
      {
        id: product.id,
        input: {
          name_pt: product.name_pt,
          name_de: product.name_de,
          category: product.category,
          subcategory: product.subcategory,
          unit: product.default_unit,
          is_produce: product.is_produce,
          track_by_unit: product.track_by_unit,
          min_level: minLevelValue,
          shelf_life_days: shelfLifeValue,
        },
      },
      {
        onSuccess: () => toast.success(t("pantry.sheet.minLevelUpdated")),
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  };

  const handleChangeUnit = (newUnit: PantryProductUnit) => {
    if (newUnit === product.unit) return;
    if (!unitsConvertExactly(product.unit as PantryProductUnit, newUnit)) {
      if (!window.confirm(t("pantry.sheet.confirmUnitReset", { unit: product.unit }))) return;
    }
    changeUnit.mutate(
      { id: product.id, unit: newUnit },
      {
        onSuccess: () => toast.success(t("pantry.sheet.unitChanged")),
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  };

  const handleDelete = () => {
    if (!window.confirm(t("pantry.sheet.confirmDelete"))) return;
    del.mutate(product.id, {
      onSuccess: () => {
        toast.success(t("pantry.sheet.deleted"));
        onOpenChange(false);
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" className="rounded-t-2xl">
        <SheetHeader>
          <SheetTitle>{t("pantry.sheet.title")}</SheetTitle>
        </SheetHeader>

        <div className="space-y-6 py-4">
          <div className="space-y-2">
            <Label htmlFor="quantity">{t("pantry.sheet.quantity")}</Label>
            <div className="flex items-center gap-2">
              <Input
                id="quantity"
                type="number"
                inputMode="decimal"
                min={0}
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
                className="h-11 rounded-xl"
              />
              <Button
                type="button"
                onClick={handleSaveQuantity}
                disabled={adjust.isPending}
                className="h-11 rounded-xl shrink-0"
              >
                {t("common.save")}
              </Button>
            </div>
          </div>

          <div className="space-y-2">
            <Label>{t("pantry.sheet.unit")}</Label>
            <Select
              value={product.unit}
              onValueChange={(v) => handleChangeUnit(v as PantryProductUnit)}
              disabled={changeUnit.isPending}
            >
              <SelectTrigger className="h-11 rounded-xl"><SelectValue /></SelectTrigger>
              <SelectContent>
                {UNITS.map((u) => (
                  <SelectItem key={u} value={u}>{u}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="minLevel">{t("pantry.form.minLevel")}</Label>
            <Input
              id="minLevel"
              type="number"
              inputMode="decimal"
              min={0}
              value={minLevel}
              onChange={(e) => setMinLevel(e.target.value)}
              placeholder={
                product.min_level_source === "auto" && product.min_level != null
                  ? String(product.min_level)
                  : undefined
              }
              className="h-11 rounded-xl"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="shelfLifeDays">{t("pantry.form.shelfLifeDays")}</Label>
            <div className="flex items-center gap-2">
              <Input
                id="shelfLifeDays"
                type="number"
                inputMode="numeric"
                min={0}
                value={shelfLifeDays}
                onChange={(e) => setShelfLifeDays(e.target.value)}
                className="h-11 rounded-xl"
              />
              <Button
                type="button"
                onClick={handleSaveMinLevel}
                disabled={update.isPending}
                className="h-11 rounded-xl shrink-0"
              >
                {t("common.save")}
              </Button>
            </div>
          </div>

          {(product.run_out_at || product.expires_at) && (
            <div className="text-sm text-muted-foreground space-y-1">
              {product.run_out_at && (
                <p>{t("pantry.sheet.runOutAt", { date: new Date(product.run_out_at).toLocaleDateString(intlLocale(language)) })}</p>
              )}
              {product.expires_at && (
                <p>{t("pantry.sheet.expiresAt", { date: new Date(product.expires_at).toLocaleDateString(intlLocale(language)) })}</p>
              )}
            </div>
          )}

          <Button
            type="button"
            variant="outline"
            onClick={handleDelete}
            disabled={del.isPending}
            className="w-full h-11 rounded-xl text-destructive"
          >
            {t("pantry.sheet.delete")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
