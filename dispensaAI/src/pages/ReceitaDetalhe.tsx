import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { RecipeView } from "@/components/RecipeView";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import {
  useAddMissingRecipeIngredientsToList,
  useCookPantryRecipe,
  useDeletePantryRecipe,
  usePantryRecipe,
  usePreviewCookPantryRecipe,
} from "@/hooks/usePantryRecipes";
import { useT } from "@/i18n";
import type { CookPreviewResult } from "@/services/api";

interface EditableDeduction {
  product_id: string;
  name: string;
  unit: string;
  available: number;
  quantity: string;
}

export default function ReceitaDetalhe() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const t = useT();

  const { recipe, isLoading } = usePantryRecipe(id);
  const deleteRecipe = useDeletePantryRecipe();
  const addMissing = useAddMissingRecipeIngredientsToList();
  const preview = usePreviewCookPantryRecipe();
  const cook = useCookPantryRecipe();

  const [cookOpen, setCookOpen] = useState(false);
  const [servings, setServings] = useState("");
  const [previewResult, setPreviewResult] = useState<CookPreviewResult | null>(null);
  const [deductions, setDeductions] = useState<EditableDeduction[]>([]);

  useEffect(() => {
    if (recipe && cookOpen) {
      setServings((prev) => prev || String(recipe.servings));
    }
  }, [recipe, cookOpen]);

  function openCookSheet() {
    setPreviewResult(null);
    setDeductions([]);
    setServings(String(recipe?.servings ?? 1));
    setCookOpen(true);
  }

  function handlePreview() {
    if (!id) return;
    const servingsNum = Number(servings);
    preview.mutate(
      { id, servings: Number.isFinite(servingsNum) && servingsNum > 0 ? servingsNum : undefined },
      {
        onSuccess: (data) => {
          setPreviewResult(data);
          setDeductions(
            data.deductions.map((d) => ({
              product_id: d.product_id,
              name: d.name,
              unit: d.unit,
              available: d.available,
              quantity: String(d.will_deduct),
            })),
          );
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  }

  function handleConfirmCook() {
    if (!id) return;
    const servingsNum = Number(servings) || recipe?.servings || 1;
    cook.mutate(
      {
        id,
        servings: servingsNum,
        deductions: deductions.map((d) => ({
          product_id: d.product_id,
          quantity: Number(d.quantity) || 0,
          unit: d.unit,
        })),
      },
      {
        onSuccess: () => {
          toast.success(t("recipes.cooked"));
          setCookOpen(false);
          navigate("/");
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
      },
    );
  }

  function handleDelete() {
    if (!id) return;
    if (!window.confirm(t("recipes.detail.confirmDelete"))) return;
    deleteRecipe.mutate(id, {
      onSuccess: () => {
        toast.success(t("recipes.detail.deleted"));
        navigate("/receitas");
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  }

  function handleAddMissing() {
    if (!id) return;
    addMissing.mutate(id, {
      onSuccess: (data) => toast.success(t("recipes.addedToList", { count: data.added })),
      onError: (err) => toast.error(err instanceof Error ? err.message : t("common.error")),
    });
  }

  if (isLoading || !recipe) {
    return (
      <div className="flex-1 min-h-0 flex flex-col">
        <PageHeader title={t("recipes.title")} showBack />
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm text-muted-foreground">
            {isLoading ? t("common.loading") : t("recipes.detail.notFound")}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader title={recipe.title} showBack />

      <div className="flex-1 overflow-y-auto px-4 py-4">
        <RecipeView recipe={recipe} showTitle={false} />
      </div>

      <div className="border-t border-border px-4 py-3 space-y-2 shrink-0 safe-bottom">
        <Button
          type="button"
          className="w-full h-12 rounded-xl font-heading font-semibold"
          onClick={openCookSheet}
        >
          {t("recipes.detail.cook")}
        </Button>
        <Button
          type="button"
          variant="outline"
          className="w-full h-11 rounded-xl"
          disabled={addMissing.isPending}
          onClick={handleAddMissing}
        >
          {t("recipes.addMissing")}
        </Button>
        <Button
          type="button"
          variant="ghost"
          className="w-full h-10 text-destructive"
          disabled={deleteRecipe.isPending}
          onClick={handleDelete}
        >
          {t("recipes.detail.delete")}
        </Button>
      </div>

      <Sheet open={cookOpen} onOpenChange={setCookOpen}>
        <SheetContent side="bottom" className="rounded-t-2xl max-h-[85vh] overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{t("recipes.cook.title")}</SheetTitle>
          </SheetHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="cookServings">{t("recipes.cook.servingsLabel")}</Label>
              <div className="flex items-center gap-2">
                <Input
                  id="cookServings"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  value={servings}
                  onChange={(e) => setServings(e.target.value)}
                  className="h-11 rounded-xl w-24"
                />
                <Button
                  type="button"
                  variant="outline"
                  className="h-11 rounded-xl flex-1"
                  disabled={preview.isPending}
                  onClick={handlePreview}
                >
                  {t("recipes.cook.preview")}
                </Button>
              </div>
            </div>

            {previewResult && (
              <div className="space-y-4">
                {deductions.length > 0 && (
                  <div className="space-y-2">
                    <p className="text-sm font-semibold">{t("recipes.cook.deductionsTitle")}</p>
                    {deductions.map((d, i) => (
                      <div key={d.product_id} className="flex items-center justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <p className="text-sm truncate">{d.name}</p>
                          <p className="text-xs text-muted-foreground">
                            {t("recipes.cook.available", { quantity: String(d.available), unit: d.unit })}
                          </p>
                        </div>
                        <Input
                          type="number"
                          inputMode="decimal"
                          min={0}
                          value={d.quantity}
                          onChange={(e) =>
                            setDeductions((prev) =>
                              prev.map((item, idx) => (idx === i ? { ...item, quantity: e.target.value } : item)),
                            )
                          }
                          className="h-10 rounded-lg w-24 shrink-0"
                        />
                      </div>
                    ))}
                  </div>
                )}

                {previewResult.missing.length > 0 && (
                  <div className="space-y-1">
                    <p className="text-sm font-semibold">{t("recipes.cook.missingTitle")}</p>
                    <ul className="text-sm text-muted-foreground space-y-1">
                      {previewResult.missing.map((ing, i) => (
                        <li key={`${ing.name}-${i}`}>
                          {ing.quantity} {ing.unit} de {ing.name}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                <Button
                  type="button"
                  className="w-full h-12 rounded-xl font-heading font-semibold"
                  disabled={cook.isPending}
                  onClick={handleConfirmCook}
                >
                  {t("recipes.cook.confirm")}
                </Button>
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
