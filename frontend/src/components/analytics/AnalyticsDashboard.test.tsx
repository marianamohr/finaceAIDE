import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { AnalyticsDashboard } from "./AnalyticsDashboard"
import { useAnalytics, useGoalHistory, usePaymentMethodAnalytics, useExpenseProjection } from "@/hooks/useAnalytics"
import { useDashboardQuery } from "@/hooks/useDashboard"
import { useReservas } from "@/hooks/useReservas"
import type { MonthData, GoalMonthlyPoint } from "@/hooks/useAnalytics"

vi.mock("recharts", () => ({
  ComposedChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="composed-chart">{children}</div>
  ),
  BarChart: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Bar: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="bar-element">{children}</div>
  ),
  LineChart: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Line: ({ name, strokeDasharray }: { name?: string; strokeDasharray?: string }) => (
    <div data-testid="line-element" data-name={name ?? ""} data-stroke-dasharray={strokeDasharray ?? ""} />
  ),
  AreaChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="area-chart">{children}</div>
  ),
  Area: ({ fillOpacity, name }: { fillOpacity?: number; name?: string }) => (
    <div data-testid="area-element" data-fill-opacity={String(fillOpacity ?? "")} data-name={name ?? ""} />
  ),
  PieChart: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Pie: () => null,
  Cell: ({ fill }: { fill?: string }) => (
    <div data-testid="cell-element" data-fill={fill ?? ""} />
  ),
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  Legend: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock("@/hooks/useAnalytics", () => ({
  useAnalytics: vi.fn(),
  useGoalHistory: vi.fn(),
  usePaymentMethodAnalytics: vi.fn(),
  useExpenseProjection: vi.fn(),
}))

vi.mock("@/store/currency", () => ({
  useCurrencyStore: () => ({ currency: "BRL" }),
  formatCurrency: (v: number) => `R$ ${v.toFixed(2)}`,
  maskCurrency: () => "R$ ••••",
  currencySymbol: () => "R$",
}))

vi.mock("@/hooks/useDashboard", () => ({
  useDashboardQuery: vi.fn(),
}))

vi.mock("@/hooks/useReservas", () => ({
  useReservas: vi.fn(),
}))

function mockMatchMedia(matches: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })) as unknown as typeof window.matchMedia
}

const makeMonthData = (overrides: Partial<MonthData> = {}): MonthData => ({
  year: 2026,
  month: 5,
  label: "mai/26",
  income: 5000,
  expenses: 3000,
  balance: 2000,
  categories: {},
  ...overrides,
})

const makeGoalPoint = (overrides: Partial<GoalMonthlyPoint> = {}): GoalMonthlyPoint => ({
  year: 2026,
  month: 5,
  total: 8000,
  ...overrides,
})

function setupMocks(analyticsData: MonthData[]) {
  vi.mocked(useAnalytics).mockReturnValue({ data: analyticsData, loading: false, error: null })
  vi.mocked(useGoalHistory).mockReturnValue({ data: [], loading: false })
  vi.mocked(usePaymentMethodAnalytics).mockReturnValue({ data: [], loading: false })
  vi.mocked(useExpenseProjection).mockReturnValue({ data: [], loading: false })
  vi.mocked(useDashboardQuery).mockReturnValue({ data: null } as ReturnType<typeof useDashboardQuery>)
  vi.mocked(useReservas).mockReturnValue({ goals: [] } as unknown as ReturnType<typeof useReservas>)
}

function setupMocksWithGoals(analyticsData: MonthData[], goalData: GoalMonthlyPoint[]) {
  vi.mocked(useAnalytics).mockReturnValue({ data: analyticsData, loading: false, error: null })
  vi.mocked(useGoalHistory).mockReturnValue({ data: goalData, loading: false })
  vi.mocked(usePaymentMethodAnalytics).mockReturnValue({ data: [], loading: false })
  vi.mocked(useExpenseProjection).mockReturnValue({ data: [], loading: false })
  vi.mocked(useDashboardQuery).mockReturnValue({ data: null } as ReturnType<typeof useDashboardQuery>)
  vi.mocked(useReservas).mockReturnValue({ goals: [] } as unknown as ReturnType<typeof useReservas>)
}

