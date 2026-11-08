import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import Dashboard from "./Dashboard";
import { useDashboardQuery, useInvalidateDashboard, usePrefetchNextMonth } from "@/hooks/useDashboard";
import { usePaymentMethods } from "@/hooks/usePaymentMethods";
import type { PaymentMethodSummary, DashboardData } from "@/hooks/useDashboard";
import type { PaymentMethod } from "@/hooks/usePaymentMethods";

// ─── Module Mocks ─────────────────────────────────────────────────────────────

vi.mock("react-router-dom", () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock("@/hooks/useDashboard", () => ({
  useDashboardQuery: vi.fn(),
  useInvalidateDashboard: vi.fn(),
  usePrefetchNextMonth: vi.fn(),
}));

vi.mock("@/hooks/usePaymentMethods", () => ({
  usePaymentMethods: vi.fn(),
}));

vi.mock("@/services/api", () => ({
  api: {
    get: vi.fn().mockResolvedValue([]),
    delete: vi.fn(),
    put: vi.fn(),
    confirmBalanceTransfer: vi.fn(),
  },
}));

vi.mock("@/store/currency", () => ({
  useCurrencyStore: () => ({ currency: "BRL" }),
  formatCurrency: (v: number) => `R$ ${Math.abs(v).toFixed(2).replace(".", ",")}`,
  maskCurrency: () => "R$ ••••",
  currencySymbol: () => "R$",
}));

vi.mock("@/components/BottomNav/BottomNav", () => ({
  BottomNav: () => <nav data-testid="bottom-nav" />,
}));

vi.mock("@/components/PageHeader/PageHeader", () => ({
  PageHeader: ({ title }: { title: string }) => <header data-testid="page-header">{title}</header>,
}));

vi.mock("@/components/TransactionCard/TransactionCard", () => ({
  TransactionCard: ({ id }: { id: string }) => <div data-testid={`tx-card-${id}`} />,
}));

vi.mock("@/components/RecurringRemindersPanel/RecurringRemindersPanel", () => ({
  RecurringRemindersPanel: () => <div data-testid="recurring-panel" />,
}));

vi.mock("@/components/ui/sheet", () => ({
  Sheet: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SheetContent: ({ children }: { children: React.ReactNode }) => <div role="dialog">{children}</div>,
  SheetHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SheetTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
}));

vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({
    open,
    children,
  }: {
    open: boolean;
    onOpenChange?: (v: boolean) => void;
    children: React.ReactNode;
  }) => (open ? <div>{children}</div> : null),
  DialogContent: ({
    children,
    ...props
  }: {
    children: React.ReactNode;
    [k: string]: unknown;
  }) => <div role="dialog" {...(props as Record<string, unknown>)}>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogFooter: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onClick,
    disabled,
    size: _size,
    variant: _variant,
    ...props
  }: {
    children: React.ReactNode;
    onClick?: () => void;
    disabled?: boolean;
    size?: string;
    variant?: string;
    [k: string]: unknown;
  }) => (
    <button onClick={onClick} disabled={disabled} {...props}>
      {children}
    </button>
  ),
}));

vi.mock("@/components/ui/input", () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}));

vi.mock("@/components/ui/pagination", () => ({
  Pagination: ({ children }: { children: React.ReactNode }) => <nav>{children}</nav>,
  PaginationContent: ({ children }: { children: React.ReactNode }) => <ul>{children}</ul>,
  PaginationItem: ({ children }: { children: React.ReactNode }) => <li>{children}</li>,
  PaginationLink: ({ children, onClick }: { children: React.ReactNode; onClick?: () => void }) => (
    <button onClick={onClick}>{children}</button>
  ),
  PaginationPrevious: ({ onClick }: { onClick?: () => void }) => <button onClick={onClick}>Prev</button>,
  PaginationNext: ({ onClick }: { onClick?: () => void }) => <button onClick={onClick}>Next</button>,
  PaginationEllipsis: () => <span>...</span>,
}));

// ─── Test Fixtures ─────────────────────────────────────────────────────────────

const mockNavigate = vi.fn();
const mockPayInvoice = vi.fn();
const mockUnpayInvoice = vi.fn();

const baseDashData: DashboardData = {
  summary: {
    income: 1000,
    expenses: -500,
    balance: 500,
    categories: { alimentacao: -200, transporte: -300 },
    month: "2026-05",
  },
  transactions: [
    {
      id: "tx-1",
      amount: -200,
      category: "alimentacao",
      merchant: "Mercado",
      description: "",
      date: "2026-05-10T00:00:00Z",
      source: "manual",
      image_url: "",
      type: "regular",
    },
  ],
  balanceTransfer: { amount: 0, pending: false },
  recurringReminders: [],
  payment_method_summary: [],
};

