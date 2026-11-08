import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/services/api";

export type { CashflowData, CashflowInvoice, CashflowTransaction, CashflowInvoiceTransaction } from "@/services/api";

export function useCashflowQuery(year: number, month: number) {
  return useQuery({
    queryKey: ["cashflow", year, month],
    queryFn: () => api.getCashflow(year, month),
    staleTime: 60 * 1000,
  });
}

export function useInvalidateCashflow() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: ["cashflow"] });
}
