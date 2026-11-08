import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { RecurringRemindersPanel } from "./RecurringRemindersPanel";
import { useRecurringReminders } from "@/hooks/useRecurringExpenses";
import { usePaymentMethods } from "@/hooks/usePaymentMethods";
import type { RecurringExpenseWithStatus } from "@/services/api";
import type { PaymentMethod } from "@/hooks/usePaymentMethods";

vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock("@/hooks/useRecurringExpenses", () => ({
  useRecurringReminders: vi.fn(),
}));

vi.mock("@/hooks/usePaymentMethods", () => ({
  usePaymentMethods: vi.fn(),
}));

vi.mock("@/hooks/useDashboard", () => ({
  useInvalidateDashboard: () => () => {},
}));

vi.mock("@/store/currency", () => ({
  useCurrencyStore: () => ({ currency: "BRL" }),
  formatCurrency: (v: number) => `R$ ${Math.abs(v).toFixed(2).replace(".", ",")}`,
  maskCurrency: () => "R$ ••••",
  currencySymbol: () => "R$",
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const nubank: PaymentMethod = {
  id: "pm-1",
  account_id: "acc-1",
  name: "Nubank",
  kind: "credit_card",
  color: "#820AD1",
  active: true,
  created_at: "2024-01-01T00:00:00Z",
};

const baseReminderHook = {
  reminders: [] as RecurringExpenseWithStatus[],
  isLoading: false,
  pendingCount: 0,
  markPaid: vi.fn(),
  unmarkPaid: vi.fn(),
  refetch: vi.fn(),
};

const basePaymentHook = {
  paymentMethods: [nubank] as PaymentMethod[],
  loading: false,
  error: null,
  createPaymentMethod: vi.fn(),
  updatePaymentMethod: vi.fn(),
  deletePaymentMethod: vi.fn(),
  payInvoice: vi.fn(),
  unpayInvoice: vi.fn(),
  refetch: vi.fn(),
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(usePaymentMethods).mockReturnValue({ ...basePaymentHook });
});

describe("RecurringRemindersPanel — chip de forma de pagamento", () => {
  it("exibe chip de forma de pagamento quando reminder tem payment_method_id", () => {
    const reminder: RecurringExpenseWithStatus = {
      id: "r-1",
      name: "Netflix",
      amount: 49.9,
      payment_method_id: "pm-1",
      paid: false,
    };

    vi.mocked(useRecurringReminders).mockReturnValue({
      ...baseReminderHook,
      reminders: [reminder],
      pendingCount: 1,
    });

    render(<RecurringRemindersPanel year={2026} month={5} />);

    expect(screen.getByTestId("reminder-pm-chip-r-1")).toBeInTheDocument();
    expect(screen.getByTestId("reminder-pm-chip-r-1").textContent).toContain("Nubank");
  });

  it("não exibe chip quando reminder não tem payment_method_id", () => {
    const reminder: RecurringExpenseWithStatus = {
      id: "r-2",
      name: "Aluguel",
      amount: 1500,
      paid: false,
    };

    vi.mocked(useRecurringReminders).mockReturnValue({
      ...baseReminderHook,
      reminders: [reminder],
      pendingCount: 1,
    });

    render(<RecurringRemindersPanel year={2026} month={5} />);

    expect(screen.queryByTestId("reminder-pm-chip-r-2")).not.toBeInTheDocument();
  });

  it("não renderiza quando não há lembretes", () => {
    vi.mocked(useRecurringReminders).mockReturnValue({
      ...baseReminderHook,
      reminders: [],
      pendingCount: 0,
    });

    const { container } = render(<RecurringRemindersPanel year={2026} month={5} />);
    expect(container.firstChild).toBeNull();
  });
});
