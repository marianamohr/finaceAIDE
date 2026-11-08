import { useQuery } from "@tanstack/react-query";
import { api } from "@/services/api";
import { useAuthStore } from "@/store/auth";

export const PANTRY_SEED_PRODUCTS_KEY = ["pantry", "seed-products"] as const;

/** Catálogo global (~150 itens) usado pela tela de inventário inicial. */
export function usePantrySeedProducts() {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: PANTRY_SEED_PRODUCTS_KEY,
    queryFn: () => api.listPantrySeedProducts(),
    enabled: !!token,
    staleTime: Infinity, // catálogo global, muda raramente — não precisa refetch
  });

  return {
    seedProducts: query.data?.seed_products,
    isLoading: query.isLoading,
    error: query.error,
  };
}
