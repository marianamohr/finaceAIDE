import { useQuery } from "@tanstack/react-query";
import { api } from "@/services/api";
import { useAuthStore } from "@/store/auth";

export const PAYMENT_METHODS_KEY = ["payment-methods"] as const;

/**
 * Lista as formas de pagamento ativas da conta (mesmo endpoint do app
 * financeiro, já que dispensaAI consome o mesmo backend/conta). Usada para
 * montar o seletor de forma de pagamento na confirmação de uma nota fiscal.
 */
export function usePaymentMethods() {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: PAYMENT_METHODS_KEY,
    queryFn: () => api.listPaymentMethods(),
    enabled: !!token,
  });

  return {
    paymentMethods: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}
