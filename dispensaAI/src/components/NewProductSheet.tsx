import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useCreatePantryProduct } from "@/hooks/usePantryProducts";
import { useT } from "@/i18n";
import type { PantryProductCategory, PantryProductUnit } from "@/services/api";

const CATEGORIES: PantryProductCategory[] = ["food", "cleaning", "hygiene", "general"];
const UNITS: PantryProductUnit[] = ["un", "kg", "g", "l", "ml", "pct", "maco", "dz"];

interface NewProductSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const initialForm = {
  namePt: "",
  nameDe: "",
  category: "food" as PantryProductCategory,
  unit: "un" as PantryProductUnit,
  initialQuantity: "",
  minLevel: "",
  shelfLifeDays: "",
  isProduce: false,
};

export function NewProductSheet({ open, onOpenChange }: NewProductSheetProps) {
  const t = useT();
  const [form, setForm] = useState(initialForm);
  const create = useCreatePantryProduct();

  const handleAdd = () => {
    if (!form.namePt.trim()) return;
    const initialQuantity = form.initialQuantity.trim() === "" ? undefined : Number(form.initialQuantity);
    const minLevel = form.minLevel.trim() === "" ? undefined : Number(form.minLevel);
    const shelfLifeDays = form.shelfLifeDays.trim() === "" ? undefined : Number(form.shelfLifeDays);

    create.mutate(
      {
        name_pt: form.namePt.trim(),
        name_de: form.nameDe.trim() || undefined,
        category: form.category,
        unit: form.unit,
        initial_quantity: initialQuantity,
        min_level: minLevel,
        shelf_life_days: shelfLifeDays,
        is_produce: form.isProduce,
      },
      {
        onSuccess: () => {
          toast.success(t("pantry.form.added"));
          setForm(initialForm);
          onOpenChange(false);
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" className="rounded-t-2xl max-h-[90vh] overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{t("pantry.addProduct")}</SheetTitle>
        </SheetHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="namePt">{t("pantry.form.namePt")}</Label>
            <Input
              id="namePt"
              value={form.namePt}
              onChange={(e) => setForm({ ...form, namePt: e.target.value })}
              className="h-11 rounded-xl"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="nameDe">{t("pantry.form.nameDe")}</Label>
            <Input
              id="nameDe"
              value={form.nameDe}
              onChange={(e) => setForm({ ...form, nameDe: e.target.value })}
              className="h-11 rounded-xl"
            />
          </div>

          <div className="space-y-2">
            <Label>{t("pantry.form.category")}</Label>
            <Select
              value={form.category}
              onValueChange={(v) => setForm({ ...form, category: v as PantryProductCategory })}
            >
              <SelectTrigger className="h-11 rounded-xl"><SelectValue /></SelectTrigger>
              <SelectContent>
                {CATEGORIES.map((c) => (
                  <SelectItem key={c} value={c}>{t(`pantry.category.${c}`)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>{t("pantry.form.unit")}</Label>
            <Select value={form.unit} onValueChange={(v) => setForm({ ...form, unit: v as PantryProductUnit })}>
              <SelectTrigger className="h-11 rounded-xl"><SelectValue /></SelectTrigger>
              <SelectContent>
                {UNITS.map((u) => (
                  <SelectItem key={u} value={u}>{u}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="initialQuantity">{t("pantry.form.initialQuantity")}</Label>
            <Input
              id="initialQuantity"
              type="number"
              inputMode="decimal"
              min={0}
              value={form.initialQuantity}
              onChange={(e) => setForm({ ...form, initialQuantity: e.target.value })}
              className="h-11 rounded-xl"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="minLevel">{t("pantry.form.minLevel")}</Label>
            <Input
              id="minLevel"
              type="number"
              inputMode="decimal"
              min={0}
              value={form.minLevel}
              onChange={(e) => setForm({ ...form, minLevel: e.target.value })}
              className="h-11 rounded-xl"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="shelfLifeDays">{t("pantry.form.shelfLifeDays")}</Label>
            <Input
              id="shelfLifeDays"
              type="number"
              inputMode="numeric"
              min={0}
              value={form.shelfLifeDays}
              onChange={(e) => setForm({ ...form, shelfLifeDays: e.target.value })}
              className="h-11 rounded-xl"
            />
          </div>

          <div className="flex items-center gap-2">
            <Checkbox
              id="isProduce"
              checked={form.isProduce}
              onCheckedChange={(v) => setForm({ ...form, isProduce: v === true })}
            />
            <Label htmlFor="isProduce" className="font-normal">{t("pantry.form.isProduce")}</Label>
          </div>

          <Button
            type="button"
            onClick={handleAdd}
            disabled={create.isPending || !form.namePt.trim()}
            className="w-full h-12 rounded-xl font-heading font-semibold"
          >
            {t("pantry.form.add")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
