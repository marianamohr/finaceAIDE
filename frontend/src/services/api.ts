import { useAuthStore } from "@/store/auth";

const API_URL = import.meta.env.VITE_API_URL || "/api";
const REFRESH_TOKEN_KEY = "fai-r";

export function saveRefreshToken(token: string) {
  try { localStorage.setItem(REFRESH_TOKEN_KEY, token); } catch {}
}

export function loadRefreshToken(): string | null {
  try { return localStorage.getItem(REFRESH_TOKEN_KEY); } catch { return null; }
}

export function clearRefreshToken() {
  try { localStorage.removeItem(REFRESH_TOKEN_KEY); } catch {}
}

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = useAuthStore.getState().token;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_URL}${endpoint}`, {
    ...options,
    headers,
    credentials: "include",
  });

  if (res.status === 401 && useAuthStore.getState().isAuthenticated) {
    // Try refresh via cookie first, then fall back to stored refresh token (PWA)
    const storedRefresh = loadRefreshToken();
    const refreshHeaders: Record<string, string> = {};
    if (storedRefresh) {
      refreshHeaders["X-Refresh-Token"] = storedRefresh;
    }

    const refreshRes = await fetch(`${API_URL}/auth/refresh`, {
      method: "POST",
      credentials: "include",
      headers: refreshHeaders,
    });

    if (refreshRes.ok) {
      const data = await refreshRes.json();
      const newToken: string = data.access_token;
      if (data.refresh_token) saveRefreshToken(data.refresh_token);
      useAuthStore.getState().setToken(newToken);
      headers["Authorization"] = `Bearer ${newToken}`;
      const retry = await fetch(`${API_URL}${endpoint}`, { ...options, headers, credentials: "include" });
      if (!retry.ok) throw new Error(`Request failed: ${retry.status}`);
      if (retry.status === 204) return undefined as unknown as T;
      return retry.json();
    } else {
      useAuthStore.getState().logout();
      setTimeout(() => {
        clearRefreshToken();
        useAuthStore.persist.clearStorage();
        localStorage.removeItem("fai-r");
        localStorage.removeItem("financeai-auth");
        localStorage.removeItem("financeai-refresh");
      }, 0);
      throw new Error("Session expired");
    }
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Request failed" }));
    throw new Error(err.error || `Error ${res.status}`);
  }

  if (res.status === 204) return undefined as unknown as T;
  return res.json();
}

export interface AdminUser {
  id: string;
  email: string;
  role: string;
  status: string;
  account_id: string;
  show_tools: boolean;
  created_at: string;
}

export interface AdminMetrics {
  receipts_24h: number;
  extraction_success_rate: number;
  extraction_failures_24h: number;
}

export interface PaymentMethodOption {
  payment_method_id: string
  label: string
  kind: 'credit_card' | 'debit' | 'pix' | 'voucher' | 'cash'
}

export interface ClarificationNeed {
  field: 'payment_method'
  kind: 'ambiguous' | 'unknown_term' | 'not_mentioned'
  prompt: string
  options: PaymentMethodOption[]
  detected_payment_label?: string
}

export interface ClarifyDraft {
  tx: { description?: string; amount: number; category: string; date: string }
  needs: ClarificationNeed
}

export interface PendingAction {
  id: string
  tool_name: string
  preview: string
  payload?: { drafts?: ClarifyDraft[] }
}

export interface AgentInsight {
  id: string
  type: 'anomaly' | 'overspend' | 'projection' | 'suggestion'
  body: string
  read_at: string | null
  created_at: string
}

export interface ConfirmedTransaction {
  id: string
  description: string
  category: string
  amount: number
}

export interface ConfirmActionResponse {
  tool_name: string
  result: string
  transactions?: ConfirmedTransaction[]
}

export interface RecurringExpense {
  id: string
  account_id: string
  name: string
  amount: number
  category_id?: string
  payment_method_id?: string
  due_day?: number
  start_month?: number
  start_year?: number
  end_month?: number
  end_year?: number
  is_variable: boolean
  active: boolean
  created_at: string
  updated_at: string
}

export interface RecurringExpenseWithStatus {
  id: string
  name: string
  amount: number
  category_id?: string
  category_slug?: string
  payment_method_id?: string
  due_day?: number
  start_month?: number
  start_year?: number
  end_month?: number
  end_year?: number
  is_variable: boolean
  paid: boolean
  transaction_id?: string
  paid_amount?: number
}

export type PaymentMethodKind = 'credit_card' | 'debit' | 'pix' | 'voucher' | 'cash';

export interface PaymentMethod {
  id: string;
  account_id: string;
  name: string;
  kind: PaymentMethodKind;
  closing_day?: number;
  due_day?: number;
  credit_limit?: number;
  last_four_digits?: string;
  color: string;
  active: boolean;
  created_at: string;
}

export interface CashflowTransaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  payment_method_id?: string;
  kind: string;
  type: string;
  image_url?: string;
  source: string;
}

export interface CashflowInvoiceTransaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
}

export interface CashflowInvoice {
  payment_method_id: string;
  name: string;
  color: string;
  billing_year: number;
  billing_month: number;
  due_day?: number;
  closing_day?: number;
  total: number;
  is_paid: boolean;
  transactions: CashflowInvoiceTransaction[];
}

export interface CashflowData {
  summary: {
    income: number;
    real_outflow: number;
    reserve_transfers?: number;
  };
  real_transactions: CashflowTransaction[];
  invoices: CashflowInvoice[];
}

export const api = {
  get: <T>(url: string) => request<T>(url),
  post: <T>(url: string, body?: unknown) =>
    request<T>(url, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(url: string, body?: unknown) =>
    request<T>(url, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(url: string, body?: unknown) =>
    request<T>(url, { method: "PATCH", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(url: string) => request<T>(url, { method: "DELETE" }),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>("/user/password", {
      method: "POST",
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),

  getMe: () =>
    request<{
      id: string;
      email: string;
      role: string;
      status: string;
      account_id: string;
      category_context: string;
      show_goals: boolean;
      user_api_key: string;
      show_limits_categories: boolean;
      show_tools: boolean;
      base_currency: string;
      distribution_method: string;
      future_pct: number;
      onboarding_seen: boolean;
    }>("/user/me"),
  updateSettings: (patch: {
    category_context?: string;
    show_goals?: boolean;
    user_api_key?: string;
    show_limits_categories?: boolean;
    show_tools?: boolean;
    base_currency?: string;
    distribution_method?: string;
    future_pct?: number;
    onboarding_seen?: boolean;
  }) =>
    request<{
      category_context: string;
      show_goals: boolean;
      user_api_key: string;
      show_limits_categories: boolean;
      show_tools: boolean;
      base_currency: string;
      distribution_method: string;
      future_pct: number;
      onboarding_seen: boolean;
    }>("/user/settings", {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),

  getBalanceTransferPending: (year: number, month: number) =>
    request<{ amount: number; pending: boolean }>(`/transactions/balance-transfer/pending?year=${year}&month=${month}`),
  confirmBalanceTransfer: (amount: number, year: number, month: number) =>
    request<void>("/transactions/balance-transfer", {
      method: "POST",
      body: JSON.stringify({ amount, year, month }),
    }),
  confirmVoucherDeposit: (paymentMethodId: string, amount: number, year: number, month: number) =>
    request<void>("/transactions/voucher-deposit", {
      method: "POST",
      body: JSON.stringify({ payment_method_id: paymentMethodId, amount, year, month }),
    }),

  getCurrency: () => request<{ base_currency: string }>("/settings/currency"),
  setCurrency: (currency: string) =>
    request<void>("/settings/currency", {
      method: "PUT",
      body: JSON.stringify({ base_currency: currency }),
    }),
  getExchangeRate: (from: string, to: string) =>
    request<{ from: string; to: string; rate: number }>(`/exchange-rate?from=${from}&to=${to}`),

  // Admin
  getAdminUsers: () =>
    request<AdminUser[]>("/admin/users"),
  getAdminMetrics: () =>
    request<AdminMetrics>("/admin/metrics"),
  updateUserStatus: (id: string, status: string) =>
    request<{ status: string }>(`/admin/users/${id}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),
  updateUserAccount: (id: string, accountId: string) =>
    request<{ account_id: string }>(`/admin/users/${id}/account`, {
      method: "PATCH",
      body: JSON.stringify({ account_id: accountId }),
    }),
  updateUserShowTools: (id: string, showTools: boolean) =>
    request<{ show_tools: boolean }>(`/admin/users/${id}/settings`, {
      method: "PATCH",
      body: JSON.stringify({ show_tools: showTools }),
    }),

  confirmAction: (pendingActionId: string): Promise<ConfirmActionResponse> =>
    request<ConfirmActionResponse>("/agent/confirm", {
      method: "POST",
      body: JSON.stringify({ pending_action_id: pendingActionId }),
    }),

  confirmClarifyPaymentMethod: (
    pendingActionId: string,
    resolutions: { index: number; payment_method_id: string }[]
  ): Promise<{ tool_name: string; transactions?: ConfirmedTransaction[] }> =>
    request("/agent/confirm", {
      method: "POST",
      body: JSON.stringify({ pending_action_id: pendingActionId, resolutions }),
    }),

  getInsights: (unreadOnly = false): Promise<AgentInsight[]> =>
    request<AgentInsight[]>(`/agent/insights${unreadOnly ? "?unread=true" : ""}`),

  markInsightRead: (id: string): Promise<void> =>
    request<void>(`/agent/insights/${id}/read`, { method: "PATCH" }),

  // Recurring expenses
  getRecurringExpenses: () =>
    request<RecurringExpense[]>("/recurring-expenses"),
  createRecurringExpense: (body: { name: string; amount: number; category_id?: string; payment_method_id?: string; due_day?: number; start_month?: number; start_year?: number; end_month?: number; end_year?: number; is_variable?: boolean }) =>
    request<RecurringExpense>("/recurring-expenses", { method: "POST", body: JSON.stringify(body) }),
  updateRecurringExpense: (id: string, body: { name?: string; amount?: number; category_id?: string | null; payment_method_id?: string | null; due_day?: number | null; active?: boolean; start_month?: number | null; start_year?: number | null; end_month?: number | null; end_year?: number | null; is_variable?: boolean }) =>
    request<RecurringExpense>(`/recurring-expenses/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteRecurringExpense: (id: string) =>
    request<void>(`/recurring-expenses/${id}`, { method: "DELETE" }),
  getRecurringReminders: (year: number, month: number) =>
    request<RecurringExpenseWithStatus[]>(`/recurring-expenses/reminders?year=${year}&month=${month}`),
  generateCardTransactions: (year: number, month: number): Promise<void> =>
    request<void>(`/recurring-expenses/generate-card-transactions?year=${year}&month=${month}`, { method: "POST" }),
  markRecurringPaid: (id: string, year: number, month: number, paymentMethodId?: string, amount?: number) =>
    request<unknown>(`/recurring-expenses/${id}/pay`, {
      method: "POST",
      body: JSON.stringify({ year, month, payment_method_id: paymentMethodId ?? null, amount: amount ?? null }),
    }),
  unmarkRecurringPaid: (id: string, year: number, month: number) =>
    request<void>(`/recurring-expenses/${id}/pay?year=${year}&month=${month}`, { method: "DELETE" }),

  // Invoice pay/unpay
  payInvoice: (paymentMethodId: string, year: number, month: number, amount: number, debitPaymentMethodId: string) =>
    request<{ transaction: { id: string }; due_day?: number }>(
      `/payment-methods/${paymentMethodId}/invoices/${year}/${month}/pay`,
      { method: "POST", body: JSON.stringify({ amount, debit_payment_method_id: debitPaymentMethodId }) }
    ),
  unpayInvoice: (paymentMethodId: string, year: number, month: number) =>
    request<void>(`/payment-methods/${paymentMethodId}/invoices/${year}/${month}/pay`, { method: "DELETE" }),

  // Payment methods
  getPaymentMethods: () =>
    request<PaymentMethod[]>("/payment-methods"),
  createPaymentMethod: (body: { name: string; kind: PaymentMethodKind; closing_day?: number; due_day?: number; credit_limit?: number; last_four_digits?: string | null; color?: string }) =>
    request<PaymentMethod>("/payment-methods", { method: "POST", body: JSON.stringify(body) }),
  updatePaymentMethod: (id: string, body: { name?: string; kind?: PaymentMethodKind; closing_day?: number | null; due_day?: number | null; credit_limit?: number | null; last_four_digits?: string | null; color?: string; active?: boolean }) =>
    request<PaymentMethod>(`/payment-methods/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deletePaymentMethod: (id: string) =>
    request<void>(`/payment-methods/${id}`, { method: "DELETE" }),

  getCashflow: (year: number, month: number) =>
    request<CashflowData>(`/cashflow?year=${year}&month=${month}`),

  getDashboard: (year: number, month: number) =>
    request<{
      summary: {
        income: number;
        expenses: number;
        balance: number;
        categories: Record<string, number>;
        month: string;
      };
      transactions: Array<{
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
        carryover_amount?: number;
        type?: string;
      }>;
      balance_transfer: { amount: number; pending: boolean };
      recurring_reminders: Array<{
        id: string;
        name: string;
        amount: number;
        category_id?: string;
        category_slug?: string;
        payment_method_id?: string;
        due_day?: number;
        paid: boolean;
        transaction_id?: string;
      }>;
      voucher_summary: Array<{
        payment_method_id: string;
        name: string;
        color: string;
        balance: number;
        prev_balance: number;
        month_deposit: number;
        month_spent: number;
        last_deposit: number;
      }>;
      voucher_pending: Array<{
        payment_method_id: string;
        name: string;
        last_deposit: number;
      }>;
    }>(`/dashboard?year=${year}&month=${month}`),

  uploadImage: async (file: File, instruction?: string) => {
    const token = useAuthStore.getState().token;
    const formData = new FormData();
    formData.append("image", file);
    formData.append("client_datetime", new Date().toISOString());
    if (instruction) formData.append("instruction", instruction);

    const res = await fetch(`${API_URL}/upload`, {
      method: "POST",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: formData,
      credentials: "include",
    });

    if (!res.ok) {
      const body = await res.json().catch(() => null);
      const message = body?.error || `Erro ${res.status}`;
      throw new Error(message);
    }
    return res.json() as Promise<{ image_url: string; transactions: Array<{ category: string; amount: number }>; pending_action?: PendingAction }>;
  },
};
