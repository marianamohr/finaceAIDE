# Painel Mobile — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Renomear `/full-dashboard` para `/painel`, dar entrada permanente no BottomNav, e criar layout mobile baseado em 3 abas (Fluxo / Categorias / Futuro) preservando o desktop atual sem regressão.

**Architecture:** `AnalyticsDashboard` continua sendo o ponto único que faz todas as queries; usa `useMediaQuery("(min-width: 1024px)")` para escolher entre `<DesktopLayout />` (markup atual extraído) e `<MobileLayout />` (KPIs empilhados + tab bar + conteúdo da aba ativa). Os 6 componentes de gráfico ganham prop opcional `compact?: boolean` para adaptarem seus headers em telas estreitas.

**Tech Stack:** React, TypeScript, Tailwind CSS, recharts, vitest + testing-library.

**Spec:** [docs/superpowers/specs/2026-05-26-painel-mobile-design.md](docs/superpowers/specs/2026-05-26-painel-mobile-design.md)

---

## Mapa de arquivos

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `frontend/src/pages/FullDashboard.tsx` | Renomear → `Painel.tsx` | Wrapper fino que renderiza `<AnalyticsDashboard />` |
| `frontend/src/App.tsx` | Modificar | Trocar rota `/full-dashboard` por `/painel`; `/insights` redireciona pra `/painel` |
| `frontend/src/components/BottomNav/BottomNav.tsx` | Modificar | Entrada `Painel` fixa em qualquer resolução; remove filtro `isDesktop` |
| `frontend/src/components/AppShell/AppShell.tsx` | Modificar | Atualizar `ALL_NAV_ITEMS`: path `/painel`, label `Painel`, key `painel` |
| `frontend/src/components/analytics/AnalyticsDashboard.tsx` | Modificar | Adicionar `useMediaQuery`, extrair `DesktopLayout`, criar `MobileLayout`, `PainelHeader`, `KpiCardsMobile`; adicionar prop `compact?: boolean` em `CategoryPieChart`, `PaymentMethodPieChart`, `MonthComparisonChart` |
| `frontend/src/components/analytics/AnalyticsDashboard.test.tsx` | Modificar | Mockar `useMediaQuery` retornando `true` (desktop) nos testes existentes; adicionar testes pra `MobileLayout` |

---

## Task 1: Renomear página e atualizar rotas

**Files:**
- Rename: `frontend/src/pages/FullDashboard.tsx` → `frontend/src/pages/Painel.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Renomear o arquivo da página e o componente exportado**

Mover o arquivo:

```bash
git mv frontend/src/pages/FullDashboard.tsx frontend/src/pages/Painel.tsx
```

Editar `frontend/src/pages/Painel.tsx` substituindo o conteúdo por:

```tsx
import { AnalyticsDashboard } from "@/components/analytics/AnalyticsDashboard"

export default function Painel() {
  return (
    <div className="h-full overflow-y-auto">
      <AnalyticsDashboard />
    </div>
  )
}
```

- [ ] **Step 2: Atualizar `App.tsx` — import e rotas**

Em `frontend/src/App.tsx`:

Linha 23, trocar:
```diff
- import FullDashboard from "./pages/FullDashboard";
+ import Painel from "./pages/Painel";
```

Linhas 85-86, trocar:
```diff
-          <Route path="/insights" element={<Navigate to="/full-dashboard" replace />} />
-          <Route path="/full-dashboard" element={<ProtectedRoute><AppShell><FullDashboard /></AppShell></ProtectedRoute>} />
+          <Route path="/insights" element={<Navigate to="/painel" replace />} />
+          <Route path="/painel" element={<ProtectedRoute><AppShell><Painel /></AppShell></ProtectedRoute>} />
```

- [ ] **Step 3: Verificar build do frontend**

```bash
cd frontend && npm run build
```

Expected: build conclui sem erros de TypeScript.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Painel.tsx frontend/src/pages/FullDashboard.tsx frontend/src/App.tsx
git commit -m "refactor: renomear /full-dashboard para /painel"
```

---

## Task 2: Atualizar BottomNav e AppShell

**Files:**
- Modify: `frontend/src/components/BottomNav/BottomNav.tsx`
- Modify: `frontend/src/components/AppShell/AppShell.tsx`

- [ ] **Step 1: Atualizar entrada no BottomNav**

