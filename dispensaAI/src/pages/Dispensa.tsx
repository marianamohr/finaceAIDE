import { useEffect, useState } from "react";
import { Plus, Refrigerator, Search } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { NewProductSheet } from "@/components/NewProductSheet";
import { PageHeader } from "@/components/PageHeader";
import { ProductRow } from "@/components/ProductRow";
import { QuantitySheet } from "@/components/QuantitySheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useConsumePantryProductFromText, usePantryProducts } from "@/hooks/usePantryProducts";
import { useT } from "@/i18n";
import { cn } from "@/lib/utils";
import { productDisplayName } from "@/lib/pantryFormat";
import { hasSeenOnboarding, markOnboardingSeen } from "@/lib/onboarding";
import { useSettingsStore } from "@/store/settings";
import type { PantryProduct, PantryProductCategory, PantryProductStatus } from "@/services/api";

const CATEGORIES: PantryProductCategory[] = ["food", "cleaning", "hygiene", "general"];
const STATUSES: PantryProductStatus[] = ["low", "out", "running_out"];

function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "shrink-0 px-3 py-1.5 rounded-full text-sm border transition-colors",
        active
          ? "bg-primary text-primary-foreground border-transparent"
          : "bg-background text-foreground border-border",
      )}
    >
      {children}
    </button>
  );
}

export default function Dispensa() {
  const t = useT();
  const navigate = useNavigate();
  const language = useSettingsStore((s) => s.language);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [category, setCategory] = useState<PantryProductCategory | undefined>(undefined);
  const [status, setStatus] = useState<PantryProductStatus | "expiring" | undefined>(undefined);
  const [newProductOpen, setNewProductOpen] = useState(false);
  const [selectedProduct, setSelectedProduct] = useState<PantryProduct | null>(null);
  const [consumeText, setConsumeText] = useState("");

  const consumeFromText = useConsumePantryProductFromText();

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const hasFilter = debouncedSearch !== "" || category !== undefined || status !== undefined;

  const { products, isLoading } = usePantryProducts({
    q: debouncedSearch || undefined,
    category,
    status,
  });

  // Primeiro acesso com dispensa vazia (sem nenhum filtro aplicado, senão uma
  // busca sem resultado dispararia isso à toa): manda direto pro inventário
  // inicial, uma única vez — marca o flag antes de navegar para nunca mais
  // redirecionar sozinho, mesmo que a usuária volte sem terminar.
  useEffect(() => {
    if (!isLoading && !hasFilter && products?.length === 0 && !hasSeenOnboarding()) {
      markOnboardingSeen();
      navigate("/inventario");
    }
  }, [isLoading, hasFilter, products, navigate]);

  const handleConsumeText = () => {
    const text = consumeText.trim();
    if (!text) return;
    consumeFromText.mutate(text, {
      onSuccess: (result) => {
        setConsumeText("");
        if (result.matched.length > 0) {
          const items = result.matched.map((m) => productDisplayName(m.product, language)).join(", ");
          toast.success(t("pantry.consumeText.matched", { items }));
        }
        if (result.unmatched.length > 0) {
          if (result.matched.length === 0) {
            toast.error(t("pantry.consumeText.allUnmatched"));
          } else {
            toast.warning(t("pantry.consumeText.unmatched", { items: result.unmatched.join(", ") }));
          }
        }
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  };

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader
        title={t("pantry.title")}
        right={
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t("pantry.addProduct")}
            onClick={() => setNewProductOpen(true)}
          >
            <Plus className="w-5 h-5" />
          </Button>
        }
      />

      <div className="px-4 py-3 space-y-3 border-b border-border shrink-0">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("pantry.search")}
            className="h-11 rounded-xl pl-9"
          />
        </div>

        <div className="flex gap-2 overflow-x-auto pb-1">
          <Chip active={category === undefined} onClick={() => setCategory(undefined)}>
            {t("pantry.filter.all")}
          </Chip>
          {CATEGORIES.map((c) => (
            <Chip key={c} active={category === c} onClick={() => setCategory(c)}>
              {t(`pantry.category.${c}`)}
            </Chip>
          ))}
        </div>

        <div className="flex gap-2 overflow-x-auto pb-1">
          <Chip active={status === undefined} onClick={() => setStatus(undefined)}>
            {t("pantry.filter.allStatus")}
          </Chip>
          {STATUSES.map((s) => (
            <Chip key={s} active={status === s} onClick={() => setStatus(s)}>
              {t(`pantry.status.${s}`)}
            </Chip>
          ))}
          <Chip active={status === "expiring"} onClick={() => setStatus("expiring")}>
            {t("pantry.filter.expiring")}
          </Chip>
        </div>

        <div className="flex items-center gap-2">
          <Input
            value={consumeText}
            onChange={(e) => setConsumeText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleConsumeText()}
            placeholder={t("pantry.consumeText.placeholder")}
            className="h-11 rounded-xl"
            aria-label={t("pantry.consumeText.label")}
          />
          <Button
            type="button"
            variant="outline"
            className="h-11 rounded-xl shrink-0"
            disabled={consumeFromText.isPending || !consumeText.trim()}
            onClick={handleConsumeText}
          >
            {consumeFromText.isPending ? t("pantry.consumeText.sending") : t("pantry.consumeText.submit")}
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <p className="text-sm text-muted-foreground text-center py-8">{t("common.loading")}</p>
        ) : !products || products.length === 0 ? (
          <div className="flex flex-col items-center justify-center px-8 py-16 text-center gap-3">
            <Refrigerator className="w-10 h-10 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              {hasFilter ? t("pantry.emptyFiltered") : t("pantry.empty")}
            </p>
          </div>
        ) : (
          products.map((product) => (
            <ProductRow key={product.id} product={product} onOpen={setSelectedProduct} />
          ))
        )}
      </div>

      <NewProductSheet open={newProductOpen} onOpenChange={setNewProductOpen} />
      <QuantitySheet
        open={selectedProduct !== null}
        onOpenChange={(open) => !open && setSelectedProduct(null)}
        product={selectedProduct}
      />
    </div>
  );
}
