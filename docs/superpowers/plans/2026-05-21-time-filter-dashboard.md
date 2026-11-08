# Time Filter Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adicionar filtro global de tempo (1/3/6/12 meses) no `AnalyticsDashboard` que controla o período exibido em todos os charts.

**Architecture:** Estado local `months: 1|3|6|12` em `AnalyticsDashboard` com `useState(12)`. `useAnalytics` aceita `months` como parâmetro. O pai chama `useAnalytics(months)` para dados filtrados e `useAnalytics(12)` para `ProjectionChart`. Hooks internos de `PatrimonioChart`, `ProjectionChart` e `PaymentMethodPieChart` são movidos para o pai e passados como props.

**Tech Stack:** React, TypeScript, Vitest, @testing-library/react

---

## Arquivos Afetados

- Modify: `frontend/src/hooks/useAnalytics.ts` — `useAnalytics` aceita `months`
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx` — filtro UI, dois calls de `useAnalytics`, prop drilling
- Test: `frontend/src/hooks/useAnalytics.test.ts` — novo arquivo de teste

---

### Task 1: Parametrizar `useAnalytics` com `months`

**Files:**
- Modify: `frontend/src/hooks/useAnalytics.ts`
- Test: `frontend/src/hooks/useAnalytics.test.ts`

- [ ] **Step 1: Criar arquivo de teste e escrever teste falhando**

Criar `frontend/src/hooks/useAnalytics.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from "vitest"
import { renderHook, waitFor } from "@testing-library/react"
import { useAnalytics } from "@/hooks/useAnalytics"
import { api } from "@/services/api"

vi.mock("@/services/api", () => ({
  api: { get: vi.fn() },
}))

const makeSummary = (year: number, month: number) => ({
  income: 1000,
  expenses: -500,
  balance: 500,
  categories: { alimentacao: -200 },
  month: `${year}-${String(month).padStart(2, "0")}`,
})

beforeEach(() => vi.clearAllMocks())

describe("useAnalytics", () => {
  it("retorna 12 meses por padrão", async () => {
    vi.mocked(api.get).mockImplementation((url: string) => {
      const m = url.match(/year=(\d+)&month=(\d+)/)
      if (!m) return Promise.reject(new Error("bad url"))
      return Promise.resolve(makeSummary(Number(m[1]), Number(m[2])))
    })
    const { result } = renderHook(() => useAnalytics())
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toHaveLength(12)
  })

  it("retorna apenas N meses quando months=3", async () => {
    vi.mocked(api.get).mockImplementation((url: string) => {
      const m = url.match(/year=(\d+)&month=(\d+)/)
      if (!m) return Promise.reject(new Error("bad url"))
      return Promise.resolve(makeSummary(Number(m[1]), Number(m[2])))
    })
    const { result } = renderHook(() => useAnalytics(3))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toHaveLength(3)
  })

  it("re-executa quando months muda", async () => {
    vi.mocked(api.get).mockImplementation((url: string) => {
      const m = url.match(/year=(\d+)&month=(\d+)/)
      if (!m) return Promise.reject(new Error("bad url"))
      return Promise.resolve(makeSummary(Number(m[1]), Number(m[2])))
    })
    const { result, rerender } = renderHook(
      ({ months }: { months: number }) => useAnalytics(months),
      { initialProps: { months: 12 } }
    )
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toHaveLength(12)

    rerender({ months: 6 })
    await waitFor(() => expect(result.current.data).toHaveLength(6))
  })
})
```

- [ ] **Step 2: Rodar teste para confirmar que falha**

```bash
cd frontend && npx vitest run src/hooks/useAnalytics.test.ts
```

Esperado: FAIL — "useAnalytics is not a function" ou erro de tipo porque o parâmetro não existe.

- [ ] **Step 3: Modificar `useAnalytics` para aceitar `months`**

Em `frontend/src/hooks/useAnalytics.ts`, alterar a assinatura e o loop:

```typescript
export function useAnalytics(months = 12) {
  const [data, setData] = useState<MonthData[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const now = new Date()
    const monthsList: { year: number; month: number }[] = []
    for (let i = months - 1; i >= 0; i--) {
      const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
      monthsList.push({ year: d.getFullYear(), month: d.getMonth() + 1 })
    }

    Promise.all(
      monthsList.map(({ year, month }) =>
        api.get<DashboardSummary>(`/transactions/summary?year=${year}&month=${month}`)
          .then((s) => ({ year, month, summary: s }))
          .catch(() => ({ year, month, summary: null as DashboardSummary | null }))
      )
    ).then((results) => {
      const allFailed = results.every((r) => r.summary === null)
      if (allFailed) {
        setError("Não foi possível carregar os dados.")
        setLoading(false)
        return
      }
      const ptMonths = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"]
      const monthData = results.map(({ year, month, summary }) => ({
        year,
        month,
        label: `${ptMonths[month - 1]}/${String(year).slice(2)}`,
        income: summary?.income ?? 0,
        expenses: Math.abs(summary?.expenses ?? 0),
        balance: summary?.balance ?? 0,
        categories: summary
          ? Object.fromEntries(Object.entries(summary.categories).map(([k, v]) => [k, Math.abs(v)]))
          : {},
      }))
      setData(monthData)
      setLoading(false)
    })
  }, [months])

  return { data, loading, error }
}
```

- [ ] **Step 4: Rodar teste para confirmar que passa**

```bash
cd frontend && npx vitest run src/hooks/useAnalytics.test.ts
```

Esperado: PASS (3 testes).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/hooks/useAnalytics.ts frontend/src/hooks/useAnalytics.test.ts
git commit -m "feat(analytics): useAnalytics aceita parâmetro months"
```