Em `frontend/src/components/BottomNav/BottomNav.tsx`:

Linha 25, trocar a entrada condicional por uma fixa:
```diff
-    ...(isDesktop ? [{ path: "/full-dashboard", icon: BarChart2, label: "Analytics" } as const] : []),
+    { path: "/painel", icon: BarChart2, label: "Painel" } as const,
```

Linha 11, remover o hook (já confirmado que `isDesktop` só era usado na entrada modificada acima):
```diff
-  const isDesktop = useMediaQuery("(min-width: 1024px)");
```

Linha 4, remover o import (não há outros consumidores no arquivo):
```diff
- import { useMediaQuery } from "@/hooks/useMediaQuery";
```

- [ ] **Step 2: Atualizar entrada no AppShell (sidebar desktop)**

Em `frontend/src/components/AppShell/AppShell.tsx`, linha 16:

```diff
-  { path: "/full-dashboard", icon: BarChart2,     label: "Analytics",  key: "analytics" },
+  { path: "/painel",         icon: BarChart2,     label: "Painel",     key: "painel" },
```

- [ ] **Step 3: Build pra garantir que não sobrou referência**

```bash
cd frontend && npm run build
```

Expected: build sem erros.

- [ ] **Step 4: Buscar referências remanescentes a `/full-dashboard` ou `FullDashboard`**

```bash
grep -rn "full-dashboard\|FullDashboard" frontend/src
```

Expected: zero resultados (todos devem ter sido removidos).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/BottomNav/BottomNav.tsx frontend/src/components/AppShell/AppShell.tsx
git commit -m "feat(nav): entrada Painel fixa no BottomNav e sidebar"
```

---

## Task 3: Blindar testes existentes contra mock de `matchMedia`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.test.tsx`

**Por quê?** O setup global em [frontend/src/test/setup.ts](frontend/src/test/setup.ts) faz `window.matchMedia` retornar `matches: false`. Quando a Task 4 adicionar `useMediaQuery("(min-width: 1024px)")` no `AnalyticsDashboard`, todos os testes existentes passariam a renderizar `MobileLayout`, e expectativas como `screen.getByText("Projeção de Saldo (próximos 3 meses)")` quebrariam (a Projeção fica na aba "Futuro", que não é a inicial). Antes de mexer no componente, vacinamos o teste forçando o mock a retornar `true` (= desktop).

- [ ] **Step 1: Adicionar mock de `matchMedia` no `beforeEach`**

Em `frontend/src/components/analytics/AnalyticsDashboard.test.tsx`, dentro do bloco `beforeEach` (linha ~93), antes do `vi.clearAllMocks()`, adicionar:

```ts
beforeEach(() => {
  vi.clearAllMocks()
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: true, // força desktop em todos os testes existentes
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })) as unknown as typeof window.matchMedia
})
```

- [ ] **Step 2: Rodar testes para garantir que ainda passam (antes de qualquer mudança de componente)**

```bash
cd frontend && npx vitest run src/components/analytics/AnalyticsDashboard.test.tsx
```

Expected: todos os testes existentes passam (nenhum quebrou com o mock adicional).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.test.tsx
git commit -m "test(analytics): mockar matchMedia=true nos testes existentes"
```

---

## Task 4: Extrair `DesktopLayout` e adicionar branch `useMediaQuery`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

Esta task é refatoração pura: extrai o markup atual do `AnalyticsDashboard` para dentro de um sub-componente `DesktopLayout`, deixa o `AnalyticsDashboard` decidindo via `useMediaQuery` (mas ainda só renderiza `DesktopLayout` — o ramo mobile é um placeholder). Sem mudança de comportamento visível.

- [ ] **Step 1: Adicionar import de `useMediaQuery`**

No topo de `frontend/src/components/analytics/AnalyticsDashboard.tsx`, somar aos imports:

```tsx
import { useMediaQuery } from "@/hooks/useMediaQuery"
```

- [ ] **Step 2: Extrair o conteúdo atual de `AnalyticsDashboard` em `DesktopLayout`**

Substituir o componente atual (a partir de `export function AnalyticsDashboard()`, linhas 620-684) por:

```tsx
function DesktopLayout({
  months,
  setMonths,
  data,
  fullData,
  goalHistory,
  fullGoalHistory,
  paymentData,
}: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
  data: MonthData[]
  fullData: MonthData[]
  goalHistory: GoalMonthlyPoint[]
  fullGoalHistory: GoalMonthlyPoint[]
  paymentData: PaymentMethodMonthData[]
}) {
  return (
    <div className="min-h-screen bg-background">
      <div className="border-b border-border px-6 py-4 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Painel</h1>
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

      <div className="p-6 space-y-6">
        <KpiCards />

        <div className="grid grid-cols-2 gap-6">
          <RevenueExpensesChart data={data} />
          <PatrimonioChart data={data} goalHistory={goalHistory} />
          <CategoryPieChart data={data} />
          <PaymentMethodPieChart data={paymentData} />
          <MonthComparisonChart data={data} />
          <ProjectionChart data={data} fullData={fullData} fullGoalHistory={fullGoalHistory} />
        </div>
      </div>
    </div>
  )
}

