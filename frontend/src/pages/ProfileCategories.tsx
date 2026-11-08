import { useEffect, useState } from "react";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { useSettings } from "@/hooks/useSettings";
import { cn } from "@/lib/utils";
import { api } from "@/services/api";

interface Category {
  slug: string;
  label: string;
  type: string;
  mode: string;
}

export default function ProfileCategories() {
  const { categoryContext, loading, updateContext } = useSettings();
  const [allCategories, setAllCategories] = useState<Category[]>([]);

  useEffect(() => {
    api.get<Category[]>("/categories").then(setAllCategories).catch(() => {});
  }, []);

  const categories = allCategories.filter(c => {
    if (c.type !== "expense") return false;
    if (categoryContext === "food_detail") return true;
    return c.mode === "standard";
  });

  const toggleContext = () => {
    updateContext(categoryContext === "standard" ? "food_detail" : "standard");
  };

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Categorias ativas" showBack />

      <div className="flex-1 overflow-y-auto px-4 pb-4 space-y-4 pt-4">
        {/* Mode toggle */}
        <div className="rounded-xl bg-card p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Modo alimentação detalhado</p>
              <p className="text-xs text-muted-foreground">
                {categoryContext === "food_detail"
                  ? "Categorias separadas por tipo de alimento"
                  : "Alimentação como categoria única"}
              </p>
            </div>
            <button
              onClick={toggleContext}
              disabled={loading}
              className={cn(
                "relative w-11 h-6 rounded-full transition-colors shrink-0",
                categoryContext === "food_detail" ? "bg-primary" : "bg-muted"
              )}
            >
              <span
                className={cn(
                  "absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform",
                  categoryContext === "food_detail" ? "translate-x-5" : "translate-x-0"
                )}
              />
            </button>
          </div>
        </div>

        {/* Category grid */}
        <div className="rounded-xl bg-card p-4 space-y-2">
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
            {categories.length} categorias ativas
          </p>
          <div className="grid grid-cols-2 gap-1.5">
            {categories.map((c) => (
              <span key={c.slug} className="text-xs bg-secondary rounded-lg px-2 py-1 text-center">{c.label}</span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