---

### Task 2: Mover hooks internos para o pai e atualizar assinaturas dos componentes

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

Os componentes `PatrimonioChart`, `ProjectionChart` e `PaymentMethodPieChart` chamam hooks internamente. Nesta task, eles passam a receber os dados como props.

- [ ] **Step 1: Atualizar `PatrimonioChart` para receber `goalHistory` como prop**

Substituir a assinatura e remover o hook interno:

```typescript
// ANTES:
function PatrimonioChart({ data }: { data: MonthData[] }) {
  const fmt = useFmt()
  const { data: goalHistory } = useGoalHistory(12)
  // ...

// DEPOIS:
function PatrimonioChart({ data, goalHistory }: { data: MonthData[]; goalHistory: GoalMonthlyPoint[] }) {
  const fmt = useFmt()
  // remover a linha do useGoalHistory
```

- [ ] **Step 2: Atualizar `ProjectionChart` para receber `fullData` e `fullGoalHistory` como props**

```typescript
// ANTES:
function ProjectionChart({ data }: { data: MonthData[] }) {
  const fmt = useFmt()
  const { data: goalHistory } = useGoalHistory(12)

  const goalMap = new Map<string, number>()
  for (const p of goalHistory) {
    goalMap.set(`${p.year}-${p.month}`, p.total)
  }

  const patrimonioData = data.map((m) => ({
// ...

// DEPOIS:
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
```

O restante do cálculo de `avgMonthlyChange` usa `patrimonioData` que agora é baseado em `fullData`. A parte de exibição da linha real deve usar apenas os pontos cujo `label` existe em `data`:

```typescript
  const displayLabels = new Set(data.map((m) => m.label))

  const projected: { label: string; patrimonio?: number; projected?: number }[] = patrimonioData
    .filter((m) => displayLabels.has(m.label))
    .map((m) => ({ label: m.label, patrimonio: m.patrimonio }))
```

O restante do `ProjectionChart` (cálculo de `avgMonthlyChange`, ponte, loop de projeção futura) permanece igual — opera sobre `patrimonioData` (12 meses para cálculo) e `projected` (meses do filtro para exibição).

- [ ] **Step 3: Atualizar `PaymentMethodPieChart` para receber `data` como prop**

```typescript
// ANTES:
function PaymentMethodPieChart() {
  const fmt = useFmt()
  const { data } = usePaymentMethodAnalytics(12)
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null)

// DEPOIS:
function PaymentMethodPieChart({ data }: { data: PaymentMethodMonthData[] }) {
  const fmt = useFmt()
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null)
```

- [ ] **Step 4: Verificar que o TypeScript compila sem erros**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros de tipo. Se houver erros de import (`GoalMonthlyPoint`, `PaymentMethodMonthData` não importados), adicionar aos imports existentes no topo do arquivo:

