import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ConfirmCookDeduction, type GenerateRecipeInput, type PantryRecipe } from "@/services/api";
import { useAuthStore } from "@/store/auth";
import { PANTRY_PRODUCTS_KEY } from "@/hooks/usePantryProducts";
import { PANTRY_SHOPPING_LIST_KEY } from "@/hooks/usePantryShoppingList";

export const PANTRY_RECIPES_KEY = ["pantry", "recipes"] as const;

function pantryRecipeKey(id: string) {
  return [...PANTRY_RECIPES_KEY, id] as const;
}

function useInvalidatePantryRecipes() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: PANTRY_RECIPES_KEY });
}

/** Lista receitas salvas, opcionalmente só as favoritas. */
export function usePantryRecipes(favoriteOnly?: boolean) {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: [...PANTRY_RECIPES_KEY, favoriteOnly ? "favorite" : "all"],
    queryFn: () => api.listPantryRecipes(favoriteOnly),
    enabled: !!token,
  });

  return {
    recipes: query.data?.recipes,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}

/** Busca uma receita salva específica, para a tela de detalhe. */
export function usePantryRecipe(id: string | undefined) {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: pantryRecipeKey(id ?? ""),
    queryFn: () => api.getPantryRecipe(id as string),
    enabled: !!id && !!token,
  });

  return {
    recipe: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}

/** Gera uma receita via IA a partir da dispensa. Não salva nada ainda. */
export function useGeneratePantryRecipe() {
  return useMutation({
    mutationFn: (input: GenerateRecipeInput) => api.generatePantryRecipe(input),
  });
}

/** Salva (favorita) uma receita gerada ou criada pelo usuário. */
export function useSavePantryRecipe() {
  const invalidate = useInvalidatePantryRecipes();
  return useMutation({
    mutationFn: (recipe: PantryRecipe) => api.savePantryRecipe(recipe),
    onSuccess: invalidate,
  });
}

/** Remove uma receita salva. */
export function useDeletePantryRecipe() {
  const invalidate = useInvalidatePantryRecipes();
  return useMutation({
    mutationFn: (id: string) => api.deletePantryRecipe(id),
    onSuccess: invalidate,
  });
}

/** Calcula o que seria descontado do estoque ao cozinhar a receita agora. */
export function usePreviewCookPantryRecipe() {
  return useMutation({
    mutationFn: ({ id, servings }: { id: string; servings?: number }) => api.previewCookPantryRecipe(id, servings),
  });
}

/**
 * Confirma "vou fazer": desconta o estoque pelas deduções confirmadas.
 * Invalida receitas, produtos (estoque mudou) e a lista de compras (a
 * regeneração automática do backend pode adicionar/remover itens).
 */
export function useCookPantryRecipe() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      servings,
      deductions,
    }: {
      id: string;
      servings: number;
      deductions: ConfirmCookDeduction[];
    }) => api.cookPantryRecipe(id, servings, deductions),
    onSuccess: (_data, { id }) => {
      queryClient.invalidateQueries({ queryKey: PANTRY_RECIPES_KEY });
      queryClient.invalidateQueries({ queryKey: pantryRecipeKey(id) });
      queryClient.invalidateQueries({ queryKey: PANTRY_PRODUCTS_KEY });
      queryClient.invalidateQueries({ queryKey: PANTRY_SHOPPING_LIST_KEY });
    },
  });
}

/** Adiciona os ingredientes faltantes de uma receita salva à lista de compras. */
export function useAddMissingRecipeIngredientsToList() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (recipeId: string) => api.addMissingRecipeIngredientsToList(recipeId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PANTRY_SHOPPING_LIST_KEY });
    },
  });
}
