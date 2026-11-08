import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  api,
  type CreatePantryProductInput,
  type PantryProductListFilters,
  type PantryProductUnit,
  type UpdatePantryProductInput,
} from "@/services/api";
import { useAuthStore } from "@/store/auth";

export const PANTRY_PRODUCTS_KEY = ["pantry", "products"] as const;

/** Lista produtos da dispensa, refazendo o fetch quando os filtros mudam. */
export function usePantryProducts(filters?: PantryProductListFilters) {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: [...PANTRY_PRODUCTS_KEY, filters ?? {}],
    queryFn: () => api.listPantryProducts(filters),
    enabled: !!token,
  });

  return {
    products: query.data?.products,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}

function useInvalidatePantryProducts() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: PANTRY_PRODUCTS_KEY });
}

export function useCreatePantryProduct() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: (input: CreatePantryProductInput) => api.createPantryProduct(input),
    onSuccess: invalidate,
  });
}

export function useUpdatePantryProduct() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdatePantryProductInput }) =>
      api.updatePantryProduct(id, input),
    onSuccess: invalidate,
  });
}

export function useDeletePantryProduct() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: (id: string) => api.deletePantryProduct(id),
    onSuccess: invalidate,
  });
}

export function useConsumePantryProduct() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: ({ id, quantity, note }: { id: string; quantity: number; note?: string }) =>
      api.consumePantryProduct(id, quantity, note),
    onSuccess: invalidate,
  });
}

export function useMarkPantryProductOut() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: (id: string) => api.markPantryProductOut(id),
    onSuccess: invalidate,
  });
}

export function useAdjustPantryProduct() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: ({ id, quantity, note }: { id: string; quantity: number; note?: string }) =>
      api.adjustPantryProduct(id, quantity, note),
    onSuccess: invalidate,
  });
}

/** Consumo por texto livre ("usei 2 ovos") — Etapa 5. */
export function useConsumePantryProductFromText() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: (text: string) => api.consumePantryProductFromText(text),
    onSuccess: invalidate,
  });
}

export function useChangePantryProductUnit() {
  const invalidate = useInvalidatePantryProducts();
  return useMutation({
    mutationFn: ({ id, unit }: { id: string; unit: PantryProductUnit }) =>
      api.changePantryProductUnit(id, unit),
    onSuccess: invalidate,
  });
}