```typescript
import { useAnalytics, useGoalHistory, usePaymentMethodAnalytics, MonthData, GoalMonthlyPoint, PaymentMethodMonthData } from "@/hooks/useAnalytics"
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "refactor(analytics): componentes recebem dados como props em vez de chamar hooks internamente"
```

---

### Task 3: Adicionar filtro global e conectar tudo no `AnalyticsDashboard`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

- [ ] **Step 1: Adicionar estado e dois calls de `useAnalytics` no componente pai**

Localizar a função `AnalyticsDashboard` (linha ~509) e substituir:

```typescript
// ANTES:
export function AnalyticsDashboard() {
  const { data, loading, error } = useAnalytics()

// DEPOIS:
type TimeFilter = 1 | 3 | 6 | 12

export function AnalyticsDashboard() {
  const [months, setMonths] = useState<TimeFilter>(12)
  const { data, loading, error } = useAnalytics(months)
  const { data: fullData } = useAnalytics(12)
  const { data: goalHistory } = useGoalHistory(months)
  const { data: fullGoalHistory } = useGoalHistory(12)
  const { data: paymentData } = usePaymentMethodAnalytics(months)
```

- [ ] **Step 2: Adicionar UI do filtro no header**

Localizar o bloco do header (ao redor da linha com `<h1 className="text-2xl font-bold">`) e substituir:

```typescript
// ANTES:
      <div className="border-b border-border px-6 py-4">
        <h1 className="text-2xl font-bold">Dashboard Analítico</h1>
        <p className="text-sm text-muted-foreground mt-0.5">Visão completa dos últimos 12 meses</p>
      </div>

// DEPOIS:
      <div className="border-b border-border px-6 py-4 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Dashboard Analítico</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Últimos {months} {months === 1 ? "mês" : "meses"}
          </p>
        </div>
        <div className="flex gap-1">
          {([1, 3, 6, 12] as TimeFilter[]).map((m) => (
            <button
              key={m}
              onClick={() => setMonths(m)}
              className={`px-3 py-1.5 text-sm rounded-md border transition-colors ${
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
```

- [ ] **Step 3: Passar props corretos para cada chart no JSX**

Localizar o bloco `<div className="grid grid-cols-2 gap-6">` e substituir os calls dos componentes:

```typescript
        <div className="grid grid-cols-2 gap-6">
          <RevenueExpensesChart data={data} />
          <PatrimonioChart data={data} goalHistory={goalHistory} />
          <CategoryPieChart data={data} />
          <PaymentMethodPieChart data={paymentData} />
          <MonthComparisonChart data={data} />
          <ProjectionChart data={data} fullData={fullData} fullGoalHistory={fullGoalHistory} />
        </div>
```

- [ ] **Step 4: Verificar que o TypeScript compila sem erros**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 5: Rodar os testes existentes para garantir nenhuma regressão**

```bash
cd frontend && npx vitest run
```

Esperado: todos passam.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "feat(analytics): filtro de tempo global no dashboard (1/3/6/12 meses)"
```

---

### Task 4: Testar manualmente no browser

- [ ] **Step 1: Subir o frontend**

```bash
cd frontend && npm run dev
```

- [ ] **Step 2: Navegar para `/full-dashboard` e verificar cada filtro**

Checklist manual:
- [ ] Padrão carrega com **12m** selecionado e todos os charts exibem 12 meses
- [ ] Clicar **6m**: `RevenueExpensesChart`, `PatrimonioChart`, `MonthComparisonChart` encolhem para 6 barras/pontos
- [ ] Clicar **6m**: dropdowns de `CategoryPieChart` e `PaymentMethodPieChart` mostram apenas 6 meses
- [ ] Clicar **3m**: `ProjectionChart` exibe apenas 3 meses reais na linha contínua, mas a projeção futura (linha tracejada) ainda usa variação calculada de 12 meses históricos
- [ ] Clicar **1m**: todos os charts de série exibem 1 único mês; dropdowns mostram 1 opção; `ProjectionChart` exibe 1 ponto real + 3 projetados
- [ ] Botão ativo tem fundo sólido (`bg-primary`), os outros têm borda apenas
- [ ] Subtítulo muda: "Últimos 1 mês", "Últimos 3 meses", etc.

- [ ] **Step 3: Commit final se tudo ok**

```bash
git add .
git commit -m "chore: verificação manual do filtro de tempo no dashboard"
```

(Se não houver arquivos novos para commitar, pular este step.)
