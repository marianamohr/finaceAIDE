import { useState, useEffect, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/services/api";

export interface Transaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
  image_url: string;
  payment_method_id?: string;
  billing_year?: number;
  billing_month?: number;
  type?: string;
  installment_purchase_id?: string | null;
  installment_number?: number | null;
}

export interface PaymentMode {
  id: string;
  slug: string;
  label: string;
}

export interface DashboardSummary {
  income: number;
  expenses: number;
  balance: number;
  categories: Record<string, number>;
  month: string;
}

interface DashboardState {
  summary: DashboardSummary | null;
  transactions: Transaction[];
  loading: boolean;
  error: string | null;
}

export function useDashboard(year: number, month: number) {
  const [state, setState] = useState<DashboardState>({
    summary: null,
    transactions: [],
    loading: true,
    error: null,
  });

  const fetch = useCallback(() => {
    setState((prev) => ({ ...prev, loading: true }));
    const qs = `?year=${year}&month=${month}`;
    Promise.all([
      api.get<DashboardSummary>(`/transactions/summary${qs}`),
      api.get<Transaction[]>(`/transactions${qs}`),
    ])
      .then(([summary, transactions]) => {
        setState({ summary, transactions, loading: false, error: null });
      })
      .catch((err: Error) => {
        setState((prev) => ({ ...prev, loading: false, error: err.message }));
      });
  }, [year, month]);

  useEffect(() => {
    fetch();
  }, [fetch]);

  useEffect(() => {
    window.addEventListener("transactions:changed", fetch)
    return () => window.removeEventListener("transactions:changed", fetch)
  }, [fetch]);

  const deleteTransaction = async (id: string) => {
    await api.delete(`/transactions/${id}`);
    fetch();
  };

  const updateTransaction = async (id: string, data: Partial<Omit<Transaction, "id" | "source" | "image_url">>) => {
    await api.put(`/transactions/${id}`, {
      amount: data.amount,
      category: data.category,
      merchant: data.merchant,
      description: data.description,
      date: data.date ? data.date.slice(0, 10) : undefined,
      source: "manual",
    });
    fetch();
  };

  return { ...state, refetch: fetch, deleteTransaction, updateTransaction };
}

export interface VoucherSummary {
  payment_method_id: string;
  name: string;
  color: string;
  balance: number;
  prev_balance: number;
  month_deposit: number;
  month_spent: number;
  last_deposit: number;
}

export interface VoucherPending {
  payment_method_id: string;
  name: string;
  last_deposit: number;
}

export interface PaymentMethodSummary {
  payment_method_id: string | null;
  name: string;
  kind: string;
  color: string;
  total: number;
  count: number;
  billing_year?: number;
  billing_month?: number;
  due_day?: number;
  credit_limit?: number;
  available?: number;
}

export interface DashboardData {
  summary: DashboardSummary;
  transactions: Transaction[];
  balanceTransfer: { amount: number; pending: boolean };
  recurringReminders: Array<{
    id: string;
    name: string;
    amount: number;
    category_id?: string;
    category_slug?: string;
    due_day?: number;
    paid: boolean;
    transaction_id?: string;
  }>;
  voucherSummary: VoucherSummary[];
  voucherPending: VoucherPending[];
  payment_method_summary?: PaymentMethodSummary[];
}

export function useDashboardQuery(year: number, month: number) {
  return useQuery({
    queryKey: ["dashboard", year, month],
    queryFn: async () => {
      await api.generateCardTransactions(year, month).catch(() => {});
      const data = await api.getDashboard(year, month);
      return {
        summary: data.summary,
        transactions: data.transactions,
        balanceTransfer: data.balance_transfer,
        recurringReminders: data.recurring_reminders,
        voucherSummary: data.voucher_summary ?? [],
        voucherPending: data.voucher_pending ?? [],
        payment_method_summary: data.payment_method_summary ?? [],
      } satisfies DashboardData;
    },
    staleTime: 60 * 1000,
  });
}

export function useInvalidateDashboard() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: ["dashboard"] });
}

export function usePrefetchNextMonth(year: number, month: number): boolean {
  const [hasData, setHasData] = useState(false);

  useEffect(() => {
    setHasData(false);
    let cancelled = false;
    const next = new Date(year, month, 1); // month é 1-based, Date usa 0-based internamente
    const nextYear = next.getFullYear();
    const nextMonth = next.getMonth() + 1;

    api
      .get<DashboardSummary>(`/transactions/summary?year=${nextYear}&month=${nextMonth}`)
      .then((summary) => {
        if (!cancelled) {
          setHasData(summary.income !== 0 || summary.expenses !== 0);
        }
      })
      .catch(() => {
        if (!cancelled) setHasData(false);
      });

    return () => {
      cancelled = true;
    };
  }, [year, month]);

  return hasData;
}