export function AnalyticsDashboard() {
  const [months, setMonths] = useState<TimeFilter>(12)
  const { data, loading, error } = useAnalytics(months)
  const { data: fullData } = useAnalytics(12)
  const { data: goalHistory } = useGoalHistory(months)
  const { data: fullGoalHistory } = useGoalHistory(12)
  const { data: paymentData } = usePaymentMethodAnalytics(months)
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
  />
}
```

- [ ] **Step 3: Criar placeholder `MobileLayout` (será preenchido na Task 5)**

No mesmo arquivo, **logo antes** de `DesktopLayout`, adicionar:

```tsx
function MobileLayout(_props: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
  data: MonthData[]
  fullData: MonthData[]
  goalHistory: GoalMonthlyPoint[]
  fullGoalHistory: GoalMonthlyPoint[]
  paymentData: PaymentMethodMonthData[]
}) {
  return (
    <div className="min-h-screen bg-background flex items-center justify-center">
      <p className="text-muted-foreground">Mobile em construção</p>
    </div>
  )
}
```

- [ ] **Step 4: Rodar tests pra confirmar que desktop ainda passa**

```bash
cd frontend && npx vitest run src/components/analytics/AnalyticsDashboard.test.tsx
```

Expected: todos os testes passam (matchMedia mockada em true → renderiza DesktopLayout).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "refactor(analytics): extrair DesktopLayout e add useMediaQuery"
```

---

## Task 5: Implementar `PainelHeader` e `KpiCardsMobile`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

- [ ] **Step 1: Criar `KpiCardsMobile`**

Em `frontend/src/components/analytics/AnalyticsDashboard.tsx`, logo após `KpiCards` (perto da linha 89), adicionar:

```tsx
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
```

- [ ] **Step 2: Criar `PainelHeader` reutilizável**

Logo antes de `MobileLayout`, adicionar:

```tsx
function PainelHeader({
  months,
  setMonths,
}: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
}) {
  return (
    <div className="border-b border-border px-4 py-3 lg:px-6 lg:py-4 flex items-center justify-between gap-2 flex-wrap">
      <div>
        <h1 className="text-xl lg:text-2xl font-bold">Painel</h1>
        <p className="text-xs lg:text-sm text-muted-foreground mt-0.5">
          Últimos {months} {months === 1 ? "mês" : "meses"}
        </p>
      </div>
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
  )
}
```

- [ ] **Step 3: Atualizar `DesktopLayout` para usar `PainelHeader` (DRY)**

Em `DesktopLayout`, **substituir** o bloco do header (`<div className="border-b border-border px-6 py-4 flex items-center justify-between">...</div>`) por:

```tsx
<PainelHeader months={months} setMonths={setMonths} />
```

Conferir que a classe Tailwind do `PainelHeader` cobre o equivalente desktop (`lg:px-6 lg:py-4`, `lg:text-2xl`, etc.) — sim, cobre.

- [ ] **Step 4: Substituir o placeholder do `MobileLayout` pela versão com header + KPIs**

```tsx
function MobileLayout({
  months,
  setMonths,
}: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
  data: MonthData[]
  fullData: MonthData[]
  goalHistory: GoalMonthlyPoint[]
  fullGoalHistory: GoalMonthlyPoint[]
  paymentData: PaymentMethodMonthData[]
}) {
  return (
    <div className="min-h-screen bg-background">
      <PainelHeader months={months} setMonths={setMonths} />
      <div className="p-4 space-y-4">
        <KpiCardsMobile />
      </div>
    </div>
  )
}
```