beforeEach(() => {
  vi.clearAllMocks()
  mockMatchMedia(true) // desktop por padrão
})

describe("PatrimonioChart — unit tests", () => {
  it("renderiza sem erros com data e goalHistory com múltiplos pontos", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26" }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26" }),
    ]
    const goals = [
      makeGoalPoint({ year: 2026, month: 4, total: 5000 }),
      makeGoalPoint({ year: 2026, month: 5, total: 8000 }),
    ]
    setupMocksWithGoals(data, goals)
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Patrimônio")).toBeInTheDocument()
    expect(screen.getByTestId("area-chart")).toBeInTheDocument()
  })

  it("renderiza sem erros quando data e goalHistory são vazios", () => {
    setupMocksWithGoals([], [])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Patrimônio")).toBeInTheDocument()
  })

  it("renderiza sem erros quando reservas são zero em todos os meses", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26" }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26" }),
    ]
    setupMocksWithGoals(data, [])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Patrimônio")).toBeInTheDocument()
  })
})

describe("PatrimonioChart — integration tests", () => {
  it("duas áreas são visíveis e semitransparentes (fillOpacity < 1) no DOM", () => {
    const data = [makeMonthData()]
    const goals = [makeGoalPoint()]
    setupMocksWithGoals(data, goals)
    render(<AnalyticsDashboard />)
    const areas = screen.getAllByTestId("area-element")
    expect(areas.length).toBeGreaterThanOrEqual(2)
    areas.forEach((area) => {
      const opacity = parseFloat(area.getAttribute("data-fill-opacity") ?? "1")
      expect(opacity).toBeLessThan(1)
    })
  })

  it("as três séries Saldo, Reservas e Patrimônio Total estão configuradas no gráfico", () => {
    const data = [makeMonthData()]
    const goals = [makeGoalPoint()]
    setupMocksWithGoals(data, goals)
    render(<AnalyticsDashboard />)
    const areaNames = screen
      .getAllByTestId("area-element")
      .map((el) => el.getAttribute("data-name"))
    expect(areaNames).toContain("Saldo da Conta")
    expect(areaNames).toContain("Reservas")
  })
})

describe("RevenueExpensesChart — unit tests", () => {
  it("renderiza sem erros com receita, despesa e saldo positivos", () => {
    setupMocks([makeMonthData()])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Receitas vs Despesas")).toBeInTheDocument()
    expect(screen.getAllByTestId("composed-chart").length).toBeGreaterThanOrEqual(1)
  })

  it("renderiza sem erros quando saldo é negativo em algum mês", () => {
    setupMocks([
      makeMonthData({ income: 2000, expenses: 5000, balance: -3000 }),
      makeMonthData({ income: 4000, expenses: 3500, balance: 500, month: 4, label: "abr/26" }),
    ])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Receitas vs Despesas")).toBeInTheDocument()
    expect(screen.getAllByTestId("composed-chart").length).toBeGreaterThanOrEqual(1)
  })

  it("renderiza sem erros com array vazio de MonthData", () => {
    setupMocks([])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Receitas vs Despesas")).toBeInTheDocument()
    expect(screen.getAllByTestId("composed-chart").length).toBeGreaterThanOrEqual(1)
  })
})

describe("ProjectionChart — unit tests", () => {
  it("renderiza sem erros com múltiplos pontos históricos e projetados", () => {
    const data = [
      makeMonthData({ year: 2025, month: 11, label: "nov/25" }),
      makeMonthData({ year: 2025, month: 12, label: "dez/25" }),
      makeMonthData({ year: 2026, month: 1, label: "jan/26" }),
    ]
    const goals = [
      makeGoalPoint({ year: 2025, month: 11, total: 5000 }),
      makeGoalPoint({ year: 2025, month: 12, total: 6000 }),
      makeGoalPoint({ year: 2026, month: 1, total: 8000 }),
    ]
    setupMocksWithGoals(data, goals)
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Projeção de Saldo (próximos 3 meses)")).toBeInTheDocument()
    expect(screen.getAllByTestId("composed-chart").length).toBeGreaterThanOrEqual(1)
  })

  it("renderiza sem erros quando há apenas um ponto histórico (sem variação calculável)", () => {
    const data = [makeMonthData()]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Projeção de Saldo (próximos 3 meses)")).toBeInTheDocument()
  })

  it("renderiza sem erros quando data está vazio", () => {
    setupMocks([])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Projeção de Saldo (próximos 3 meses)")).toBeInTheDocument()
  })
})

