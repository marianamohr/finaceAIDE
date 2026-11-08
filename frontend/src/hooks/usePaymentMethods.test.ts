import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods";
import { api } from "@/services/api";

vi.mock("@/services/api", () => ({
  api: {
    getPaymentMethods: vi.fn(),
    createPaymentMethod: vi.fn(),
    updatePaymentMethod: vi.fn(),
    deletePaymentMethod: vi.fn(),
  },
}));

const mockMethod: PaymentMethod = {
  id: "pm-1",
  account_id: "acc-1",
  name: "Nubank Crédito",
  kind: "credit_card",
  closing_day: 3,
  due_day: 10,
  color: "#820AD1",
  active: true,
  created_at: "2024-01-01T00:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("usePaymentMethods", () => {
  it("initializes with loading=true and empty paymentMethods", () => {
    vi.mocked(api.getPaymentMethods).mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => usePaymentMethods());
    expect(result.current.loading).toBe(true);
    expect(result.current.paymentMethods).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("sets loading=false and paymentMethods after successful fetch", async () => {
    vi.mocked(api.getPaymentMethods).mockResolvedValue([mockMethod]);
    const { result } = renderHook(() => usePaymentMethods());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.paymentMethods).toEqual([mockMethod]);
    expect(result.current.error).toBeNull();
  });

  it("sets error after API failure", async () => {
    vi.mocked(api.getPaymentMethods).mockRejectedValue(new Error("Network error"));
    const { result } = renderHook(() => usePaymentMethods());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe("Network error");
    expect(result.current.paymentMethods).toEqual([]);
  });

  it("createPaymentMethod calls API and refetches", async () => {
    vi.mocked(api.getPaymentMethods).mockResolvedValue([]);
    vi.mocked(api.createPaymentMethod).mockResolvedValue(mockMethod);
    const { result } = renderHook(() => usePaymentMethods());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.createPaymentMethod({ name: "Nubank Crédito", kind: "credit_card", closing_day: 3 });
    });

    expect(api.createPaymentMethod).toHaveBeenCalledWith({ name: "Nubank Crédito", kind: "credit_card", closing_day: 3 });
    expect(api.getPaymentMethods).toHaveBeenCalledTimes(2);
  });

  it("deletePaymentMethod calls API with correct ID and refetches", async () => {
    vi.mocked(api.getPaymentMethods).mockResolvedValue([mockMethod]);
    vi.mocked(api.deletePaymentMethod).mockResolvedValue(undefined);
    const { result } = renderHook(() => usePaymentMethods());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.deletePaymentMethod("pm-1");
    });

    expect(api.deletePaymentMethod).toHaveBeenCalledWith("pm-1");
    expect(api.getPaymentMethods).toHaveBeenCalledTimes(2);
  });

  it("updatePaymentMethod calls PUT with correct ID and refetches", async () => {
    vi.mocked(api.getPaymentMethods).mockResolvedValue([mockMethod]);
    vi.mocked(api.updatePaymentMethod).mockResolvedValue({ ...mockMethod, name: "Nubank Atualizado" });
    const { result } = renderHook(() => usePaymentMethods());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.updatePaymentMethod("pm-1", { name: "Nubank Atualizado" });
    });

    expect(api.updatePaymentMethod).toHaveBeenCalledWith("pm-1", { name: "Nubank Atualizado" });
    expect(api.getPaymentMethods).toHaveBeenCalledTimes(2);
  });
});