> Nota: As props `data, fullData, goalHistory, fullGoalHistory, paymentData` aparecem no tipo mas não são destruturadas — TypeScript não reclama de prop não-destruturada. A Task 8 vai consumi-las.

- [ ] **Step 5: Rodar tests**

```bash
cd frontend && npx vitest run src/components/analytics/AnalyticsDashboard.test.tsx
```

Expected: todos passam (desktop continua mockada → `DesktopLayout` renderiza com o novo `PainelHeader` cujo título "Painel" não é checado por nenhum teste existente — todos checam textos de gráficos).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "feat(painel): PainelHeader compartilhado + KpiCardsMobile"
```

---

## Task 6: Adicionar `compact` prop em `CategoryPieChart` e `PaymentMethodPieChart`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

- [ ] **Step 1: `CategoryPieChart` aceita `compact?`**

Localizar a assinatura `function CategoryPieChart({ data }: { data: MonthData[] })` (~linha 173) e trocar por:

```tsx
function CategoryPieChart({ data, compact = false }: { data: MonthData[]; compact?: boolean }) {
```

Localizar o `<CardHeader className="flex flex-row items-center justify-between pb-2">` (~linha 201) e trocar por uma versão condicional:

```tsx
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
```

- [ ] **Step 2: `PaymentMethodPieChart` aceita `compact?`**

Localizar a assinatura `function PaymentMethodPieChart({ data }: { data: PaymentMethodMonthData[] })` (~linha 247) e trocar por:

```tsx
function PaymentMethodPieChart({ data, compact = false }: { data: PaymentMethodMonthData[]; compact?: boolean }) {
```

Localizar o `<CardHeader className="flex flex-row items-center justify-between pb-2">` (~linha 266) e trocar por:

```tsx
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
```

- [ ] **Step 3: Rodar tests**

```bash
cd frontend && npx vitest run src/components/analytics/AnalyticsDashboard.test.tsx
```

Expected: testes existentes passam (não passam `compact`, então cai no default `false` = comportamento atual).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "feat(analytics): compact prop em CategoryPieChart e PaymentMethodPieChart"
```

---

## Task 7: Adicionar `compact` prop em `MonthComparisonChart`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

- [ ] **Step 1: `MonthComparisonChart` aceita `compact?`**

Localizar a assinatura `function MonthComparisonChart({ data }: { data: MonthData[] })` (~linha 415) e trocar por:

```tsx
function MonthComparisonChart({ data, compact = false }: { data: MonthData[]; compact?: boolean }) {
```

- [ ] **Step 2: Adaptar header em modo `compact`**

Localizar o bloco `<CardHeader className="flex flex-row items-start justify-between pb-2 flex-wrap gap-3">...</CardHeader>` (~linhas 484-536). Trocar a className por uma condicional:

```tsx
<CardHeader className={compact
  ? "flex flex-col items-stretch gap-3 pb-2"
  : "flex flex-row items-start justify-between pb-2 flex-wrap gap-3"
}>
```

E adaptar o sub-bloco com os controles `Mês A / Mês B / toggles`. Localizar:

```tsx
<div className="flex gap-3 items-center flex-wrap">
```

Trocar por:

```tsx
<div className={compact ? "flex flex-col gap-2" : "flex gap-3 items-center flex-wrap"}>
```

- [ ] **Step 3: Esconder botão `+ categoria` em modo `compact`**

Localizar o bloco do dropdown `unselected.length > 0 && (...)` dentro do `CardContent` (~linha 555-577). Envolver com guard:

```tsx
{!compact && unselected.length > 0 && (
  <div className="relative" ref={dropdownRef}>
    ...
  </div>
)}
```

- [ ] **Step 4: Rodar tests**

```bash
cd frontend && npx vitest run src/components/analytics/AnalyticsDashboard.test.tsx
```

Expected: testes existentes passam (não passam `compact`, comportamento desktop preservado).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "feat(analytics): compact prop em MonthComparisonChart"
```

---

## Task 8: Implementar tab bar e conteúdo das abas no `MobileLayout`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

- [ ] **Step 1: Adicionar estado da aba ativa e tab bar**

Substituir o `MobileLayout` (criado na Task 5) por:

```tsx
type PainelTab = "fluxo" | "categorias" | "futuro"

function MobileLayout({
  months,
  setMonths,
  data,
  fullData,
  goalHistory,
  fullGoalHistory,
  paymentData,
}: {
  months: TimeFilter
  setMonths: (m: TimeFilter) => void
  data: MonthData[]
  fullData: MonthData[]
  goalHistory: GoalMonthlyPoint[]
  fullGoalHistory: GoalMonthlyPoint[]
  paymentData: PaymentMethodMonthData[]
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
          <ProjectionChart data={data} fullData={fullData} fullGoalHistory={fullGoalHistory} />
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Rodar tests para garantir que desktop não regrediu**

```bash
cd frontend && npx vitest run src/components/analytics/AnalyticsDashboard.test.tsx
```

Expected: todos os testes existentes passam (matchMedia=true → DesktopLayout, sem alteração).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "feat(painel): tab bar e conteúdo das abas no mobile"
```

---

## Task 9: Testes do `MobileLayout`

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.test.tsx`

- [ ] **Step 1: Adicionar helper `mockMatchMedia` controlável**

No topo do arquivo de teste (após os imports e antes dos mocks de módulos), adicionar:

```ts
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
```

E atualizar o `beforeEach` (que já chama `matchMedia` inline) para usar o helper:

```ts
beforeEach(() => {
  vi.clearAllMocks()
  mockMatchMedia(true) // desktop por padrão
})
```

- [ ] **Step 2: Escrever testes para o `MobileLayout`**

No final do arquivo de teste, adicionar um novo `describe`:

```ts
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
```

- [ ] **Step 3: Rodar suíte completa do teste**

```bash
cd frontend && npx vitest run src/components/analytics/AnalyticsDashboard.test.tsx
```

Expected: todos os testes (existentes + novos do MobileLayout) passam.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.test.tsx
git commit -m "test(painel): cobertura do MobileLayout (tabs, KPIs, troca de conteúdo)"
```

---

## Task 10: Verificação manual no browser

**Files:** nenhum (verificação no app rodando)

- [ ] **Step 1: Subir o frontend localmente**

```bash
cd frontend && npm run dev
```

Expected: dev server inicia (normalmente em http://localhost:5173).

- [ ] **Step 2: Verificar desktop (≥ 1024px)**

Abrir `http://localhost:5173/painel` em viewport desktop.

Conferir:
- Título "Painel" no header.
- KPIs em grid 3 colunas (mesma aparência de antes).
- Grid 2x3 com os 6 gráficos.
- Sidebar mostra "Painel" no item.
- Acessar `http://localhost:5173/full-dashboard` → mostra NotFound.
- Acessar `http://localhost:5173/insights` → redireciona pra `/painel`.

- [ ] **Step 3: Verificar mobile (390px, iPhone 12 Pro no DevTools)**

Conferir:
- Header compacto, título e chips na mesma linha.
- 3 KPIs empilhados (label esquerda / valor direita) — valores não truncados.
- Tab bar com Fluxo / Categorias / Futuro.
- Aba inicial Fluxo mostra Receitas × Despesas + Patrimônio.
- Trocar para Categorias → 3 gráficos (Categorias, Formas de Pagamento, Comparação) com selects abaixo do título, sem botão `+ categoria`.
- Trocar para Futuro → Projeção.
- Sem horizontal scroll em 390px.
- BottomNav mostra "Painel" como entrada.

- [ ] **Step 4: Verificar transição responsiva**

Redimensionar do mobile pra desktop (passa pelos 1024px). O layout troca sem erros no console.

- [ ] **Step 5: Sem commit (verificação humana, nada a versionar)**

---

## Resumo de commits esperados

```
refactor: renomear /full-dashboard para /painel
feat(nav): entrada Painel fixa no BottomNav e sidebar
test(analytics): mockar matchMedia=true nos testes existentes
refactor(analytics): extrair DesktopLayout e add useMediaQuery
feat(painel): PainelHeader compartilhado + KpiCardsMobile
feat(analytics): compact prop em CategoryPieChart e PaymentMethodPieChart
feat(analytics): compact prop em MonthComparisonChart
feat(painel): tab bar e conteúdo das abas no mobile
test(painel): cobertura do MobileLayout (tabs, KPIs, troca de conteúdo)
```