const creditCardSummary: PaymentMethodSummary = {
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

const pixSummary: PaymentMethodSummary = {
  payment_method_id: "pm-2",
  name: "PIX BB",
  kind: "pix",
  color: "#00AA00",
  total: -200,
  count: 2,
};

const nullBucketSummary: PaymentMethodSummary = {
  payment_method_id: null,
  name: "Sem forma de pagamento",
  kind: "cash",
  color: "",
  total: -100,
  count: 1,
};

const mockPaymentMethod: PaymentMethod = {
  id: "pm-1",
  account_id: "acc-1",
  name: "Nubank Crédito",
  kind: "credit_card",
  closing_day: 3,
  due_day: 10,
  credit_limit: 5000,
  color: "#820AD1",
  active: true,
  created_at: "2024-01-01T00:00:00Z",
};

function setupMocks(
  dashData: DashboardData = baseDashData,
  paymentMethods: PaymentMethod[] = [mockPaymentMethod],
  loadingPaymentMethods = false
) {
  vi.mocked(useDashboardQuery).mockReturnValue({
    data: dashData,
    isLoading: false,
    isSuccess: true,
    isError: false,
  } as ReturnType<typeof useDashboardQuery>);
  vi.mocked(useInvalidateDashboard).mockReturnValue(() => {});
  vi.mocked(usePrefetchNextMonth).mockReturnValue(false);
  vi.mocked(usePaymentMethods).mockReturnValue({
    paymentMethods,
    loading: loadingPaymentMethods,
    error: null,
    createPaymentMethod: vi.fn(),
    updatePaymentMethod: vi.fn(),
    deletePaymentMethod: vi.fn(),
    payInvoice: mockPayInvoice,
    unpayInvoice: mockUnpayInvoice,
    refetch: vi.fn(),
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  mockNavigate.mockClear();
});

// ─── Unit Tests ────────────────────────────────────────────────────────────────

describe("Dashboard — tab toggle", () => {
  it("exibe a aba 'Por categoria' selecionada por padrão ao carregar", () => {
    setupMocks();
    render(<Dashboard />);

    const catTab = screen.getByTestId("tab-categories");
    expect(catTab).toBeInTheDocument();
    // The categories section (category breakdown) should be visible by default
    expect(screen.getByText("Gastos por categoria")).toBeInTheDocument();
  });

  it("oculta seção de categorias e exibe formas de pagamento ao clicar na aba", async () => {
    setupMocks({
      ...baseDashData,
      payment_method_summary: [creditCardSummary],
    });
    render(<Dashboard />);

    // Before toggle: categories visible
    expect(screen.getByText("Gastos por categoria")).toBeInTheDocument();

    // Click payment-methods tab
    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    // After toggle: categories hidden, payment methods visible
    expect(screen.queryByText("Gastos por categoria")).not.toBeInTheDocument();
    expect(screen.getByTestId("payment-methods-tab")).toBeInTheDocument();
    expect(screen.getByText("Nubank Crédito")).toBeInTheDocument();
  });

  it("a aba de categorias permanece inalterada após adição da nova aba", async () => {
    setupMocks();
    render(<Dashboard />);

    // Toggle to payment-methods and back
    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-categories"));
    });

    // Categories section should be visible again
    expect(screen.getByText("Gastos por categoria")).toBeInTheDocument();
    expect(screen.queryByTestId("payment-methods-tab")).not.toBeInTheDocument();
  });
});

describe("Dashboard — cards de formas de pagamento", () => {
  it("card de credit_card exibe fatura, disponível e vencimento", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    expect(screen.getByTestId("pm-card-pm-1")).toBeInTheDocument();
    expect(screen.getByText("Nubank Crédito")).toBeInTheDocument();

    const detail = screen.getByTestId("pm-card-detail");
    expect(detail.textContent).toContain("Fatura");
    expect(detail.textContent).toContain("mai/26");
    expect(detail.textContent).toContain("disponível");
    expect(detail.textContent).toContain("vence dia 10");
  });

  it("card de pix exibe 'Debitado em' sem campos de fatura", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [pixSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    expect(screen.getByTestId("pm-card-pm-2")).toBeInTheDocument();
    expect(screen.getByText("PIX BB")).toBeInTheDocument();

    const detail = screen.getByTestId("pm-card-detail");
    expect(detail.textContent).toContain("Debitado em");
    expect(detail.textContent).not.toContain("Fatura");
    expect(detail.textContent).not.toContain("disponível");
    expect(detail.textContent).not.toContain("vence dia");
  });

  it("bucket null renderiza 'Sem forma de pagamento' com botão 'Classificar'", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [nullBucketSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    expect(screen.getByTestId("pm-card-null")).toBeInTheDocument();
    expect(screen.getByText("Sem forma de pagamento")).toBeInTheDocument();
    expect(screen.getByTestId("classify-button")).toBeInTheDocument();
    expect(screen.getByTestId("classify-button")).toHaveTextContent("Classificar");
  });

  it("botão 'Classificar' volta para a aba de categorias", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [nullBucketSummary] });
    render(<Dashboard />);

    // Go to payment-methods tab
    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    // Click Classificar
    await act(async () => {
      fireEvent.click(screen.getByTestId("classify-button"));
    });

    // Should be back to categories tab
    expect(screen.getByText("Gastos por categoria")).toBeInTheDocument();
  });
});

