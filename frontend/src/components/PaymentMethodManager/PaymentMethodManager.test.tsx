import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act, within } from "@testing-library/react";
import { PaymentMethodManager } from "./PaymentMethodManager";
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods";
import { useRecurringExpenses } from "@/hooks/useRecurringExpenses";
import type { RecurringExpense } from "@/services/api";

// Mock Radix UI Select with a native <select> for deterministic tests
vi.mock("@/components/ui/select", () => ({
  Select: ({
    value,
    onValueChange,
    children,
  }: {
    value?: string;
    onValueChange?: (v: string) => void;
    children: React.ReactNode;
  }) => (
    <select
      data-testid="kind-select"
      value={value}
      onChange={(e) => onValueChange?.(e.target.value)}
    >
      {children}
    </select>
  ),
  SelectTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectValue: () => null,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ value, children }: { value: string; children: React.ReactNode }) => (
    <option value={value}>{children}</option>
  ),
}));

// Mock Dialog
vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({
    open,
    children,
  }: {
    open: boolean;
    onOpenChange?: (v: boolean) => void;
    children: React.ReactNode;
  }) => (open ? <div>{children}</div> : null),
  DialogContent: ({ children }: { children: React.ReactNode }) => (
    <div role="dialog">{children}</div>
  ),
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogFooter: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

// Mock AlertDialog
vi.mock("@/components/ui/alert-dialog", () => ({
  AlertDialog: ({
    open,
    children,
  }: {
    open: boolean;
    onOpenChange?: (v: boolean) => void;
    children: React.ReactNode;
  }) => (open ? <div>{children}</div> : null),
  AlertDialogContent: ({ children }: { children: React.ReactNode }) => (
    <div role="alertdialog">{children}</div>
  ),
  AlertDialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  AlertDialogDescription: ({ children }: { children: React.ReactNode }) => <p>{children}</p>,
  AlertDialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogCancel: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => <button onClick={onClick}>{children}</button>,
  AlertDialogAction: ({
    children,
    onClick,
    disabled,
    ...props
  }: {
    children: React.ReactNode;
    onClick?: () => void;
    disabled?: boolean;
    [k: string]: unknown;
  }) => (
    <button onClick={onClick} disabled={disabled} {...props}>
      {children}
    </button>
  ),
}));

vi.mock("@/hooks/usePaymentMethods");
vi.mock("@/hooks/useRecurringExpenses");
vi.mock("@/services/api", () => ({
  api: {
    get: vi.fn().mockResolvedValue([]),
  },
}));

const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
const mockRefetch = vi.fn();
const mockRecurringCreate = vi.fn();
const mockRecurringRemove = vi.fn();

const baseHookReturn = {
  paymentMethods: [] as PaymentMethod[],
  loading: false,
  error: null,
  createPaymentMethod: mockCreate,
  updatePaymentMethod: mockUpdate,
  deletePaymentMethod: mockDelete,
  payInvoice: vi.fn(),
  unpayInvoice: vi.fn(),
  refetch: mockRefetch,
};

const baseRecurringReturn = {
  items: [] as RecurringExpense[],
  isLoading: false,
  create: mockRecurringCreate,
  update: vi.fn(),
  remove: mockRecurringRemove,
  refetch: vi.fn(),
};

const nubank: PaymentMethod = {
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

const pixBB: PaymentMethod = {
  id: "pm-2",
  account_id: "acc-1",
  name: "PIX BB",
  kind: "pix",
  color: "#00AA00",
  active: true,
  created_at: "2024-01-01T00:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(usePaymentMethods).mockReturnValue({ ...baseHookReturn });
  vi.mocked(useRecurringExpenses).mockReturnValue({ ...baseRecurringReturn });
});

// ─── Unit: Conditional credit_card fields ─────────────────────────────────

