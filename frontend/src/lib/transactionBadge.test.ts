import { describe, it, expect } from "vitest";
import { getTransactionBadge } from "./transactionBadge";

describe("getTransactionBadge", () => {
  it("returns Assinatura for recurring with billing_year", () => {
    const badge = getTransactionBadge("recurring", 2026);
    expect(badge.label).toBe("Assinatura");
    expect(badge.icon).toBe("💳");
    expect(badge.color).toBe("#6366f1");
  });

  it("returns Recorrente for recurring without billing_year", () => {
    const badge = getTransactionBadge("recurring", undefined);
    expect(badge.label).toBe("Recorrente");
    expect(badge.icon).toBe("🔁");
    expect(badge.color).toBe("#10b981");
  });

  it("returns Avulso for manual source", () => {
    const badge = getTransactionBadge("manual", undefined);
    expect(badge.label).toBe("Avulso");
    expect(badge.icon).toBe("📌");
    expect(badge.color).toBe("#64748b");
  });

  it("returns Avulso for photo source", () => {
    const badge = getTransactionBadge("photo", 2026);
    expect(badge.label).toBe("Avulso");
    expect(badge.icon).toBe("📌");
    expect(badge.color).toBe("#64748b");
  });

  it("returns Avulso for balance_transfer source", () => {
    const badge = getTransactionBadge("balance_transfer", undefined);
    expect(badge.label).toBe("Avulso");
  });

  it("returns a Parcela badge when installmentNumber is set", () => {
    const badge = getTransactionBadge("installment", undefined, 3);
    expect(badge.label).toBe("Parcela 3");
    expect(badge.icon).toBe("📅");
    expect(badge.color).toBe("#f59e0b");
  });

  it("installment badge takes precedence over source", () => {
    const badge = getTransactionBadge("recurring", 2026, 1);
    expect(badge.label).toBe("Parcela 1");
  });
});
