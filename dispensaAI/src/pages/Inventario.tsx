import { useMemo, useState } from "react";
import { Check, Plus, Search } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { NewProductSheet } from "@/components/NewProductSheet";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useCreatePantryProduct, usePantryProducts } from "@/hooks/usePantryProducts";
import { usePantrySeedProducts } from "@/hooks/usePantrySeedProducts";
import { useT } from "@/i18n";
import { productDisplayName } from "@/lib/pantryFormat";
import { markOnboardingSeen } from "@/lib/onboarding";
import { useSettingsStore } from "@/store/settings";
import type { PantryLanguage, PantryProductCategory, PantryProductUnit, PantrySeedProduct } from "@/services/api";

const CATEGORIES: PantryProductCategory[] = ["food", "cleaning", "hygiene", "general"];
const UNITS: PantryProductUnit[] = ["un", "kg", "g", "l", "ml", "pct", "maco", "dz"];

interface InventoryRowProps {
  seed: PantrySeedProduct;
  alreadyAdded: boolean;
  language: PantryLanguage;
  onAdd: (seed: PantrySeedProduct, quantity: number, unit: PantryProductUnit) => void;
  isPending: boolean;
}

function InventoryRow({ seed, alreadyAdded, language, onAdd, isPending }: InventoryRowProps) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  const [quantity, setQuantity] = useState("1");
  const [unit, setUnit] = useState<PantryProductUnit>(seed.default_unit);
  const name = productDisplayName(seed, language);

  if (alreadyAdded) {
    return (
      <div className="flex items-center justify-between px-4 py-3 border-b border-border/50 text-muted-foreground">
        <span>{name}</span>
        <Check className="w-4 h-4" />
      </div>
    );
  }

  if (!expanded) {
    return (
      <button
        type="button"
        onClick={() => setExpanded(true)}
        className="w-full flex items-center justify-between px-4 py-3 border-b border-border/50 text-left hover:bg-secondary/50 transition-colors"
      >
        <span>{name}</span>
        <Plus className="w-4 h-4 text-muted-foreground" />
      </button>
    );
  }

  return (
    <div className="px-4 py-3 border-b border-border/50 space-y-2">
      <p className="font-medium">{name}</p>
      <div className="flex items-center gap-2">
        <Input
          type="number"
          inputMode="decimal"
          min={0}
          value={quantity}
          onChange={(e) => setQuantity(e.target.value)}
          className="h-10 rounded-lg w-20"
          autoFocus
        />
        <Select value={unit} onValueChange={(v) => setUnit(v as PantryProductUnit)}>
          <SelectTrigger className="h-10 rounded-lg w-20"><SelectValue /></SelectTrigger>
          <SelectContent>
            {UNITS.map((u) => (
              <SelectItem key={u} value={u}>{u}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          type="button"
          size="sm"
          className="h-10 rounded-lg shrink-0"
          disabled={isPending || Number(quantity) <= 0 || Number.isNaN(Number(quantity))}
          onClick={() => onAdd(seed, Number(quantity), unit)}
        >
          {t("inventory.rowAdd")}
        </Button>
        <Button type="button" size="sm" variant="ghost" className="h-10 rounded-lg shrink-0" onClick={() => setExpanded(false)}>
          {t("common.cancel")}
        </Button>
      </div>
    </div>
  );
}

export default function Inventario() {
  const t = useT();
  const navigate = useNavigate();
  const language = useSettingsStore((s) => s.language);
  const [search, setSearch] = useState("");
  const [newProductOpen, setNewProductOpen] = useState(false);

  const { seedProducts, isLoading } = usePantrySeedProducts();
  const { products } = usePantryProducts();
  const create = useCreatePantryProduct();

  const existingNames = useMemo(
    () => new Set((products ?? []).map((p) => p.name_pt.trim().toLowerCase())),
    [products],
  );

  const grouped = useMemo(() => {
    const term = search.trim().toLowerCase();
    const filtered = (seedProducts ?? []).filter((s) => productDisplayName(s, language).toLowerCase().includes(term));
    const byCategory = new Map<PantryProductCategory, PantrySeedProduct[]>();
    for (const s of filtered) {
      const list = byCategory.get(s.category) ?? [];
      list.push(s);
      byCategory.set(s.category, list);
    }
    return byCategory;
  }, [seedProducts, search, language]);

  const handleFinish = () => {
    markOnboardingSeen();
    navigate("/");
  };

  const handleAdd = (seed: PantrySeedProduct, quantity: number, unit: PantryProductUnit) => {
    create.mutate(
      {
        name_pt: seed.name_pt,
        name_de: seed.name_de || undefined,
        category: seed.category,
        subcategory: seed.subcategory || undefined,
        unit,
        is_produce: seed.is_produce,
        initial_quantity: quantity,
        shelf_life_days: seed.shelf_life_days,
      },
      {
        onSuccess: () => toast.success(t("inventory.added", { name: productDisplayName(seed, language) })),
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  };

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader
        title={t("inventory.title")}
        right={
          <Button type="button" variant="ghost" onClick={handleFinish}>
            {t("inventory.finish")}
          </Button>
        }
      />

      <div className="px-4 py-3 space-y-2 border-b border-border shrink-0">
        <p className="text-sm text-muted-foreground">{t("inventory.subtitle")}</p>
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("pantry.search")}
            className="h-11 rounded-xl pl-9"
          />
        </div>
        <Button type="button" variant="outline" className="w-full h-11 rounded-xl gap-2" onClick={() => setNewProductOpen(true)}>
          <Plus className="w-4 h-4" />
          {t("inventory.addOther")}
        </Button>
      </div>

      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <p className="text-sm text-muted-foreground text-center py-8">{t("common.loading")}</p>
        ) : (
          CATEGORIES.filter((c) => (grouped.get(c) ?? []).length > 0).map((category) => (
            <div key={category}>
              <p className="px-4 pt-3 pb-1 text-sm font-semibold text-muted-foreground">
                {t(`pantry.category.${category}`)}
              </p>
              {(grouped.get(category) ?? []).map((seed) => (
                <InventoryRow
                  key={seed.name_pt}
                  seed={seed}
                  alreadyAdded={existingNames.has(seed.name_pt.trim().toLowerCase())}
                  language={language}
                  onAdd={handleAdd}
                  isPending={create.isPending}
                />
              ))}
            </div>
          ))
        )}
      </div>

      <NewProductSheet open={newProductOpen} onOpenChange={setNewProductOpen} />
    </div>
  );
}
