import { useState, useRef, useEffect } from "react"
import { Eye, EyeOff } from "lucide-react"
import { useNavigate } from "react-router-dom"
import { categoryLabel, EXPENSE_SLUGS_STANDARD, EXPENSE_SLUGS_FOOD_DETAIL } from "@/lib/categories"
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  AreaChart,
  Area,
  ComposedChart,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAnalytics, useGoalHistory, usePaymentMethodAnalytics, useExpenseProjection, MonthData, GoalMonthlyPoint, PaymentMethodMonthData, ExpenseProjectionMonth } from "@/hooks/useAnalytics"
import { useCurrencyStore, formatCurrency, currencySymbol, maskCurrency } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"
import { useDashboardQuery } from "@/hooks/useDashboard"
import { useReservas } from "@/hooks/useReservas"
import { useMediaQuery } from "@/hooks/useMediaQuery"

const COLORS = ["#6366f1", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4", "#f97316", "#84cc16"]
const COLOR_INCOME = "#22c55e"
const COLOR_EXPENSE = "#f97316"
const COLOR_BALANCE = "#3b82f6"
const COLOR_RESERVAS = "#10b981"
const COLOR_TOTAL = "#8b5cf6"

function useFmt() {
  const { currency } = useCurrencyStore()
  const hidden = usePrivacyStore((s) => s.valuesHidden)
  return (value: number) =>
    hidden ? maskCurrency(value, currency) : formatCurrency(value, currency)
}

// ---------- KpiCards ----------

function KpiCards() {
  const fmt = useFmt()
  const now = new Date()
  const { data: dashData } = useDashboardQuery(now.getFullYear(), now.getMonth() + 1)
  const { goals } = useReservas()

  const saldoMes = dashData?.summary.balance ?? 0
  const totalReservas = goals.reduce((sum, g) => sum + g.current_amount, 0)
  const patrimonio = saldoMes + totalReservas

  const ptMonths = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"]
  const mesLabel = `${ptMonths[now.getMonth()]}/${String(now.getFullYear()).slice(2)}`

  return (
    <div className="grid grid-cols-3 gap-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">Saldo da Conta</CardTitle>
        </CardHeader>
        <CardContent>
          <p className={`text-2xl font-bold ${saldoMes >= 0 ? "text-blue-500" : "text-red-500"}`}>
            {fmt(saldoMes)}
          </p>
          <p className="text-xs text-muted-foreground mt-1">{mesLabel}</p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">Reservas</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-2xl font-bold text-emerald-500">{fmt(totalReservas)}</p>
          <p className="text-xs text-muted-foreground mt-1">{goals.length} {goals.length === 1 ? "objetivo" : "objetivos"}</p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">Patrimônio Total</CardTitle>
        </CardHeader>
        <CardContent>
          <p className={`text-2xl font-bold ${patrimonio >= 0 ? "text-indigo-500" : "text-red-500"}`}>
            {fmt(patrimonio)}
          </p>
          <p className="text-xs text-muted-foreground mt-1">conta + reservas</p>
        </CardContent>
      </Card>
    </div>
  )
}

// ---------- KpiCardsMobile ----------

function KpiCardsMobile() {
  const fmt = useFmt()
  const now = new Date()
  const { data: dashData } = useDashboardQuery(now.getFullYear(), now.getMonth() + 1)
  const { goals } = useReservas()

  const saldoMes = dashData?.summary.balance ?? 0
  const totalReservas = goals.reduce((sum, g) => sum + g.current_amount, 0)
  const patrimonio = saldoMes + totalReservas

  type Row = { label: string; value: number; color: string }
  const rows: Row[] = [
    { label: "Saldo da Conta", value: saldoMes, color: saldoMes >= 0 ? "text-blue-500" : "text-red-500" },
    { label: "Reservas", value: totalReservas, color: "text-emerald-500" },
    { label: "Patrimônio Total", value: patrimonio, color: patrimonio >= 0 ? "text-indigo-500" : "text-red-500" },
  ]

  return (
    <div className="flex flex-col gap-2">
      {rows.map((r) => (
        <Card key={r.label}>
          <CardContent className="flex items-center justify-between py-3 px-4">
            <span className="text-sm text-muted-foreground">{r.label}</span>
            <span className={`text-lg font-bold ${r.color}`}>{fmt(r.value)}</span>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

// ---------- RevenueExpensesChart ----------

function RevenueExpensesChart({ data }: { data: MonthData[] }) {
  const fmt = useFmt()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Receitas vs Despesas</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={260}>
          <ComposedChart data={data} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="label" tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `R$${(v / 1000).toFixed(0)}k`} />
            <Tooltip formatter={(value: number) => fmt(value)} />
            <Legend />
            <Bar dataKey="income" name="Receita" fill={COLOR_INCOME} radius={[3, 3, 0, 0]} />
            <Bar dataKey="expenses" name="Despesa" fill={COLOR_EXPENSE} radius={[3, 3, 0, 0]} />
            <Line type="monotone" dataKey="balance" name="Saldo" stroke={COLOR_BALANCE} strokeWidth={2} dot={false} />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

// ---------- PatrimonioChart ----------

function PatrimonioChart({ data, goalHistory }: { data: MonthData[]; goalHistory: GoalMonthlyPoint[] }) {
  const fmt = useFmt()

  const goalMap = new Map<string, number>()
  for (const p of goalHistory) {
    goalMap.set(`${p.year}-${p.month}`, p.total)
  }

  const chartData = data.map((m) => {
    const reservas = goalMap.get(`${m.year}-${m.month}`) ?? 0
    return {
      label: m.label,
      saldo: m.balance,
      reservas,
      patrimonio: m.balance + reservas,
    }
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Patrimônio</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={260}>
          <ComposedChart data={chartData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id="gradSaldo" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%"  stopColor={COLOR_BALANCE}   stopOpacity={0.5} />
                <stop offset="95%" stopColor={COLOR_BALANCE}   stopOpacity={0.05} />
              </linearGradient>
              <linearGradient id="gradReservasPatrimonio" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%"  stopColor={COLOR_RESERVAS}  stopOpacity={0.5} />
                <stop offset="95%" stopColor={COLOR_RESERVAS}  stopOpacity={0.05} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="label" tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `R$${(v / 1000).toFixed(0)}k`} />
            <Tooltip formatter={(value: number) => fmt(value)} />
            <Legend />
            <Area type="monotone" dataKey="saldo"    name="Saldo da Conta"   stroke={COLOR_BALANCE}   fill="url(#gradSaldo)"               fillOpacity={0.5} strokeWidth={2} dot={false} />
            <Area type="monotone" dataKey="reservas" name="Reservas"          stroke={COLOR_RESERVAS}  fill="url(#gradReservasPatrimonio)"   fillOpacity={0.5} strokeWidth={2} dot={false} />
            <Line type="monotone" dataKey="patrimonio" name="Patrimônio Total" stroke={COLOR_TOTAL} strokeWidth={2} dot={false} />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

// ---------- CategoryPieChart ----------

function CategoryPieChart({ data, compact = false }: { data: MonthData[]; compact?: boolean }) {
  const fmt = useFmt()
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null)

  useEffect(() => {
    setSelectedIdx(null)
  }, [data.length])

  const idx = selectedIdx !== null ? selectedIdx : data.length - 1
  const month = data[idx]
  const expenseSlugs = new Set([...EXPENSE_SLUGS_STANDARD, ...EXPENSE_SLUGS_FOOD_DETAIL])
  const pieData = month
    ? (() => {
        const items = Object.entries(month.categories)
          .filter(([slug, v]) => v > 0 && expenseSlugs.has(slug))
          .map(([slug, value]) => ({ name: categoryLabel(slug), value }))
          .sort((a, b) => b.value - a.value)
        const top5 = items.slice(0, 5)
        const rest = items.slice(5)
        if (rest.length > 0) {
          top5.push({ name: "Outros", value: rest.reduce((s, x) => s + x.value, 0) })
        }
        return top5
      })()
    : []

  return (
    <Card>
      <CardHeader className={compact ? "pb-2 space-y-2" : "flex flex-row items-center justify-between pb-2"}>
        <CardTitle className="text-base">Categorias por Mês</CardTitle>
        <select
          value={idx}
          onChange={(e) => setSelectedIdx(Number(e.target.value))}
          className={`text-xs border rounded px-2 py-1 bg-background ${compact ? "self-start" : ""}`}
        >
          {data.map((m, i) => (
            <option key={i} value={i}>{m.label}</option>
          ))}
        </select>
      </CardHeader>
      <CardContent>
        {pieData.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-8">Sem dados para este mês.</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <PieChart>
              <Pie
                key={month.label}
                data={pieData}
                dataKey="value"
                nameKey="name"
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={90}
                isAnimationActive={true}
                animationBegin={0}
              >
                {pieData.map((_, i) => (
                  <Cell key={i} fill={COLORS[i % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip formatter={(value: number) => fmt(value)} />
              <Legend />
            </PieChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}

// ---------- PaymentMethodPieChart ----------

function PaymentMethodPieChart({ data, compact = false }: { data: PaymentMethodMonthData[]; compact?: boolean }) {
  const fmt = useFmt()
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null)

  useEffect(() => {
    setSelectedIdx(null)
  }, [data.length])

  const idx = selectedIdx !== null ? selectedIdx : data.length - 1
  const month = data[idx]

  const pieData = month
    ? month.methods
        .filter((m) => m.total < 0)
        .map((m) => ({ name: m.name, value: Math.abs(m.total), color: m.color }))
    : []

  return (
    <Card>
      <CardHeader className={compact ? "pb-2 space-y-2" : "flex flex-row items-center justify-between pb-2"}>
        <CardTitle className="text-base">Despesas por Forma de Pagamento</CardTitle>
        <select
          value={idx}
          onChange={(e) => setSelectedIdx(Number(e.target.value))}
          className={`text-xs border rounded px-2 py-1 bg-background ${compact ? "self-start" : ""}`}
        >
          {data.map((m, i) => (
            <option key={i} value={i}>{m.label}</option>
          ))}
        </select>
      </CardHeader>
      <CardContent>
        {pieData.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-8">Sem despesas para este mês.</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <PieChart>
              <Pie
                key={month?.label}
                data={pieData}
                dataKey="value"
                nameKey="name"
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={90}
              >
                {pieData.map((entry, i) => (
                  <Cell key={i} fill={entry.color && entry.color !== "" ? entry.color : COLORS[i % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip formatter={(value: number) => fmt(value)} />
              <Legend />
            </PieChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}

// ---------- ProjectionChart ----------

function ProjectionChart({
  data,
  fullData,
  fullGoalHistory,
}: {
  data: MonthData[]
  fullData: MonthData[]
  fullGoalHistory: GoalMonthlyPoint[]
}) {
  const fmt = useFmt()

  const goalMap = new Map<string, number>()
  for (const p of fullGoalHistory) {
    goalMap.set(`${p.year}-${p.month}`, p.total)
  }

  const patrimonioData = fullData.map((m) => ({
    label: m.label,
    year: m.year,
    month: m.month,
    patrimonio: m.balance + (goalMap.get(`${m.year}-${m.month}`) ?? 0),
  }))

  // Exclui o mês corrente (parcial) do cálculo — usa os 3 meses completos anteriores
  const completeMonths = patrimonioData.slice(0, -1)
  const lastThree = completeMonths.slice(-3)
  // Variação mensal do patrimônio total nos últimos 3 meses completos
  const avgMonthlyChange =
    lastThree.length >= 2
      ? (lastThree[lastThree.length - 1].patrimonio - lastThree[0].patrimonio) / (lastThree.length - 1)
      : 0

  const ptMonths = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"]
  const now = new Date()
  const lastPatrimonio = patrimonioData.length > 0 ? patrimonioData[patrimonioData.length - 1].patrimonio : 0
  const lastLabel = patrimonioData.length > 0 ? patrimonioData[patrimonioData.length - 1].label : ""

  const displayLabels = new Set(data.map((m) => m.label))

  const projected: { label: string; patrimonio?: number; projected?: number }[] = patrimonioData
    .filter((m) => displayLabels.has(m.label))
    .map((m) => ({ label: m.label, patrimonio: m.patrimonio }))

  // Ponte: último ponto real aparece também na linha de projeção
  if (patrimonioData.length > 0) {
    projected[projected.length - 1] = { label: lastLabel, patrimonio: lastPatrimonio, projected: lastPatrimonio }
  }

  for (let i = 1; i <= 3; i++) {
    const d = new Date(now.getFullYear(), now.getMonth() + i, 1)
    projected.push({
      label: `${ptMonths[d.getMonth()]}/${String(d.getFullYear()).slice(2)}`,
      projected: lastPatrimonio + avgMonthlyChange * i,
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Projeção de Saldo (próximos 3 meses)</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={260}>
          <ComposedChart data={projected} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id="gradProjecaoReal" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor={COLOR_BALANCE} stopOpacity={0.5} />
                <stop offset="95%" stopColor={COLOR_BALANCE} stopOpacity={0.05} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="label" tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `R$${(v / 1000).toFixed(0)}k`} />
            <Tooltip formatter={(value: number) => fmt(value)} />
            <Legend />
            <Area
              type="monotone"
              dataKey="patrimonio"
              name="Patrimônio real"
              stroke={COLOR_BALANCE}
              strokeWidth={2}
              fill="url(#gradProjecaoReal)"
              fillOpacity={0.5}
              dot={false}
              connectNulls={false}
            />
            <Line
              type="monotone"
              dataKey="projected"
              name="Projeção"
              stroke={COLOR_BALANCE}
              strokeWidth={2}
              strokeDasharray="6 3"
              dot={false}
              connectNulls={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

// ---------- ExpenseProjectionChart ----------

const COLOR_INSTALLMENTS = "#8b5cf6"
const COLOR_CARD = "#06b6d4"

function ExpenseProjectionChart({ data }: { data: ExpenseProjectionMonth[] }) {
  const fmt = useFmt()
  const navigate = useNavigate()

  if (data.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Projeção de Gastos</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Sem gastos fixos, parcelas ou assinaturas de cartão para projetar.
          </p>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Projeção de Gastos ({data.length} {data.length === 1 ? "mês" : "meses"})</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={260}>
          <BarChart data={data} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="label" tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `R$${(v / 1000).toFixed(0)}k`} />
            <Tooltip formatter={(value: number) => fmt(value)} />
            <Legend />
            <Bar dataKey="fixed" stackId="g" name="Fixos" fill={COLOR_EXPENSE} />
            <Bar dataKey="installments" stackId="g" name="Parcelas" fill={COLOR_INSTALLMENTS} />
            <Bar dataKey="card" stackId="g" name="Cartão" fill={COLOR_CARD} radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
      <div className="px-6 pb-4">
        <button
          onClick={() => navigate("/projecao")}
          className="text-sm text-primary hover:underline"
        >
          Ver detalhes →
        </button>
      </div>
    </Card>
  )
}

// ---------- MonthComparisonChart ----------

function MonthComparisonChart({ data, compact = false }: { data: MonthData[]; compact?: boolean }) {
  const fmt = useFmt()
  const { currency } = useCurrencyStore()
  const [idxA, setIdxA] = useState(Math.max(0, data.length - 2))
  const [idxB, setIdxB] = useState(data.length - 1)
  const [compMode, setCompMode] = useState<"grouped" | "pct">("grouped")

  useEffect(() => {
    setIdxA(Math.max(0, data.length - 2))
    setIdxB(data.length - 1)
  }, [data.length])

  const expenseSlugs = new Set([...EXPENSE_SLUGS_STANDARD, ...EXPENSE_SLUGS_FOOD_DETAIL])
  const allCategories = Array.from(
    new Set(data.flatMap((m) => Object.keys(m.categories).filter((k) => expenseSlugs.has(k))))
  ).sort()

  const [selected, setSelected] = useState<Set<string>>(new Set(allCategories.slice(0, 3)))

  useEffect(() => {
    const cats = Array.from(new Set(data.flatMap((m) => Object.keys(m.categories)).filter((k) => expenseSlugs.has(k)))).sort()
    setSelected(new Set(cats.slice(0, 3)))
  }, [data])
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [])

  const monthA = data[idxA]
  const monthB = data[idxB]

  const chartData = Array.from(selected).map((cat) => ({
    category: categoryLabel(cat),
    [monthA?.label ?? "A"]: monthA?.categories[cat] ?? 0,
    [monthB?.label ?? "B"]: monthB?.categories[cat] ?? 0,
  }))

  const pctData = Array.from(selected).map((cat) => {
    const a = monthA?.categories[cat] ?? 0
    const b = monthB?.categories[cat] ?? 0
    const diff = b - a
    return { category: categoryLabel(cat), diff }
  })

  const unselected = allCategories.filter((c) => !selected.has(c))

  function addCategory(cat: string) {
    setSelected((prev) => new Set([...prev, cat]))
    setDropdownOpen(false)
  }

  function removeCategory(cat: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      next.delete(cat)
      return next
    })
  }

  return (
    <Card className="col-span-2">
      <CardHeader className={compact
          ? "flex flex-col items-stretch gap-3 pb-2"
          : "flex flex-row items-start justify-between pb-2 flex-wrap gap-3"}>
        <CardTitle className="text-base">Comparação de Meses por Categoria</CardTitle>
        <div className={compact ? "flex flex-col gap-2" : "flex gap-3 items-center flex-wrap"}>
          <div className="flex items-center gap-2">
            <label className="text-xs text-muted-foreground">Mês A:</label>
            <select
              value={idxA}
              onChange={(e) => setIdxA(Number(e.target.value))}
              className="text-xs border rounded px-2 py-1 bg-background"
            >
              {data.map((m, i) => (
                <option key={i} value={i}>{m.label}</option>
              ))}
            </select>
          </div>
          <div className="flex items-center gap-2">
            <label className="text-xs text-muted-foreground">Mês B:</label>
            <select
              value={idxB}
              onChange={(e) => setIdxB(Number(e.target.value))}
              className="text-xs border rounded px-2 py-1 bg-background"
            >
              {data.map((m, i) => (
                <option key={i} value={i}>{m.label}</option>
              ))}
            </select>
          </div>
          <div className="flex gap-1">
            <button
              onClick={() => setCompMode("grouped")}
              aria-label="Barras agrupadas"
              className={`px-2 py-1 text-xs rounded border transition-colors ${
                compMode === "grouped"
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-background text-muted-foreground border-border hover:text-foreground"
              }`}
            >
              ≡≡
            </button>
            <button
              onClick={() => setCompMode("pct")}
              aria-label="Variação em valor"
              className={`px-2 py-1 text-xs rounded border transition-colors ${
                compMode === "pct"
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-background text-muted-foreground border-border hover:text-foreground"
              }`}
            >
              Δ
            </button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {/* Tags das categorias selecionadas + dropdown para adicionar */}
        <div className="flex flex-wrap gap-2 mb-4 items-center">
          {Array.from(selected).map((cat) => (
            <span
              key={cat}
              className="inline-flex items-center gap-1 text-xs bg-secondary text-secondary-foreground px-2 py-1 rounded-full"
            >
              {categoryLabel(cat)}
              <button
                onClick={() => removeCategory(cat)}
                className="hover:text-destructive transition-colors leading-none"
                aria-label={`Remover ${categoryLabel(cat)}`}
              >
                ×
              </button>
            </span>
          ))}
          {!compact && unselected.length > 0 && (
            <div className="relative" ref={dropdownRef}>
              <button
                onClick={() => setDropdownOpen((v) => !v)}
                className="text-xs border border-dashed border-border rounded-full px-2 py-1 text-muted-foreground hover:text-foreground hover:border-foreground transition-colors"
              >
                + categoria
              </button>
              {dropdownOpen && (
                <div className="absolute left-0 top-full mt-1 z-10 bg-popover border border-border rounded-md shadow-md min-w-[140px]">
                  {unselected.map((cat) => (
                    <button
                      key={cat}
                      onClick={() => addCategory(cat)}
                      className="w-full text-left text-xs px-3 py-1.5 hover:bg-accent hover:text-accent-foreground transition-colors"
                    >
                      {categoryLabel(cat)}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
        {idxA === idxB && (
          <p className="text-xs text-yellow-600 mb-2">Selecione meses diferentes para comparar.</p>
        )}
        {selected.size === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-8">Selecione ao menos uma categoria.</p>
        ) : (
          <ResponsiveContainer width="100%" height={280}>
            {compMode === "grouped" ? (
              <BarChart data={chartData} margin={{ top: 4, right: 8, left: 0, bottom: 50 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="category" tick={{ fontSize: 10 }} interval={0} />
                <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `R$${(v / 1000).toFixed(0)}k`} />
                <Tooltip formatter={(value: number) => fmt(value)} />
                <Legend />
                <Bar dataKey={monthA?.label ?? "A"} fill="#3b82f6" radius={[3, 3, 0, 0]} />
                <Bar dataKey={monthB?.label ?? "B"} fill="#8b5cf6" radius={[3, 3, 0, 0]} />
              </BarChart>
            ) : (
              <BarChart data={pctData} margin={{ top: 4, right: 8, left: 8, bottom: 50 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="category" tick={{ fontSize: 10 }} interval={0} />
                <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `${currencySymbol(currency)}${(v / 1000).toFixed(0)}k`} />
                <Tooltip formatter={(value: number) => fmt(value)} />
                <Bar dataKey="diff" radius={[3, 3, 0, 0]}>
                  {pctData.map((_entry, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Bar>
              </BarChart>
            )}
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  )
}

// ---------- Main Dashboard ----------

type TimeFilter = 1 | 3 | 6 | 12

function PainelHeader({
  months,
  setMonths,
}: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
}) {
  const valuesHidden = usePrivacyStore((s) => s.valuesHidden)
  const toggleValuesHidden = usePrivacyStore((s) => s.toggleValuesHidden)

  return (
    <div className="border-b border-border px-4 py-3 lg:px-6 lg:py-4 flex items-center justify-between gap-2 flex-wrap">
      <div>
        <h1 className="hidden lg:block text-xl lg:text-2xl font-bold">Painel</h1>
        <p className="text-xs lg:text-sm text-muted-foreground mt-0.5">
          Últimos {months} {months === 1 ? "mês" : "meses"}
        </p>
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={toggleValuesHidden}
          aria-label={valuesHidden ? "Mostrar valores" : "Ocultar valores"}
          aria-pressed={valuesHidden}
          className="hidden lg:flex w-8 h-8 items-center justify-center rounded-lg hover:bg-secondary transition-colors shrink-0"
        >
          {valuesHidden ? (
            <EyeOff className="w-5 h-5 text-muted-foreground" />
          ) : (
            <Eye className="w-5 h-5 text-muted-foreground" />
          )}
        </button>
        <div className="flex gap-1">
          {([1, 3, 6, 12] as TimeFilter[]).map((m) => (
            <button
              key={m}
              onClick={() => setMonths(m)}
              className={`px-2 py-1 lg:px-3 lg:py-1.5 text-xs lg:text-sm rounded-md border transition-colors ${
                months === m
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-background text-muted-foreground border-border hover:text-foreground"
              }`}
            >
              {m}m
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}

type PainelTab = "fluxo" | "categorias" | "futuro"

function MobileLayout({
  months,
  setMonths,
  data,
  fullData,
  goalHistory,
  fullGoalHistory,
  paymentData,
  projection,
}: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
  data: MonthData[]
  fullData: MonthData[]
  goalHistory: GoalMonthlyPoint[]
  fullGoalHistory: GoalMonthlyPoint[]
  paymentData: PaymentMethodMonthData[]
  projection: ExpenseProjectionMonth[]
}) {
  const [tab, setTab] = useState<PainelTab>("fluxo")

  const tabs: { id: PainelTab; label: string }[] = [
    { id: "fluxo", label: "Fluxo" },
    { id: "categorias", label: "Categorias" },
    { id: "futuro", label: "Futuro" },
  ]

  return (
    <div className="min-h-screen bg-background">
      <PainelHeader months={months} setMonths={setMonths} />
      <div className="p-4">
        <KpiCardsMobile />
      </div>
      <div className="sticky top-0 z-10 bg-background border-b border-border px-4 py-2">
        <div role="tablist" className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t.id}
              role="tab"
              aria-selected={tab === t.id}
              onClick={() => setTab(t.id)}
              className={`flex-1 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                tab === t.id
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>
      <div className="p-4 space-y-4">
        {tab === "fluxo" && (
          <>
            <RevenueExpensesChart data={data} />
            <PatrimonioChart data={data} goalHistory={goalHistory} />
          </>
        )}
        {tab === "categorias" && (
          <>
            <CategoryPieChart data={data} compact />
            <PaymentMethodPieChart data={paymentData} compact />
            <MonthComparisonChart data={data} compact />
          </>
        )}
        {tab === "futuro" && (
          <>
            <ProjectionChart data={data} fullData={fullData} fullGoalHistory={fullGoalHistory} />
            <ExpenseProjectionChart data={projection} />
          </>
        )}
      </div>
    </div>
  )
}

function DesktopLayout({
  months,
  setMonths,
  data,
  fullData,
  goalHistory,
  fullGoalHistory,
  paymentData,
  projection,
}: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
  data: MonthData[]
  fullData: MonthData[]
  goalHistory: GoalMonthlyPoint[]
  fullGoalHistory: GoalMonthlyPoint[]
  paymentData: PaymentMethodMonthData[]
  projection: ExpenseProjectionMonth[]
}) {
  return (
    <div className="min-h-screen bg-background">
      <PainelHeader months={months} setMonths={setMonths} />

      <div className="p-6 space-y-6">
        <KpiCards />

        <div className="grid grid-cols-2 gap-6">
          <RevenueExpensesChart data={data} />
          <PatrimonioChart data={data} goalHistory={goalHistory} />
          <CategoryPieChart data={data} />
          <PaymentMethodPieChart data={paymentData} />
          <MonthComparisonChart data={data} />
          <ProjectionChart data={data} fullData={fullData} fullGoalHistory={fullGoalHistory} />
          <ExpenseProjectionChart data={projection} />
        </div>
      </div>
    </div>
  )
}

export function AnalyticsDashboard() {
  const [months, setMonths] = useState<TimeFilter>(3)
  const { data, loading, error } = useAnalytics(months)
  const { data: fullData } = useAnalytics(12)
  const { data: goalHistory } = useGoalHistory(months)
  const { data: fullGoalHistory } = useGoalHistory(12)
  const { data: paymentData } = usePaymentMethodAnalytics(months)
  const { data: projection } = useExpenseProjection(months)
  const isDesktop = useMediaQuery("(min-width: 1024px)")

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full min-h-screen">
        <p className="text-muted-foreground">Carregando dados analíticos...</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full min-h-screen">
        <p className="text-muted-foreground">{error}</p>
      </div>
    )
  }

  if (!isDesktop) {
    return <MobileLayout
      months={months}
      setMonths={setMonths}
      data={data}
      fullData={fullData}
      goalHistory={goalHistory}
      fullGoalHistory={fullGoalHistory}
      paymentData={paymentData}
      projection={projection}
    />
  }

  return <DesktopLayout
    months={months}
    setMonths={setMonths}
    data={data}
    fullData={fullData}
    goalHistory={goalHistory}
    fullGoalHistory={fullGoalHistory}
    paymentData={paymentData}
    projection={projection}
  />
}
