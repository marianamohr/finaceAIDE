import { Badge } from "@/components/ui/badge";
import { useT } from "@/i18n";
import { cn } from "@/lib/utils";
import type { PantryRecipe } from "@/services/api";

interface RecipeViewProps {
  recipe: PantryRecipe;
  /** A tela de detalhe já mostra o título no PageHeader, então esconde aqui. */
  showTitle?: boolean;
}

export function RecipeView({ recipe, showTitle = true }: RecipeViewProps) {
  const t = useT();

  return (
    <div className="space-y-5">
      {showTitle && <h2 className="font-heading text-lg font-semibold">{recipe.title}</h2>}

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>{t("recipes.result.servings", { n: recipe.servings })}</span>
        {recipe.prep_minutes > 0 && (
          <>
            <span>&middot;</span>
            <span>{t("recipes.result.prepMinutes", { n: recipe.prep_minutes })}</span>
          </>
        )}
      </div>

      <div className="space-y-2">
        <p className="text-sm font-semibold">{t("recipes.result.ingredients")}</p>
        <ul className="space-y-2">
          {recipe.ingredients.map((ing, i) => (
            <li key={`${ing.name}-${i}`} className="flex items-center justify-between gap-3 text-sm">
              <span className={cn("min-w-0 truncate", !ing.have && "text-muted-foreground")}>
                {ing.quantity} {ing.unit} de {ing.name}
              </span>
              <Badge
                variant={ing.have ? "secondary" : "outline"}
                className={cn("shrink-0", !ing.have && "text-destructive border-destructive/40")}
              >
                {ing.have
                  ? ing.have_quantity > 0
                    ? t("recipes.result.haveQuantity", {
                        quantity: String(ing.have_quantity),
                        unit: ing.unit,
                      })
                    : t("recipes.result.have")
                  : t("recipes.result.missing")}
              </Badge>
            </li>
          ))}
        </ul>
      </div>

      {recipe.steps.length > 0 && (
        <div className="space-y-2">
          <p className="text-sm font-semibold">{t("recipes.result.steps")}</p>
          <ol className="space-y-2 list-decimal list-inside text-sm">
            {recipe.steps.map((step, i) => (
              <li key={i}>{step}</li>
            ))}
          </ol>
        </div>
      )}

      {recipe.tips.trim() !== "" && (
        <div className="space-y-1">
          <p className="text-sm font-semibold">{t("recipes.result.tips")}</p>
          <p className="text-sm text-muted-foreground">{recipe.tips}</p>
        </div>
      )}
    </div>
  );
}
