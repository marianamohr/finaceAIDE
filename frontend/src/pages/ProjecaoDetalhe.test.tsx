import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import ProjecaoDetalhe from "./ProjecaoDetalhe"

vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn(),
}))

vi.mock("@/components/BottomNav/BottomNav", () => ({
  BottomNav: () => <div data-testid="bottom-nav" />,
}))

vi.mock("@/components/PageHeader/PageHeader", () => ({
  PageHeader: ({ title }: { title: string }) => <div data-testid="page-header">{title}</div>,
}))

vi.mock("@/store/currency", () => ({
  useCurrencyStore: () => ({ currency: "BRL" }),
  formatCurrency: (v: number) => `R$ ${v.toFixed(2)}`,
  maskCurrency: () => "R$ •••",
}))

vi.mock("@/store/privacy", () => ({
  usePrivacyStore: (sel: (s: { valuesHidden: boolean }) => unknown) => sel({ valuesHidden: false }),
}))

const mockData = [
  {
    year: 2026,
    month: 7,
    label: "Jul/26",
    fixed: 3180,
    installments: 300,
    card: 120,
    total: 3600,
    items: {
      fixed: [
        { name: "Aluguel", amount: 2800, category_label: "casa" },
        { name: "Condomínio", amount: 380, category_label: "casa" },
        { name: "Plano de saúde", amount: 320, category_label: "saude" },
      ],
      installments: [
        { name: "Notebook", amount: 300, installment_info: "3/10", card_name: "Nubank Ana" },
      ],
      card: [
        { name: "Claude", amount: 120, card_name: "Nubank Ana" },
      ],
    },
  },
]

vi.mock("@/hooks/useAnalytics", () => ({
  useExpenseProjectionDetail: () => ({ loading: false, error: false, data: mockData }),
}))

describe("ProjecaoDetalhe — AccordionGroup", () => {
  it("exibe header do grupo de fixos com label da categoria", () => {
    render(<ProjecaoDetalhe />)
    expect(screen.getByText(/Casa/i)).toBeInTheDocument()
    expect(screen.getByText(/Saúde/i)).toBeInTheDocument()
  })

  it("itens de fixos ficam ocultos por padrão", () => {
    render(<ProjecaoDetalhe />)
    expect(screen.queryByText("Aluguel")).not.toBeInTheDocument()
    expect(screen.queryByText("Plano de saúde")).not.toBeInTheDocument()
  })

  it("expande grupo de fixos ao clicar no header", () => {
    render(<ProjecaoDetalhe />)
    fireEvent.click(screen.getByRole("button", { name: /Casa/i }))
    expect(screen.getByText("Aluguel")).toBeInTheDocument()
    expect(screen.getByText("Condomínio")).toBeInTheDocument()
    expect(screen.queryByText("Plano de saúde")).not.toBeInTheDocument()
  })

  it("exibe um único grupo Nubank Ana (parcelas e cartão unificados)", () => {
    render(<ProjecaoDetalhe />)
    expect(screen.getAllByText(/Nubank Ana/i)).toHaveLength(1)
  })

  it("itens de cartão e parcelas ficam ocultos por padrão", () => {
    render(<ProjecaoDetalhe />)
    expect(screen.queryByText("Claude")).not.toBeInTheDocument()
    expect(screen.queryByText("Notebook")).not.toBeInTheDocument()
  })

  it("expande cartão e mostra parcelas e gastos regulares juntos", () => {
    render(<ProjecaoDetalhe />)
    fireEvent.click(screen.getByRole("button", { name: /Nubank Ana/i }))
    expect(screen.getByText("Notebook")).toBeInTheDocument()
    expect(screen.getByText("parcela 3/10")).toBeInTheDocument()
    expect(screen.getByText("Claude")).toBeInTheDocument()
  })
})
