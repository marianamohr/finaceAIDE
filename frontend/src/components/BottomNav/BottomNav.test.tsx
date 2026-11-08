import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import { BottomNav } from "./BottomNav"

const mockNavigate = vi.fn()
let mockPathname = "/chat"

vi.mock("react-router-dom", () => ({
  useLocation: () => ({ pathname: mockPathname }),
  useNavigate: () => mockNavigate,
}))

vi.mock("@/store/settings", () => ({
  useSettingsStore: (selector: (s: { showGoals: boolean; showTools: boolean; showLimitsCategories: boolean }) => unknown) =>
    selector({ showGoals: true, showTools: false, showLimitsCategories: true }),
}))

vi.mock("@/hooks/useRecurringExpenses", () => ({
  useRecurringReminders: () => ({ pendingCount: 0 }),
}))

beforeEach(() => {
  mockPathname = "/chat"
  mockNavigate.mockClear()
})

describe("BottomNav", () => {
  it("exibe os 4 itens fixos: Chat, Consumo, Caixa, Gestão", () => {
    render(<BottomNav />)
    expect(screen.getByText("Chat")).toBeInTheDocument()
    expect(screen.getByText("Consumo")).toBeInTheDocument()
    expect(screen.getByText("Caixa")).toBeInTheDocument()
    expect(screen.getByText("Gestão")).toBeInTheDocument()
  })

  it("não exibe Painel, Limites nem Reservas como itens do nav", () => {
    render(<BottomNav />)
    expect(screen.queryByText("Painel")).not.toBeInTheDocument()
    expect(screen.queryByText("Limites")).not.toBeInTheDocument()
    expect(screen.queryByText("Reservas")).not.toBeInTheDocument()
  })

  it("item Gestão fica ativo quando pathname é /gestao", () => {
    mockPathname = "/gestao"
    render(<BottomNav />)
    const btn = screen.getByText("Gestão").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })

  it("item Gestão fica ativo quando pathname é /painel (sub-rota)", () => {
    mockPathname = "/painel"
    render(<BottomNav />)
    const btn = screen.getByText("Gestão").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })

  it("item Gestão fica ativo quando pathname é /projecao (sub-rota)", () => {
    mockPathname = "/projecao"
    render(<BottomNav />)
    const btn = screen.getByText("Gestão").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })

  it("item Chat fica ativo quando pathname é /chat", () => {
    mockPathname = "/chat"
    render(<BottomNav />)
    const btn = screen.getByText("Chat").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })
})
