import { useQuery } from "@tanstack/react-query";
import { api } from "@/services/api";
import { useAuthStore } from "@/store/auth";

export const PANTRY_AI_USAGE_KEY = ["pantry", "ai-usage"] as const;

/** Uso/custo estimado de IA da dispensa no mês corrente — cartão em Ajustes. */
export function usePantryAIUsage() {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: PANTRY_AI_USAGE_KEY,
    queryFn: () => api.getPantryAIUsage(),
    enabled: !!token,
  });

  return {
    usage: query.data,
    isLoading: query.isLoading,
    error: query.error,
  };
}
