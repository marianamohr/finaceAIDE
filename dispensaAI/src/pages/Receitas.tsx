import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ChefHat } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { RecipeView } from "@/components/RecipeView";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { usePantryRecipes, useGeneratePantryRecipe, useSavePantryRecipe } from "@/hooks/usePantryRecipes";
import { useT, type Key } from "@/i18n";
import { cn } from "@/lib/utils";
import type { GenerateRecipeResult, PantryRecipe } from "@/services/api";

const FILTERS = [
  "beef",
  "chicken",
  "fish",
  "pork",
  "vegetarian",
  "vegan",
  "pasta",
  "rice",
  "soup",
  "salad",
  "breakfast",
  "dessert",
  "quick",
  "cheap",
  "use_expiring",
  "use_running_out",
] as const;

const RESTRICTIONS = ["gluten_free", "lactose_free", "nut_free", "egg_free", "low_carb", "no_sugar"] as const;

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

function toggle<T>(list: T[], value: T): T[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

function GenerateTab() {
  const t = useT();
  const navigate = useNavigate();

  const [filters, setFilters] = useState<string[]>([]);
  const [restrictions, setRestrictions] = useState<string[]>([]);
  const [servings, setServings] = useState("2");
  const [freeText, setFreeText] = useState("");
  const [result, setResult] = useState<GenerateRecipeResult | null>(null);

  const generate = useGeneratePantryRecipe();
  const save = useSavePantryRecipe();

  function handleGenerate() {
    const servingsNum = Number(servings);
    generate.mutate(
      {
        filters: filters.length > 0 ? filters : undefined,
        restrictions: restrictions.length > 0 ? restrictions : undefined,
        servings: Number.isFinite(servingsNum) && servingsNum > 0 ? servingsNum : undefined,
        free_text: freeText.trim() || undefined,
      },
      {
        onSuccess: (data) => setResult(data),
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  }

  function handleSave(recipe: PantryRecipe) {
    save.mutate(recipe, {
      onSuccess: (saved) => {
        toast.success(t("recipes.saved"));
        navigate(`/receitas/${saved.id}`);
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  }

  return (
    <div className="space-y-5 pb-4">
      <div className="space-y-2">
        <p className="text-sm font-semibold">{t("recipes.filters.title")}</p>
        <div className="flex flex-wrap gap-2">
          {FILTERS.map((f) => (
            <Chip key={f} active={filters.includes(f)} onClick={() => setFilters((prev) => toggle(prev, f))}>
              {t(`recipes.filter.${f}` as Key)}
            </Chip>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <p className="text-sm font-semibold">{t("recipes.restrictions.title")}</p>
        <div className="flex flex-wrap gap-2">
          {RESTRICTIONS.map((r) => (
            <Chip
              key={r}
              active={restrictions.includes(r)}
              onClick={() => setRestrictions((prev) => toggle(prev, r))}
            >
              {t(`recipes.restriction.${r}` as Key)}
            </Chip>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="servings">{t("recipes.servings")}</Label>
        <Input
          id="servings"
          type="number"
          inputMode="numeric"
          min={1}
          value={servings}
          onChange={(e) => setServings(e.target.value)}
          className="h-11 rounded-xl w-24"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="freeText">{t("recipes.freeText.label")}</Label>
        <Textarea
          id="freeText"
          value={freeText}
          onChange={(e) => setFreeText(e.target.value)}
          placeholder={t("recipes.freeText.placeholder")}
          className="rounded-xl"
          rows={2}
        />
      </div>

      <Button
        type="button"
        className="w-full h-12 rounded-xl font-heading font-semibold"
        disabled={generate.isPending}
        onClick={handleGenerate}
      >
        {generate.isPending ? t("recipes.generating") : t("recipes.generate")}
      </Button>
      {generate.isPending && <p className="text-xs text-muted-foreground text-center">{t("recipes.generateHint")}</p>}

      {result && (
        <div className="border-t border-border pt-4 space-y-4">
          <RecipeView recipe={result.recipe} />

          <div className="space-y-2">
            <Button
              type="button"
              className="w-full h-12 rounded-xl font-heading font-semibold"
              disabled={save.isPending}
              onClick={() => handleSave(result.recipe)}
            >
              {save.isPending ? t("common.saving") : t("recipes.save")}
            </Button>

            {/*
              Decisão de UX: "Adicionar faltantes" precisa de um recipe_id que só
              existe depois de salvar — e Salvar já navega para a tela de
              detalhe (onde esse botão funciona de verdade). Então aqui ele
              fica desabilitado com uma dica, em vez de salvar automaticamente
              nos bastidores e duplicar o fluxo de navegação.
            */}
            <Button type="button" variant="outline" className="w-full h-11 rounded-xl" disabled>
              {t("recipes.addMissing")}
            </Button>
            <p className="text-xs text-muted-foreground text-center">{t("recipes.addMissingDisabledHint")}</p>
          </div>
        </div>
      )}
    </div>
  );
}

function FavoritesTab() {
  const t = useT();
  const navigate = useNavigate();
  const { recipes, isLoading } = usePantryRecipes(true);

  if (isLoading) {
    return <p className="text-sm text-muted-foreground text-center py-8">{t("common.loading")}</p>;
  }

  if (!recipes || recipes.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center px-8 py-16 text-center gap-3">
        <ChefHat className="w-10 h-10 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">{t("recipes.empty")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {recipes.map((recipe) => {
        const missing = recipe.ingredients.filter((i) => !i.have).length;
        return (
          <button
            key={recipe.id}
            type="button"
            onClick={() => navigate(`/receitas/${recipe.id}`)}
            className="w-full text-left border border-border rounded-xl p-4 space-y-1 hover:bg-secondary/50 transition-colors"
          >
            <p className="font-heading font-semibold">{recipe.title}</p>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span>{t("recipes.result.servings", { n: recipe.servings })}</span>
              {recipe.prep_minutes > 0 && (
                <>
                  <span>&middot;</span>
                  <span>{t("recipes.result.prepMinutes", { n: recipe.prep_minutes })}</span>
                </>
              )}
              {missing > 0 && (
                <>
                  <span>&middot;</span>
                  <span>{t("recipes.card.missing", { count: missing })}</span>
                </>
              )}
            </div>
          </button>
        );
      })}
    </div>
  );
}

export default function Receitas() {
  const t = useT();

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader title={t("recipes.title")} />

      <Tabs defaultValue="generate" className="flex-1 min-h-0 flex flex-col">
        <div className="px-4 pt-3 shrink-0">
          <TabsList className="w-full">
            <TabsTrigger value="generate" className="flex-1">
              {t("recipes.tab.generate")}
            </TabsTrigger>
            <TabsTrigger value="favorites" className="flex-1">
              {t("recipes.tab.favorites")}
            </TabsTrigger>
          </TabsList>
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-4">
          <TabsContent value="generate" className="mt-3">
            <GenerateTab />
          </TabsContent>
          <TabsContent value="favorites" className="mt-3">
            <FavoritesTab />
          </TabsContent>
        </div>
      </Tabs>
    </div>
  );
}