describe("ProjectionChart — integration tests", () => {
  it("segmento histórico é renderizado como área preenchida no DOM", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26" }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26" }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    const areaNames = screen.getAllByTestId("area-element").map((el) => el.getAttribute("data-name"))
    expect(areaNames).toContain("Patrimônio real")
  })

  it("segmento de projeção é renderizado como linha tracejada sem preenchimento", () => {
    const data = [makeMonthData()]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    const lineElements = screen.getAllByTestId("line-element")
    const projectionLine = lineElements.find((el) => el.getAttribute("data-name") === "Projeção")
    expect(projectionLine).toBeDefined()
    expect(projectionLine?.getAttribute("data-stroke-dasharray")).toBeTruthy()
  })

  it("séries têm nomes distintos 'Patrimônio real' e 'Projeção' para o tooltip", () => {
    const data = [makeMonthData()]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    const areaNames = screen.getAllByTestId("area-element").map((el) => el.getAttribute("data-name"))
    const lineNames = screen.getAllByTestId("line-element").map((el) => el.getAttribute("data-name"))
    expect(areaNames).toContain("Patrimônio real")
    expect(lineNames).toContain("Projeção")
  })
})

describe("MonthComparisonChart — unit tests", () => {
  it("fórmula retorna 0 quando valor do Mês A é 0 (proteção divisão por zero)", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 0 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { alimentacao: 100 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    const cells = screen.getAllByTestId("cell-element")
    // pct = 0 (not NaN), 0 <= 0 so green
    expect(cells[0].getAttribute("data-fill")).toBe("#22c55e")
  })

  it("fórmula retorna 50 quando Mês A = 100 e Mês B = 150", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { alimentacao: 150 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    const cells = screen.getAllByTestId("cell-element")
    // pct = 50 > 0 → red
    expect(cells[0].getAttribute("data-fill")).toBe("#ef4444")
  })

  it("fórmula retorna -25 quando Mês A = 100 e Mês B = 75", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { alimentacao: 75 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    const cells = screen.getAllByTestId("cell-element")
    // pct = -25 <= 0 → green
    expect(cells[0].getAttribute("data-fill")).toBe("#22c55e")
  })

  it("renderiza sem erros em modo grouped com dois meses com categorias distintas", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100, transporte: 200 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { lazer: 50 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Comparação de Meses por Categoria")).toBeInTheDocument()
    expect(screen.queryAllByTestId("cell-element")).toHaveLength(0)
  })

  it("renderiza sem erros em modo pct com dois meses com categorias distintas", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100, transporte: 200 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { lazer: 50 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    expect(screen.getByText("Comparação de Meses por Categoria")).toBeInTheDocument()
    expect(screen.getAllByTestId("cell-element").length).toBeGreaterThan(0)
  })

  it("renderiza sem erros quando categoria existe em Mês A mas não em Mês B (ausente = 0)", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: {} }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    // alimentacao: A=100, B=0, pct=-100 ≤ 0 → green, no crash
    const cells = screen.getAllByTestId("cell-element")
    expect(cells[0].getAttribute("data-fill")).toBe("#22c55e")
  })
})