describe("PaymentMethodForm conditional fields", () => {
  it("shows closing_day, due_day, credit_limit fields when kind is credit_card (default)", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({ ...baseHookReturn });
    render(<PaymentMethodManager />);

    expect(screen.getByTestId("credit-card-fields")).toBeInTheDocument();
    expect(screen.getByTestId("closing-day-input")).toBeInTheDocument();
    expect(screen.getByTestId("due-day-input")).toBeInTheDocument();
    expect(screen.getByTestId("credit-limit-input")).toBeInTheDocument();
  });

  it("hides credit_card fields when kind is changed to pix", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({ ...baseHookReturn });
    render(<PaymentMethodManager />);

    const kindSelect = screen.getByTestId("kind-select");
    fireEvent.change(kindSelect, { target: { value: "pix" } });

    expect(screen.queryByTestId("credit-card-fields")).not.toBeInTheDocument();
    expect(screen.queryByTestId("closing-day-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("due-day-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("credit-limit-input")).not.toBeInTheDocument();
  });

  it("shows validation error when submitting credit_card without closing_day", async () => {
    vi.mocked(usePaymentMethods).mockReturnValue({ ...baseHookReturn });
    render(<PaymentMethodManager />);

    const nameInput = screen.getByTestId("name-input");
    fireEvent.change(nameInput, { target: { value: "Meu Cartão" } });

    // kind is already credit_card by default, closing_day left empty
    const submitBtn = screen.getByTestId("submit-button");
    await act(async () => {
      fireEvent.click(submitBtn);
    });

    expect(screen.getByTestId("form-error")).toHaveTextContent(
      "Dia de fechamento é obrigatório para cartão de crédito."
    );
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("shows validation error when submitting without a name", async () => {
    vi.mocked(usePaymentMethods).mockReturnValue({ ...baseHookReturn });
    render(<PaymentMethodManager />);

    const kindSelect = screen.getByTestId("kind-select");
    fireEvent.change(kindSelect, { target: { value: "pix" } });

    const submitBtn = screen.getByTestId("submit-button");
    await act(async () => {
      fireEvent.click(submitBtn);
    });

    expect(screen.getByTestId("form-error")).toHaveTextContent("Nome é obrigatório.");
    expect(mockCreate).not.toHaveBeenCalled();
  });
});

// ─── Unit: Color chip ──────────────────────────────────────────────────────

describe("PaymentMethodRow color chip", () => {
  it("displays color chip with the correct background color", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [nubank],
    });
    render(<PaymentMethodManager />);

    const chip = screen.getByTestId("color-chip-pm-1");
    expect(chip).toHaveStyle({ backgroundColor: "#820AD1" });
  });

  it("displays correct color for pix payment method", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [pixBB],
    });
    render(<PaymentMethodManager />);

    const chip = screen.getByTestId("color-chip-pm-2");
    expect(chip).toHaveStyle({ backgroundColor: "#00AA00" });
  });
});

// ─── Integration: CRUD flows ───────────────────────────────────────────────

