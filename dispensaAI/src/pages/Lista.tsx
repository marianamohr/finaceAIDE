import { useState } from "react";
import { ListChecks, Plus, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  useAddPantryShoppingItem,
  usePantryShoppingList,
  useRegeneratePantryShoppingList,
  useUpdatePantryShoppingItemStatus,
} from "@/hooks/usePantryShoppingList";
import { useT } from "@/i18n";
import { formatQuantity } from "@/lib/pantryFormat";
import { cn } from "@/lib/utils";
import type { PantryShoppingItem } from "@/services/api";

function ShoppingItemRow({ item }: { item: PantryShoppingItem }) {
  const t = useT();
  const updateStatus = useUpdatePantryShoppingItemStatus();

  const handleBought = () => {
    updateStatus.mutate(
      { id: item.id, status: "bought" },
      { onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")) },
    );
  };

  const quantityLabel = item.quantity != null && item.unit ? formatQuantity(item.quantity, item.unit) : null;

  return (
    <div
      className={cn(
        "flex items-center gap-3 px-4 py-3 border-b border-border transition-opacity",
        updateStatus.isPending && "opacity-50",
      )}
    >
      <Checkbox
        checked={false}
        disabled={updateStatus.isPending}
        onCheckedChange={(checked) => checked && handleBought()}
        aria-label={t("list.bought")}
      />
      <div className="flex-1 min-w-0">
        <p className="font-medium truncate">{item.name}</p>
        <div className="flex items-center gap-2 mt-1">
          {quantityLabel && <span className="text-sm text-muted-foreground">{quantityLabel}</span>}
          {item.reason !== "" && (
            <Badge variant="secondary" className="text-[10px]">
              {t(`list.reason.${item.reason}`)}
            </Badge>
          )}
        </div>
      </div>
    </div>
  );
}

export default function Lista() {
  const t = useT();
  const [name, setName] = useState("");

  const { items, isLoading } = usePantryShoppingList("open");
  const addItem = useAddPantryShoppingItem();
  const regenerate = useRegeneratePantryShoppingList();

  const handleAdd = () => {
    const trimmed = name.trim();
    if (trimmed === "") return;
    addItem.mutate(
      { name: trimmed },
      {
        onSuccess: () => setName(""),
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  };

  const handleRefresh = () => {
    regenerate.mutate(undefined, {
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  };

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader
        title={t("list.title")}
        right={
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t("list.refresh")}
            disabled={regenerate.isPending}
            onClick={handleRefresh}
          >
            <RefreshCw className={cn("w-5 h-5", regenerate.isPending && "animate-spin")} />
          </Button>
        }
      />

      <div className="flex items-center gap-2 px-4 py-3 border-b border-border shrink-0">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleAdd();
          }}
          placeholder={t("list.addPlaceholder")}
          className="h-11 rounded-xl flex-1"
        />
        <Button
          type="button"
          size="icon"
          className="h-11 w-11 rounded-xl shrink-0"
          disabled={addItem.isPending || name.trim() === ""}
          onClick={handleAdd}
          aria-label={t("list.add")}
        >
          <Plus className="w-5 h-5" />
        </Button>
      </div>

      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <p className="text-sm text-muted-foreground text-center py-8">{t("common.loading")}</p>
        ) : !items || items.length === 0 ? (
          <div className="flex flex-col items-center justify-center px-8 py-16 text-center gap-3">
            <ListChecks className="w-10 h-10 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">{t("list.empty")}</p>
          </div>
        ) : (
          items.map((item) => <ShoppingItemRow key={item.id} item={item} />)
        )}
      </div>
    </div>
  );
}
