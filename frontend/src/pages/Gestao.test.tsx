import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import Gestao from "./Gestao"

const mockNavigate = vi.fn()

const mockSettings = { showGoals: true, showTools: false, showLimitsCategories: true }

vi.mock("react-router-dom", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => ({ pathname: "/gestao" }),
}))

vi.mock("@/store/settings", () => ({
  useSettingsStore: (selector: (s: typeof mockSettings) => unknown) =>
    selector(mockSettings),
}))

vi.mock("@/components/BottomNav/BottomNav", () => ({
  BottomNav: () => <div data-testid="bottom-nav" />,
}))

vi.mock("@/components/PageHeader/PageHeader", () => ({
  PageHeader: ({ title }: { title: string }) => <div data-testid="page-header">{title}</div>,
}))

beforeEach(() => {
  mockNavigate.mockClear()
  mockSettings.showGoals = true
  mockSettings.showLimitsCategories = true
  mockSettings.showTools = false
})

describe("Gestao hub page", () => {
  it("renderiza o título 'Gestão'", () => {
    render(<Gestao />)
    expect(screen.getByTestId("page-header")).toHaveTextContent("Gestão")
  })

  it("exibe cards Painel e Projeção sempre", () => {
    render(<Gestao />)
    expect(screen.getByText("Painel")).toBeInTheDocument()
    expect(screen.getByText("Projeção")).toBeInTheDocument()
  })

  it("exibe cards Limites e Reservas quando flags ativas", () => {
    render(<Gestao />)
    expect(screen.getByText("Limites")).toBeInTheDocument()
    expect(screen.getByText("Reservas")).toBeInTheDocument()
  })

  it("navega para /painel ao clicar no card Painel", () => {
    render(<Gestao />)
    fireEvent.click(screen.getByText("Painel").closest("button")!)
    expect(mockNavigate).toHaveBeenCalledWith("/painel")
  })

  it("navega para /projecao ao clicar no card Projeção", () => {
    render(<Gestao />)
    fireEvent.click(screen.getByText("Projeção").closest("button")!)
    expect(mockNavigate).toHaveBeenCalledWith("/projecao")
  })

  it("oculta card Limites quando showLimitsCategories é false", () => {
    mockSettings.showLimitsCategories = false
    render(<Gestao />)
    expect(screen.queryByText("Limites")).not.toBeInTheDocument()
  })

  it("oculta card Reservas quando showGoals é false", () => {
    mockSettings.showGoals = false
    render(<Gestao />)
    expect(screen.queryByText("Reservas")).not.toBeInTheDocument()
  })

  it("renderiza o BottomNav", () => {
    render(<Gestao />)
    expect(screen.getByTestId("bottom-nav")).toBeInTheDocument()
  })
})