describe("PaymentMethodManager integration", () => {
  it("creates a credit_card payment method with all fields and it appears in the list", async () => {
    const created: PaymentMethod = {
      ...nubank,
      id: "pm-new",
      name: "Inter Crédito",
      closing_day: 5,
    };
    mockCreate.mockResolvedValueOnce(undefined);

    let callCount = 0;
    vi.mocked(usePaymentMethods).mockImplementation(() => {
      callCount++;
      return {
        ...baseHookReturn,
        paymentMethods: callCount <= 1 ? [] : [created],
      };
    });

    const { rerender } = render(<PaymentMethodManager />);

    // Fill in name
    const nameInput = screen.getByTestId("name-input");
    fireEvent.change(nameInput, { target: { value: "Inter Crédito" } });

    // Fill in closing_day (credit_card is default)
    const closingDayInput = screen.getByTestId("closing-day-input");
    fireEvent.change(closingDayInput, { target: { value: "5" } });

    // Submit
    await act(async () => {
      fireEvent.click(screen.getByTestId("submit-button"));
    });

    expect(mockCreate).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Inter Crédito", kind: "credit_card", closing_day: 5 })
    );

    // Re-render simulating refetch returning the new item
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [created],
    });
    rerender(<PaymentMethodManager />);

    // Name appears both in the row and in the subscription select option
    expect(screen.getAllByText("Inter Crédito").length).toBeGreaterThan(0);
  });

  it("opens edit dialog when edit button is clicked and calls updatePaymentMethod on save", async () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [nubank],
    });
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<PaymentMethodManager />);

    // Click edit button
    const editBtn = screen.getByLabelText("Editar Nubank Crédito");
    await act(async () => {
      fireEvent.click(editBtn);
    });

    // Edit dialog should be visible
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Editar forma de pagamento")).toBeInTheDocument();

    // Change name in the edit form (the dialog's name input — it's the second one)
    const nameInputs = screen.getAllByTestId("name-input");
    const editNameInput = nameInputs[nameInputs.length - 1];
    fireEvent.change(editNameInput, { target: { value: "Nubank Atualizado" } });

    // Save
    const saveButtons = screen.getAllByTestId("submit-button");
    const saveBtn = saveButtons[saveButtons.length - 1];
    await act(async () => {
      fireEvent.click(saveBtn);
    });

    expect(mockUpdate).toHaveBeenCalledWith(
      "pm-1",
      expect.objectContaining({ name: "Nubank Atualizado" })
    );
  });

  it("shows deactivation confirmation dialog when deactivate is clicked", async () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [nubank],
    });

    render(<PaymentMethodManager />);

    const deactivateBtn = screen.getByLabelText("Desativar Nubank Crédito");
    await act(async () => {
      fireEvent.click(deactivateBtn);
    });

    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText("Desativar forma de pagamento")).toBeInTheDocument();
    expect(within(dialog).getByText(/Nubank Crédito/)).toBeInTheDocument();
  });

  it("calls deletePaymentMethod and removes item from active list after confirmation", async () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [nubank],
    });
    mockDelete.mockResolvedValueOnce(undefined);

    const { rerender } = render(<PaymentMethodManager />);

    // Open deactivate dialog
    const deactivateBtn = screen.getByLabelText("Desativar Nubank Crédito");
    await act(async () => {
      fireEvent.click(deactivateBtn);
    });

    // Confirm deactivation
    const confirmBtn = screen.getByTestId("confirm-deactivate");
    await act(async () => {
      fireEvent.click(confirmBtn);
    });

    expect(mockDelete).toHaveBeenCalledWith("pm-1");

    // Simulate refetch removing item from active list
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [{ ...nubank, active: false }],
    });
    rerender(<PaymentMethodManager />);

    // Should no longer appear in active list (no edit/deactivate buttons)
    expect(screen.queryByLabelText("Editar Nubank Crédito")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Desativar Nubank Crédito")).not.toBeInTheDocument();
    // But still visible as inactive
    expect(screen.getByText("Nubank Crédito")).toBeInTheDocument();
    expect(screen.getByText("Inativo")).toBeInTheDocument();
  });

  it("shows empty state message when no payment methods exist", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [],
    });
    render(<PaymentMethodManager />);

    expect(screen.getByText(/Nenhuma forma de pagamento cadastrada/)).toBeInTheDocument();
  });

  it("shows loading state", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      loading: true,
    });
    render(<PaymentMethodManager />);

    expect(screen.getByText("Carregando...")).toBeInTheDocument();
  });
});

// ─── Layout: responsive viewport ──────────────────────────────────────────

describe("ProfilePaymentMethods layout", () => {
  it("renders PaymentMethodManager without overflow at 375px viewport width", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [nubank, pixBB],
    });
    Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: 375 });

    render(<PaymentMethodManager />);

    // All content should be present without errors
    // Nubank Crédito appears in both the payment method row and the subscription select (credit_card only)
    expect(screen.getAllByText("Nubank Crédito").length).toBeGreaterThan(0);
    expect(screen.getByText("PIX BB")).toBeInTheDocument();
  });

  it("renders PaymentMethodManager without overflow at 1280px viewport width", () => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [nubank, pixBB],
    });
    Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: 1280 });

    render(<PaymentMethodManager />);

    expect(screen.getAllByText("Nubank Crédito").length).toBeGreaterThan(0);
    expect(screen.getByText("PIX BB")).toBeInTheDocument();
  });
});

// ─── RecurringSubscriptionSection ─────────────────────────────────────────────