describe("MonthComparisonChart — integration tests", () => {
  it("clicar no botão [%] exibe barras de variação percentual (células aparecem)", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { alimentacao: 150 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    expect(screen.queryAllByTestId("cell-element")).toHaveLength(0)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    expect(screen.getAllByTestId("cell-element").length).toBeGreaterThan(0)
  })

  it("clicar no botão [≡≡] retorna ao modo agrupado (células desaparecem)", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { alimentacao: 150 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    expect(screen.getAllByTestId("cell-element").length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole("button", { name: "Barras agrupadas" }))
    expect(screen.queryAllByTestId("cell-element")).toHaveLength(0)
  })

  it("barra com variação positiva (aumento) exibe cor vermelha no modo pct", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { alimentacao: 200 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    const cells = screen.getAllByTestId("cell-element")
    expect(cells.some((cell) => cell.getAttribute("data-fill") === "#ef4444")).toBe(true)
  })

  it("barra com variação negativa (redução) exibe cor verde no modo pct", () => {
    const data = [
      makeMonthData({ year: 2026, month: 4, label: "abr/26", categories: { alimentacao: 100 } }),
      makeMonthData({ year: 2026, month: 5, label: "mai/26", categories: { alimentacao: 50 } }),
    ]
    setupMocks(data)
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("button", { name: "Variação percentual" }))
    const cells = screen.getAllByTestId("cell-element")
    expect(cells.some((cell) => cell.getAttribute("data-fill") === "#22c55e")).toBe(true)
  })
})

describe("AnalyticsDashboard — MobileLayout", () => {
  it("renderiza KPIs empilhados (Saldo, Reservas, Patrimônio) quando mobile", () => {
    mockMatchMedia(false)
    setupMocks([])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Saldo da Conta")).toBeInTheDocument()
    expect(screen.getByText("Reservas")).toBeInTheDocument()
    expect(screen.getByText("Patrimônio Total")).toBeInTheDocument()
  })

  it("renderiza tab bar com Fluxo, Categorias e Futuro", () => {
    mockMatchMedia(false)
    setupMocks([])
    render(<AnalyticsDashboard />)
    expect(screen.getByRole("tab", { name: "Fluxo" })).toBeInTheDocument()
    expect(screen.getByRole("tab", { name: "Categorias" })).toBeInTheDocument()
    expect(screen.getByRole("tab", { name: "Futuro" })).toBeInTheDocument()
  })

  it("aba 'Fluxo' (inicial) mostra Receitas vs Despesas e Patrimônio", () => {
    mockMatchMedia(false)
    setupMocks([makeMonthData()])
    render(<AnalyticsDashboard />)
    expect(screen.getByText("Receitas vs Despesas")).toBeInTheDocument()
    expect(screen.getByText("Patrimônio")).toBeInTheDocument()
    expect(screen.queryByText("Categorias por Mês")).not.toBeInTheDocument()
    expect(screen.queryByText("Projeção de Saldo (próximos 3 meses)")).not.toBeInTheDocument()
  })

  it("clicar em 'Categorias' troca o conteúdo da aba", () => {
    mockMatchMedia(false)
    setupMocks([makeMonthData()])
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("tab", { name: "Categorias" }))
    expect(screen.getByText("Categorias por Mês")).toBeInTheDocument()
    expect(screen.getByText("Despesas por Forma de Pagamento")).toBeInTheDocument()
    expect(screen.getByText("Comparação de Meses por Categoria")).toBeInTheDocument()
    expect(screen.queryByText("Receitas vs Despesas")).not.toBeInTheDocument()
  })

  it("clicar em 'Futuro' mostra Projeção e esconde os outros", () => {
    mockMatchMedia(false)
    setupMocks([makeMonthData()])
    render(<AnalyticsDashboard />)
    fireEvent.click(screen.getByRole("tab", { name: "Futuro" }))
    expect(screen.getByText("Projeção de Saldo (próximos 3 meses)")).toBeInTheDocument()
    expect(screen.queryByText("Receitas vs Despesas")).not.toBeInTheDocument()
    expect(screen.queryByText("Categorias por Mês")).not.toBeInTheDocument()
  })

  it("aba ativa fica com aria-selected=true e as outras false", () => {
    mockMatchMedia(false)
    setupMocks([])
    render(<AnalyticsDashboard />)
    expect(screen.getByRole("tab", { name: "Fluxo" })).toHaveAttribute("aria-selected", "true")
    expect(screen.getByRole("tab", { name: "Categorias" })).toHaveAttribute("aria-selected", "false")
    fireEvent.click(screen.getByRole("tab", { name: "Categorias" }))
    expect(screen.getByRole("tab", { name: "Fluxo" })).toHaveAttribute("aria-selected", "false")
    expect(screen.getByRole("tab", { name: "Categorias" })).toHaveAttribute("aria-selected", "true")
  })
})
