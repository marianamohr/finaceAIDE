# Painel Mobile — Design

## Goal

A página `/full-dashboard` (hoje chamada "Dashboard Analítico") não tem versão mobile usável: KPIs e gráficos ficam comprimidos em 390px, valores são truncados e o `BottomNav` esconde a entrada em telas < 1024px. Este design entrega:

1. Renomeia a página para **Painel** (rota `/painel`, label "Painel" no BottomNav).
2. Mantém a entrada no `BottomNav` em qualquer resolução.
3. Cria um layout mobile baseado em **3 abas** (Fluxo / Categorias / Futuro), preservando os 6 gráficos existentes com adaptações específicas.
4. **Mantém o desktop intacto** — o grid 2x3 atual não muda.

## Contexto

- Página atual: [frontend/src/pages/FullDashboard.tsx](frontend/src/pages/FullDashboard.tsx) → renderiza `<AnalyticsDashboard />`.
- Componente principal: [frontend/src/components/analytics/AnalyticsDashboard.tsx](frontend/src/components/analytics/AnalyticsDashboard.tsx). Contém 6 gráficos:
  1. `RevenueExpensesChart` (receitas × despesas, barras + linha)
  2. `PatrimonioChart` (saldo + reservas + patrimônio total, áreas + linha)
  3. `CategoryPieChart` (pizza por categoria, com seletor de mês)
  4. `PaymentMethodPieChart` (pizza por forma de pagamento, com seletor de mês)
  5. `MonthComparisonChart` (compara 2 meses por categorias selecionadas — componente mais pesado, com 5 controles no header)
  6. `ProjectionChart` (projeção de patrimônio nos próximos 3 meses)
