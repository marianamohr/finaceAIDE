import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  api,
  type AddPantryShoppingItemInput,
  type PantryShoppingStatus,
} from "@/services/api";
import { useAuthStore } from "@/store/auth";

export const PANTRY_SHOPPING_LIST_KEY = ["pantry", "shopping-list"] as const;

/**
 * Lista os itens da lista de compras. Sem `status`, o backend devolve apenas
 * os itens "open" (tela principal); passe "all" para ver todos os status.
 */
export function usePantryShoppingList(status?: PantryShoppingStatus | "all") {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: [...PANTRY_SHOPPING_LIST_KEY, status ?? "open"],
    queryFn: () => api.listPantryShoppingItems(status),
    enabled: !!token,
  });

  return {
    items: query.data?.items,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}

function useInvalidatePantryShoppingList() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: PANTRY_SHOPPING_LIST_KEY });
}

/** Adiciona um item manual à lista de compras. */
export function useAddPantryShoppingItem() {
  const invalidate = useInvalidatePantryShoppingList();
  return useMutation({
    mutationFn: (input: AddPantryShoppingItemInput) => api.addPantryShoppingItem(input),
    onSuccess: invalidate,
  });
}

/** Muda o status de um item (comprado/dispensado/reaberto). */
export function useUpdatePantryShoppingItemStatus() {
  const invalidate = useInvalidatePantryShoppingList();
  return useMutation({
    mutationFn: ({ id, status, quantity }: { id: string; status: PantryShoppingStatus; quantity?: number }) =>
      api.updatePantryShoppingItemStatus(id, status, quantity),
    onSuccess: invalidate,
  });
}

/** Dispara a regeneração automática da lista (estoque zerado/abaixo do mínimo). */
export function useRegeneratePantryShoppingList() {
  const invalidate = useInvalidatePantryShoppingList();
  return useMutation({
    mutationFn: () => api.regeneratePantryShoppingList(),
    onSuccess: invalidate,
  });
}
