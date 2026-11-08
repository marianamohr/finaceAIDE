import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  api,
  type ConfirmPantryReceiptInput,
  type PantryReceiptStatus,
  type UpdatePantryReceiptItemInput,
} from "@/services/api";
import { useAuthStore } from "@/store/auth";
import { PANTRY_PRODUCTS_KEY } from "@/hooks/usePantryProducts";

export const PANTRY_RECEIPTS_KEY = ["pantry", "receipts"] as const;

function pantryReceiptKey(id: string) {
  return [...PANTRY_RECEIPTS_KEY, id] as const;
}

/** Lista notas fiscais da dispensa, opcionalmente filtradas por status. */
export function usePantryReceipts(status?: PantryReceiptStatus) {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: [...PANTRY_RECEIPTS_KEY, status ?? "all"],
    queryFn: () => api.listPantryReceipts(status),
    enabled: !!token,
  });

  return {
    receipts: query.data?.receipts,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}

/** Busca uma nota fiscal específica com seus itens, para a tela de revisão. */
export function usePantryReceipt(id: string | undefined) {
  const token = useAuthStore((s) => s.token);

  const query = useQuery({
    queryKey: pantryReceiptKey(id ?? ""),
    queryFn: () => api.getPantryReceipt(id as string),
    enabled: !!id && !!token,
  });

  return {
    receipt: query.data?.receipt,
    items: query.data?.items,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}

function useInvalidatePantryReceipts() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: PANTRY_RECEIPTS_KEY });
}

/** Envia a foto da nota para extração via IA (multipart). */
export function useCreatePantryReceipt() {
  const invalidate = useInvalidatePantryReceipts();
  return useMutation({
    mutationFn: (input: Parameters<typeof api.createPantryReceipt>[0]) => api.createPantryReceipt(input),
    onSuccess: invalidate,
  });
}

/** Salva edições dos itens de uma nota (tela de revisão), sem confirmar. */
export function useUpdatePantryReceiptItems() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, items }: { id: string; items: UpdatePantryReceiptItemInput[] }) =>
      api.updatePantryReceiptItems(id, items),
    onSuccess: (_data, { id }) => {
      queryClient.invalidateQueries({ queryKey: pantryReceiptKey(id) });
    },
  });
}

/**
 * Confirma a nota: baixa os itens no estoque e, opcionalmente, cria a(s)
 * transação(ões) financeira(s). Invalida também os produtos da dispensa,
 * já que a confirmação muda o estoque.
 */
export function useConfirmPantryReceipt() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: ConfirmPantryReceiptInput }) =>
      api.confirmPantryReceipt(id, input),
    onSuccess: (_data, { id }) => {
      queryClient.invalidateQueries({ queryKey: PANTRY_RECEIPTS_KEY });
      queryClient.invalidateQueries({ queryKey: pantryReceiptKey(id) });
      queryClient.invalidateQueries({ queryKey: PANTRY_PRODUCTS_KEY });
    },
  });
}

/** Descarta uma nota pendente sem afetar o estoque. */
export function useDiscardPantryReceipt() {
  const invalidate = useInvalidatePantryReceipts();
  return useMutation({
    mutationFn: (id: string) => api.discardPantryReceipt(id),
    onSuccess: invalidate,
  });
}