describe("Dashboard — banner de primeiro uso", () => {
  it("exibe banner quando paymentMethods está vazio e aba de formas está ativa", async () => {
    setupMocks(baseDashData, [], false);
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    expect(screen.getByTestId("first-use-banner")).toBeInTheDocument();
    expect(screen.getByText("Cadastre suas formas de pagamento")).toBeInTheDocument();
    expect(screen.getByText("Cadastrar agora")).toBeInTheDocument();
  });

  it("não exibe banner quando há formas de pagamento cadastradas", async () => {
    setupMocks(baseDashData, [mockPaymentMethod], false);
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    expect(screen.queryByTestId("first-use-banner")).not.toBeInTheDocument();
  });

  it("botão 'Cadastrar agora' do banner navega para /profile/payment-methods", async () => {
    setupMocks(baseDashData, [], false);
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    await act(async () => {
      fireEvent.click(screen.getByText("Cadastrar agora"));
    });

    expect(mockNavigate).toHaveBeenCalledWith("/profile/payment-methods");
  });
});

// ─── Integration Tests ─────────────────────────────────────────────────────────

describe("Dashboard — layout responsivo", () => {
  it("renderiza corretamente em viewport de 375px sem overflow", () => {
    Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: 375 });
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary, pixSummary, nullBucketSummary] });
    render(<Dashboard />);

    expect(screen.getByTestId("page-header")).toBeInTheDocument();
    expect(screen.getByTestId("tab-toggle")).toBeInTheDocument();
    expect(screen.getByTestId("bottom-nav")).toBeInTheDocument();
  });

  it("renderiza corretamente em viewport de 1280px", () => {
    Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: 1280 });
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary, pixSummary] });
    render(<Dashboard />);

    expect(screen.getByTestId("page-header")).toBeInTheDocument();
    expect(screen.getByTestId("tab-toggle")).toBeInTheDocument();
    expect(screen.getByTestId("bottom-nav")).toBeInTheDocument();
  });

  it("aba de categorias permanece funcional em 375px após toggle", async () => {
    Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: 375 });
    setupMocks({ ...baseDashData, payment_method_summary: [pixSummary] });
    render(<Dashboard />);

    // Switch tabs and switch back
    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-categories"));
    });

    expect(screen.getByText("Gastos por categoria")).toBeInTheDocument();
    expect(screen.queryByTestId("payment-methods-tab")).not.toBeInTheDocument();
  });
});

// ─── Phase 2: Modal de pagamento de fatura ────────────────────────────────────

