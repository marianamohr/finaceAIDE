export interface TransactionBadge {
  icon: string;
  label: string;
  color: string;
}

export function getTransactionBadge(
  source: string,
  billingYear?: number,
  installmentNumber?: number | null,
): TransactionBadge {
  if (installmentNumber != null) {
    return { icon: "📅", label: `Parcela ${installmentNumber}`, color: "#f59e0b" };
  }
  if (source === "recurring") {
    if (billingYear != null) {
      return { icon: "💳", label: "Assinatura", color: "#6366f1" };
    }
    return { icon: "🔁", label: "Recorrente", color: "#10b981" };
  }
  return { icon: "📌", label: "Avulso", color: "#64748b" };
}
