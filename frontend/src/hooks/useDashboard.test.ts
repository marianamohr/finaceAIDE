import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import { useDashboardQuery, PaymentMethodSummary, Transaction } from "@/hooks/useDashboard";
import { api } from "@/services/api";

vi.mock("@/services/api", () => ({
  api: {
    getDashboard: vi.fn(),
    get: vi.fn(),
  },
}));

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const baseSummary = {
  income: 1000,
  expenses: -500,
  balance: 500,
  categories: { alimentacao: -200 },
  month: "2026-05",
};

const baseTransaction = {
  id: "tx-1",
  amount: -200,
  category: "alimentacao",
  merchant: "Mercado",
  description: "",
  date: "2026-05-10T00:00:00Z",
  source: "manual",
  image_url: "",
  type: "regular",
};

const baseDashboardResponse = {
  summary: baseSummary,
  transactions: [baseTransaction],
  balance_transfer: { amount: 0, pending: false },
  recurring_reminders: [],
  payment_method_summary: [],
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useDashboardQuery", () => {
  it("retorna payment_method_summary como array vazio quando backend retorna array vazio", async () => {
    vi.mocked(api.getDashboard).mockResolvedValue(baseDashboardResponse);
    const { result } = renderHook(() => useDashboardQuery(2026, 5), {
      wrapper: makeWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.payment_method_summary).toEqual([]);
  });

  it("retorna payment_method_summary preenchido quando backend retorna dados", async () => {
    const pmSummary: PaymentMethodSummary = {
      payment_method_id: "pm-1",
      name: "Nubank Crédito",
      kind: "credit_card",
      color: "#820AD1",
      total: -500,
      count: 3,
      billing_year: 2026,
      billing_month: 5,
      due_day: 10,
      credit_limit: 5000,
      available: 4500,
    };
    vi.mocked(api.getDashboard).mockResolvedValue({
      ...baseDashboardResponse,
      payment_method_summary: [pmSummary],
    });
    const { result } = renderHook(() => useDashboardQuery(2026, 5), {
      wrapper: makeWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.payment_method_summary).toHaveLength(1);
    expect(result.current.data?.payment_method_summary[0].payment_method_id).toBe("pm-1");
    expect(result.current.data?.payment_method_summary[0].billing_year).toBe(2026);
  });

  it("inclui o bucket null (sem forma de pagamento) no payment_method_summary", async () => {
    const nullBucket: PaymentMethodSummary = {
      payment_method_id: null,
      name: "Sem forma de pagamento",
      kind: "cash",
      color: "",
      total: -100,
      count: 1,
    };
    vi.mocked(api.getDashboard).mockResolvedValue({
      ...baseDashboardResponse,
      payment_method_summary: [nullBucket],
    });
    const { result } = renderHook(() => useDashboardQuery(2026, 5), {
      wrapper: makeWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.payment_method_summary[0].payment_method_id).toBeNull();
  });

  it("usa array vazio como fallback quando backend omite payment_method_summary", async () => {
    const responseWithoutSummary = {
      summary: baseSummary,
      transactions: [],
      balance_transfer: { amount: 0, pending: false },
      recurring_reminders: [],
      payment_method_summary: undefined as unknown as [],
    };
    vi.mocked(api.getDashboard).mockResolvedValue(responseWithoutSummary);
    const { result } = renderHook(() => useDashboardQuery(2026, 5), {
      wrapper: makeWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.payment_method_summary).toEqual([]);
  });
});

describe("PaymentMethodSummary type", () => {
  it("aceita objeto com apenas campos obrigatórios (sem opcionais de crédito)", () => {
    const summary: PaymentMethodSummary = {
      payment_method_id: "pm-2",
      name: "PIX BB",
      kind: "pix",
      color: "#0000FF",
      total: -50,
      count: 1,
    };
    expect(summary.payment_method_id).toBe("pm-2");
    expect(summary.billing_year).toBeUndefined();
    expect(summary.credit_limit).toBeUndefined();
    expect(summary.available).toBeUndefined();
  });

  it("aceita objeto com todos os campos incluindo os opcionais de crédito", () => {
    const summary: PaymentMethodSummary = {
      payment_method_id: "pm-3",
      name: "Visa Ouro",
      kind: "credit_card",
      color: "#FFD700",
      total: -1200,
      count: 5,
      billing_year: 2026,
      billing_month: 5,
      due_day: 15,
      credit_limit: 10000,
      available: 8800,
    };
    expect(summary.billing_year).toBe(2026);
    expect(summary.billing_month).toBe(5);
    expect(summary.due_day).toBe(15);
    expect(summary.credit_limit).toBe(10000);
    expect(summary.available).toBe(8800);
  });

  it("aceita payment_method_id null para o bucket sem forma de pagamento", () => {
    const summary: PaymentMethodSummary = {
      payment_method_id: null,
      name: "Sem forma de pagamento",
      kind: "cash",
      color: "",
      total: -30,
      count: 2,
    };
    expect(summary.payment_method_id).toBeNull();
  });
});

describe("Transaction type", () => {
  it("contém payment_method_id e não contém payment_mode_id", () => {
    const tx: Transaction = {
      id: "tx-1",
      amount: -100,
      category: "alimentacao",
      merchant: "Padaria",
      description: "",
      date: "2026-05-01T00:00:00Z",
      source: "manual",
      image_url: "",
      payment_method_id: "pm-1",
    };
    expect(tx.payment_method_id).toBe("pm-1");
    // @ts-expect-error payment_mode_id deve ter sido removido da interface
    expect(tx.payment_mode_id).toBeUndefined();
  });

  it("aceita billing_year e billing_month opcionais", () => {
    const tx: Transaction = {
      id: "tx-2",
      amount: -200,
      category: "transporte",
      merchant: "Uber",
      description: "",
      date: "2026-05-05T00:00:00Z",
      source: "chat",
      image_url: "",
      billing_year: 2026,
      billing_month: 5,
    };
    expect(tx.billing_year).toBe(2026);
    expect(tx.billing_month).toBe(5);
  });
});