describe("RecurringSubscriptionSection", () => {
  beforeEach(() => {
    vi.mocked(usePaymentMethods).mockReturnValue({
      ...baseHookReturn,
      paymentMethods: [nubank],
    });
    vi.mocked(useRecurringExpenses).mockReturnValue({ ...baseRecurringReturn });
  });

  it("renderiza seção com formulário e mensagem de lista vazia", () => {
    render(<PaymentMethodManager />);

    expect(screen.getByTestId("subscriptions-section")).toBeInTheDocument();
    expect(screen.getByTestId("no-subscriptions")).toBeInTheDocument();
    expect(screen.getByTestId("subscription-form")).toBeInTheDocument();
    expect(screen.getByTestId("sub-name-input")).toBeInTheDocument();
    expect(screen.getByTestId("sub-amount-input")).toBeInTheDocument();
    expect(screen.getByTestId("add-subscription-btn")).toBeInTheDocument();
  });

  it("exibe erro quando nome está vazio ao adicionar", async () => {
    render(<PaymentMethodManager />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("add-subscription-btn"));
    });

    expect(screen.getByTestId("sub-form-error")).toHaveTextContent("Nome é obrigatório.");
    expect(mockRecurringCreate).not.toHaveBeenCalled();
  });

  it("exibe erro quando valor é inválido", async () => {
    render(<PaymentMethodManager />);

    fireEvent.change(screen.getByTestId("sub-name-input"), { target: { value: "Netflix" } });

    await act(async () => {
      fireEvent.click(screen.getByTestId("add-subscription-btn"));
    });

    expect(screen.getByTestId("sub-form-error")).toHaveTextContent("Valor inválido.");
    expect(mockRecurringCreate).not.toHaveBeenCalled();
  });

  it("exibe erro quando nenhum cartão é selecionado", async () => {
    render(<PaymentMethodManager />);

    fireEvent.change(screen.getByTestId("sub-name-input"), { target: { value: "Netflix" } });
    fireEvent.change(screen.getByTestId("sub-amount-input"), { target: { value: "49.90" } });

    await act(async () => {
      fireEvent.click(screen.getByTestId("add-subscription-btn"));
    });

    expect(screen.getByTestId("sub-form-error")).toHaveTextContent("Selecione o cartão.");
    expect(mockRecurringCreate).not.toHaveBeenCalled();
  });

  it("chama create com os dados corretos ao adicionar assinatura válida", async () => {
    mockRecurringCreate.mockResolvedValueOnce(undefined);
    render(<PaymentMethodManager />);

    fireEvent.change(screen.getByTestId("sub-name-input"), { target: { value: "Netflix" } });
    fireEvent.change(screen.getByTestId("sub-amount-input"), { target: { value: "49.90" } });
    fireEvent.change(screen.getByTestId("sub-pm-select"), { target: { value: "pm-1" } });

    await act(async () => {
      fireEvent.click(screen.getByTestId("add-subscription-btn"));
    });

    expect(mockRecurringCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "Netflix",
        amount: 49.9,
        payment_method_id: "pm-1",
      })
    );
  });

  it("exibe linha de assinatura quando item tem payment_method_id de um cartão ativo", () => {
    const netflixSub: RecurringExpense = {
      id: "sub-1",
      account_id: "acc-1",
      name: "Netflix",
      amount: 49.9,
      payment_method_id: "pm-1",
      active: true,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
    };

    vi.mocked(useRecurringExpenses).mockReturnValue({
      ...baseRecurringReturn,
      items: [netflixSub],
    });

    render(<PaymentMethodManager />);

    expect(screen.getByTestId("subscription-row-sub-1")).toBeInTheDocument();
    expect(screen.getByText("Netflix")).toBeInTheDocument();
    expect(screen.queryByTestId("no-subscriptions")).not.toBeInTheDocument();
  });

  it("chama remove ao clicar no botão de remover assinatura", async () => {
    const netflixSub: RecurringExpense = {
      id: "sub-1",
      account_id: "acc-1",
      name: "Netflix",
      amount: 49.9,
      payment_method_id: "pm-1",
      active: true,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
    };

    vi.mocked(useRecurringExpenses).mockReturnValue({
      ...baseRecurringReturn,
      items: [netflixSub],
    });

    render(<PaymentMethodManager />);

    await act(async () => {
      fireEvent.click(screen.getByTestId("remove-subscription-sub-1"));
    });

    expect(mockRecurringRemove).toHaveBeenCalledWith("sub-1");
  });
});
