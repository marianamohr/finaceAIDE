import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useConsumePantryProduct, useMarkPantryProductOut } from "@/hooks/usePantryProducts";
import { useSettingsStore } from "@/store/settings";
import { useT } from "@/i18n";
import { formatQuantity, productDisplayName } from "@/lib/pantryFormat";
import type { PantryProduct, PantryProductStatus } from "@/services/api";

const STATUS_BADGE_CLASS: Record<PantryProductStatus, string> = {
  ok: "border-transparent bg-success text-success-foreground",
  low: "border-transparent bg-warning text-warning-foreground",
  out: "border-transparent bg-destructive text-destructive-foreground",
  running_out: "border-transparent bg-warning text-warning-foreground",
};

const EXPIRING_BADGE_CLASS = "border-transparent bg-warning text-warning-foreground";

interface ProductRowProps {
  product: PantryProduct;
  onOpen: (product: PantryProduct) => void;
}

export function ProductRow({ product, onOpen }: ProductRowProps) {
  const t = useT();
  const language = useSettingsStore((s) => s.language);
  const consume = useConsumePantryProduct();
  const markOut = useMarkPantryProductOut();

  const handleConsume = (e: React.MouseEvent) => {
    e.stopPropagation();
    consume.mutate(
      { id: product.id, quantity: 1 },
      { onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")) },
    );
  };

  const handleMarkOut = (e: React.MouseEvent) => {
    e.stopPropagation();
    markOut.mutate(product.id, {
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  };

  return (
    <button
      type="button"
      onClick={() => onOpen(product)}
      className="w-full flex items-center gap-3 px-4 py-3 border-b border-border text-left hover:bg-secondary/50 transition-colors"
    >
      <div className="flex-1 min-w-0">
        <p className="font-medium truncate">{productDisplayName(product, language)}</p>
        <div className="flex items-center gap-2 mt-1">
          <span className="text-sm text-muted-foreground">{formatQuantity(product.quantity, product.unit)}</span>
          <Badge className={STATUS_BADGE_CLASS[product.status]}>{t(`pantry.status.${product.status}`)}</Badge>
          {product.expiring && (
            <Badge className={EXPIRING_BADGE_CLASS}>{t("pantry.status.expiring")}</Badge>
          )}
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={consume.isPending || product.quantity <= 0}
          onClick={handleConsume}
        >
          {t("pantry.action.consumeOne")}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={markOut.isPending || product.status === "out"}
          onClick={handleMarkOut}
        >
          {t("pantry.action.markOut")}
        </Button>
      </div>
    </button>
  );
}
