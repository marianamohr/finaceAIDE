import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { ConfirmationCard } from "./ConfirmationCard"
import type { PendingAction } from "@/services/api"

function renderCard(pa: PendingAction) {
  const qc = new QueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <ConfirmationCard pendingAction={pa} />
    </QueryClientProvider>,
  )
}

const baseDraft = {
  tx: { description: "Gasolina", amount: -100, category: "transporte", date: "2026-05-28" },
  needs: {
    field: "payment_method" as const,
    kind: "not_mentioned" as const,
    prompt: "Qual forma de pagamento você usou?",
    options: [{ payment_method_id: "1", label: "Nubank", kind: "credit_card" as const }],
  },
}

function makePA(detected?: string): PendingAction {
  return {
    id: "pa-1",
    tool_name: "clarify_payment_method",
    payload: {
      drafts: [{ ...baseDraft, needs: { ...baseDraft.needs, detected_payment_label: detected } }],
    },
  } as unknown as PendingAction
}

describe("ClarifyPaymentMethodCard context line", () => {
  it("mostra a linha de contexto quando detected_payment_label existe", () => {
    renderCard(makePA("Cartão de Crédito Visa"))
    expect(screen.getByText(/A nota indica: Cartão de Crédito Visa/)).toBeInTheDocument()
  })

  it("não mostra a linha quando ausente", () => {
    renderCard(makePA(undefined))
    expect(screen.queryByText(/A nota indica:/)).not.toBeInTheDocument()
  })
})
