import { useState, useEffect, useCallback } from "react";
import { api, PaymentMethod, PaymentMethodKind } from "@/services/api";

export type { PaymentMethod, PaymentMethodKind };

export function usePaymentMethods() {
  const [paymentMethods, setPaymentMethods] = useState<PaymentMethod[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refetch = useCallback(() => {
    setLoading(true);
    api.getPaymentMethods()
      .then((data) => { setPaymentMethods(data); setError(null); })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { refetch(); }, [refetch]);

  const createPaymentMethod = async (body: Parameters<typeof api.createPaymentMethod>[0]) => {
    await api.createPaymentMethod(body);
    refetch();
  };

  const updatePaymentMethod = async (id: string, body: Parameters<typeof api.updatePaymentMethod>[1]) => {
    await api.updatePaymentMethod(id, body);
    refetch();
  };

  const deletePaymentMethod = async (id: string) => {
    await api.deletePaymentMethod(id);
    refetch();
  };

  const payInvoice = async (paymentMethodId: string, year: number, month: number, amount: number, debitPaymentMethodId: string) => {
    await api.payInvoice(paymentMethodId, year, month, amount, debitPaymentMethodId);
  };

  const unpayInvoice = async (paymentMethodId: string, year: number, month: number) => {
    await api.unpayInvoice(paymentMethodId, year, month);
  };

  return { paymentMethods, loading, error, createPaymentMethod, updatePaymentMethod, deletePaymentMethod, payInvoice, unpayInvoice, refetch };
}