- KPIs hoje: `Saldo da Conta`, `Reservas`, `Patrimônio Total` em `grid-cols-3`.
- Filtro de tempo no header: chips `1m / 3m / 6m / 12m`.
- BottomNav atual ([frontend/src/components/BottomNav/BottomNav.tsx:25](frontend/src/components/BottomNav/BottomNav.tsx#L25)) inclui a entrada **só** quando `useMediaQuery("(min-width: 1024px)")` é true.

---

## Mudanças

### 1. Rota e BottomNav

**Em [frontend/src/App.tsx](frontend/src/App.tsx):**

- Remove a rota `/full-dashboard`.
- Adiciona `/painel` apontando para o novo componente `<Painel />`.
- `/insights` (que hoje redireciona para `/full-dashboard`) passa a redirecionar para `/painel`.

```diff
- import FullDashboard from "./pages/FullDashboard";
+ import Painel from "./pages/Painel";
...
-  <Route path="/insights" element={<Navigate to="/full-dashboard" replace />} />
-  <Route path="/full-dashboard" element={<ProtectedRoute><AppShell><FullDashboard /></AppShell></ProtectedRoute>} />
+  <Route path="/insights" element={<Navigate to="/painel" replace />} />
+  <Route path="/painel" element={<ProtectedRoute><AppShell><Painel /></AppShell></ProtectedRoute>} />
```

**Em [frontend/src/components/BottomNav/BottomNav.tsx](frontend/src/components/BottomNav/BottomNav.tsx):**

- Substitui a entrada condicional por uma entrada fixa (visível em qualquer resolução).
- Remove a chamada a `useMediaQuery("(min-width: 1024px)")` se não houver outros consumidores no arquivo.

```diff
-  ...(isDesktop ? [{ path: "/full-dashboard", icon: BarChart2, label: "Analytics" } as const] : []),
+  { path: "/painel", icon: BarChart2, label: "Painel" },
```

Ícone mantém-se como `BarChart2`. Label: "Painel".

### 2. Renomeação de arquivo

- `frontend/src/pages/FullDashboard.tsx` → `frontend/src/pages/Painel.tsx`.
- Renomeia o componente exportado: `FullDashboard` → `Painel`.
- Conteúdo do arquivo continua sendo o wrapper fino que renderiza `<AnalyticsDashboard />`.
- O nome interno `AnalyticsDashboard` **não muda** — "Painel" é apenas a marca user-facing (rota, título, label).

### 3. Layout responsivo em `AnalyticsDashboard`

`AnalyticsDashboard.tsx` continua sendo o ponto único que faz todas as queries (`useAnalytics`, `useGoalHistory`, `usePaymentMethodAnalytics`, `useDashboardQuery`, `useReservas`) e decide o layout via `useMediaQuery("(min-width: 1024px)")`.

```tsx
export function AnalyticsDashboard() {
  const [months, setMonths] = useState<TimeFilter>(12)
  const { data, loading, error } = useAnalytics(months)
  const { data: fullData } = useAnalytics(12)
  const { data: goalHistory } = useGoalHistory(months)
  const { data: fullGoalHistory } = useGoalHistory(12)
  const { data: paymentData } = usePaymentMethodAnalytics(months)
  const isDesktop = useMediaQuery("(min-width: 1024px)")

  if (loading) return <LoadingState />
  if (error) return <ErrorState message={error} />

  const sharedProps = { data, fullData, goalHistory, fullGoalHistory, paymentData, months, setMonths }

  return isDesktop ? <DesktopLayout {...sharedProps} /> : <MobileLayout {...sharedProps} />
}
```

Sub-componentes novos, todos no mesmo arquivo `AnalyticsDashboard.tsx`:

- **`PainelHeader`** — título "Painel" + chips de filtro de tempo. Responsivo via Tailwind (chips menores em < lg).
- **`DesktopLayout`** — extrai o markup atual (KPIs em `grid-cols-3` + grid `grid-cols-2` com os 6 gráficos). Funcionalmente **idêntico** ao que existe hoje.
- **`MobileLayout`** — KPIs stacked + tab bar + conteúdo da aba ativa.
- **`KpiCardsMobile`** — variação vertical do `KpiCards`: label à esquerda, valor à direita (não empilhado).

### 4. MobileLayout — estrutura

```
┌─────────────────────────────────────┐
│ Painel              [1m][3m][6m][12m]│  PainelHeader
│ Últimos 12 meses                     │
├─────────────────────────────────────┤
│ Saldo da Conta            R$ 141,9k │  KpiCardsMobile
│ Reservas                   R$ 19,3k │  (3 cards empilhados, label/valor flex row)
│ Patrimônio Total          R$ 161,2k │
├─────────────────────────────────────┤
│ [ Fluxo ][ Categorias ][ Futuro ]   │  Tab bar (sticky top: 0)
├─────────────────────────────────────┤
│                                      │
│   Conteúdo da aba ativa             │
│   (scroll vertical natural)         │
│                                      │
└─────────────────────────────────────┘
```

**Estado da aba ativa:**

```tsx
type Tab = "fluxo" | "categorias" | "futuro"
const [tab, setTab] = useState<Tab>("fluxo")
```

Sem persistência em URL. Ao trocar de página e voltar, reinicia em `"fluxo"`.

**Conteúdo por aba:**

| Aba | Gráficos |
|---|---|
| **Fluxo** | `RevenueExpensesChart`, `PatrimonioChart` |
| **Categorias** | `CategoryPieChart`, `PaymentMethodPieChart`, `MonthComparisonChart` (compact) |
| **Futuro** | `ProjectionChart` |

### 5. Adaptações dos gráficos para mobile

Os 6 componentes de gráfico recebem (quando necessário) uma prop opcional `compact?: boolean`. Quando `true`, ajustam o header e controles para caber em telas estreitas.

- **`CategoryPieChart`** — `compact={true}`:
  - O `<select>` de mês sai do `CardHeader` (que usa `flex-row`) e fica numa linha própria abaixo do título.
- **`PaymentMethodPieChart`** — mesma adaptação que acima.
- **`MonthComparisonChart`** — `compact={true}`:
  - Header em coluna (sem `flex-row` com 5 controles).
  - Pré-seleciona top 3 categorias (já é o default).
  - Esconde o botão `+ categoria` no mobile. O usuário fica com as top 3 categorias pré-selecionadas (ainda pode remover tags, mas não adicionar).
  - Selects de Mês A / Mês B e o toggle `≡≡ / Δ` ficam empilhados, não lado a lado.

Os outros gráficos (`RevenueExpensesChart`, `PatrimonioChart`, `ProjectionChart`) **não precisam de prop nova** — o markup atual já se adapta bem (sem controles no header além do título).

### 6. PainelHeader — responsividade

```tsx
function PainelHeader({ months, setMonths }: { ... }) {
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

Em telas muito estreitas (≤ 320px) os chips podem quebrar linha — aceitável.

### 7. Tab bar — sticky

A barra de abas usa `position: sticky` + `top: 0`. O `PainelHeader` e os `KpiCardsMobile` rolam normalmente; quando a tab bar encosta no topo do scroll container, ela gruda ali. Depende do container pai ter `overflow-y-auto`. Verificar no `AppShell.tsx`; se o sticky não funcionar de primeira, fallback é tab bar não-sticky (rola junto com o conteúdo).

```tsx
<div className="sticky top-0 z-10 bg-background border-b border-border">
  <div className="flex gap-1 px-4 py-2">
    {(["fluxo", "categorias", "futuro"] as const).map((t) => (
      <button
        key={t}
        onClick={() => setTab(t)}
        className={`flex-1 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
          tab === t ? "bg-primary text-primary-foreground" : "text-muted-foreground"
        }`}
      >
        {tabLabels[t]}
      </button>
    ))}
  </div>
</div>
```

---

## Testes

- [frontend/src/components/analytics/AnalyticsDashboard.test.tsx](frontend/src/components/analytics/AnalyticsDashboard.test.tsx) — atualizar:
  - Mock de `useMediaQuery` retornando `true` para validar `DesktopLayout` (caminho atual).
  - Mock de `useMediaQuery` retornando `false` para validar `MobileLayout`:
    - Render dos 3 KPIs empilhados.
    - Tab bar com "Fluxo", "Categorias", "Futuro".
    - Aba inicial `"fluxo"` renderiza `RevenueExpensesChart` e `PatrimonioChart`.
    - Clique em "Categorias" troca o conteúdo para os 3 gráficos da aba.
    - Clique em "Futuro" renderiza `ProjectionChart`.

---

## Riscos / atenção

1. **`useMediaQuery` flicker no mount** — o hook pode retornar `false` no primeiro tick antes de medir o viewport, causando flash do `MobileLayout` em telas grandes. Verificar a implementação atual de `useMediaQuery` em [frontend/src/hooks/useMediaQuery.ts](frontend/src/hooks/useMediaQuery.ts); se houver flicker, ajustar para inicializar com o valor correto via `window.matchMedia(...).matches` na primeira chamada.
2. **Sticky tabs** — depende do container pai ter `overflow-y-auto` (e não ser o próprio `<html>`). Se não funcionar, manter as abas não-sticky.
3. **`MonthComparisonChart` em 320px** — é o gráfico com mais controles. A versão compact precisa caber sem horizontal scroll. Testar manualmente nessa largura.

---

## Fora de escopo

- Persistir aba ativa em URL (`?tab=fluxo`).
- Swipe horizontal entre abas.
- Animação de transição entre abas.
- Renomeio interno de `AnalyticsDashboard` para `Painel`.
- Lazy-load por aba (todas as queries continuam sendo feitas no mount, como hoje).
- Mudar o ícone do BottomNav.

---

## Critério de sucesso

- `/painel` abre em mobile (390px) **sem horizontal scroll**; KPIs legíveis, abas funcionando, todos os 6 gráficos acessíveis através das 3 abas.
- `/painel` em desktop (≥ 1024px) renderiza **exatamente** o grid 2x3 atual, sem regressão visual.
- BottomNav mostra "Painel" em qualquer resolução.
- `/full-dashboard` retorna 404.
- `/insights` redireciona para `/painel`.
- `AnalyticsDashboard.test.tsx` passa, cobrindo ambos os layouts.