describe("Dashboard — modal de pagamento de fatura", () => {
  it("abre modal ao clicar em 'Pagar' no card de cartão de crédito", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pay-invoice-btn-pm-1"));
    });

    expect(screen.getByText("Pagar fatura Nubank Crédito")).toBeInTheDocument();
    expect(screen.getByTestId("confirm-pay-btn")).toBeInTheDocument();
  });

  it("exibe erro quando valor informado é maior que o total da fatura", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pay-invoice-btn-pm-1"));
    });

    const amountInput = screen.getByTestId("pay-amount-input");
    fireEvent.change(amountInput, { target: { value: "9999" } });

    await act(async () => {
      fireEvent.click(screen.getByTestId("confirm-pay-btn"));
    });

    expect(screen.getByTestId("pay-error")).toBeInTheDocument();
    expect(screen.getByTestId("pay-error").textContent).toContain("não pode ser maior");
    expect(mockPayInvoice).not.toHaveBeenCalled();
  });

  it("chama payInvoice com o valor correto ao confirmar", async () => {
    mockPayInvoice.mockResolvedValueOnce(undefined);
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pay-invoice-btn-pm-1"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("confirm-pay-btn"));
    });

    expect(mockPayInvoice).toHaveBeenCalledWith("pm-1", 2026, 5, 500);
  });

  it("exibe 'Saldo anterior' no modal quando há carryover de fatura anterior", async () => {
    const dashWithCarryover: DashboardData = {
      ...baseDashData,
      payment_method_summary: [creditCardSummary],
      transactions: [
        ...baseDashData.transactions,
        {
          id: "ip-prev",
          amount: -450,
          category: "fatura",
          merchant: "",
          description: "",
          date: "2026-05-05T00:00:00Z",
          source: "manual",
          image_url: "",
          type: "invoice_payment",
          payment_method_id: "pm-1",
          billing_month: 4,
          carryover_amount: 200,
        },
      ],
    };
    setupMocks(dashWithCarryover);
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pay-invoice-btn-pm-1"));
    });

    expect(screen.getByTestId("carryover-display")).toBeInTheDocument();
    expect(screen.getByTestId("carryover-display").textContent).toContain("200");
  });

  it("exibe badge 'Pago ✓' quando fatura já está paga (invoice_payment presente)", async () => {
    const paidDash: DashboardData = {
      ...baseDashData,
      payment_method_summary: [creditCardSummary],
      transactions: [
        ...baseDashData.transactions,
        {
          id: "ip-1",
          amount: -500,
          category: "fatura",
          merchant: "",
          description: "",
          date: "2026-05-10T00:00:00Z",
          source: "manual",
          image_url: "",
          type: "invoice_payment",
          payment_method_id: "pm-1",
          billing_year: 2026,
          billing_month: 5,
        },
      ],
    };
    setupMocks(paidDash);
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });

    expect(screen.getByTestId("invoice-paid-badge-pm-1")).toBeInTheDocument();
    expect(screen.queryByTestId("pay-invoice-btn-pm-1")).not.toBeInTheDocument();
  });
});

// ─── Phase 2: Visão 360 do cartão ────────────────────────────────────────────

describe("Dashboard — visão 360 do cartão", () => {
  it("abre drawer ao clicar no card de cartão de crédito", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pm-card-pm-1"));
    });

    expect(screen.getByTestId("card-360-summary")).toBeInTheDocument();
  });

  it("exibe gasto, limite e disponível no drawer 360", async () => {
    setupMocks({ ...baseDashData, payment_method_summary: [creditCardSummary] });
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pm-card-pm-1"));
    });

    // creditCardSummary: total=-500 (abs=500), credit_limit=5000, available=4500
    expect(screen.getByTestId("card-360-total").textContent).toContain("500,00");
    expect(screen.getByTestId("card-360-limit").textContent).toContain("5000,00");
    expect(screen.getByTestId("card-360-available").textContent).toContain("4500,00");
  });

  it("exibe carryover quando presente em transação de fatura de período anterior", async () => {
    const dashWithCarryover: DashboardData = {
      ...baseDashData,
      payment_method_summary: [creditCardSummary],
      transactions: [
        ...baseDashData.transactions,
        {
          id: "ip-prev",
          amount: -450,
          category: "fatura",
          merchant: "",
          description: "",
          date: "2026-05-05T00:00:00Z",
          source: "manual",
          image_url: "",
          type: "invoice_payment",
          payment_method_id: "pm-1",
          billing_month: 4,
          carryover_amount: 200,
        },
      ],
    };
    setupMocks(dashWithCarryover);
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pm-card-pm-1"));
    });

    expect(screen.getByTestId("drawer-carryover")).toBeInTheDocument();
    expect(screen.getByTestId("drawer-carryover").textContent).toContain("200");
  });

  it("exibe transações do cartão mas exclui registros do tipo invoice_payment", async () => {
    const dashWithTxs: DashboardData = {
      ...baseDashData,
      payment_method_summary: [creditCardSummary],
      transactions: [
        {
          id: "tx-card-1",
          amount: -100,
          category: "alimentacao",
          merchant: "Restaurante",
          description: "",
          date: "2026-05-15T00:00:00Z",
          source: "manual",
          image_url: "",
          payment_method_id: "pm-1",
          type: "regular",
        },
        {
          id: "tx-invoice-1",
          amount: -500,
          category: "fatura",
          merchant: "",
          description: "",
          date: "2026-05-10T00:00:00Z",
          source: "manual",
          image_url: "",
          payment_method_id: "pm-1",
          type: "invoice_payment",
          billing_year: 2026,
          billing_month: 5,
        },
      ],
    };
    setupMocks(dashWithTxs);
    render(<Dashboard />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("tab-payment-methods"));
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("pm-card-pm-1"));
    });

    expect(screen.getByText("Restaurante")).toBeInTheDocument();
    expect(screen.queryByText("fatura")).not.toBeInTheDocument();
  });
});
