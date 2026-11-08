# Design: Filtro de Tempo Global no Full Dashboard

**Data:** 2026-05-21

## Contexto

O `/full-dashboard` (`AnalyticsDashboard`) exibe 6 charts com dados dos últimos 12 meses fixos. O objetivo é adicionar um filtro global de tempo (1, 3, 6, 12 meses) que controla o período exibido em todos os charts simultaneamente.

## Decisões

- **Abordagem:** estado local em `AnalyticsDashboard` — `months: 1 | 3 | 6 | 12` como `useState(12)`.
- **Projeção:** `ProjectionChart` sempre calcula a média com 12 meses de histórico (`fullData`), mas exibe apenas os meses do filtro na parte real do gráfico.
- **Dropdowns internos:** `CategoryPieChart`, `PaymentMethodPieChart` e `MonthComparisonChart` têm seus dropdowns de mês limitados aos meses do filtro global.

## Data Layer

### `useAnalytics(months: number)`

Adicionar parâmetro `months` (padrão 12). O loop que gera a lista de meses usa `months`. Adicionar `months` ao array de deps do `useEffect`.

### `useGoalHistory` e `usePaymentMethodAnalytics`

Já aceitam `months` — apenas passar o valor do filtro vindo do pai.

### `ProjectionChart`

Recebe dois props:
- `data: MonthData[]` — fatia do filtro (para exibição da linha real)
- `fullData: MonthData[]` — sempre 12 meses (para cálculo de `avgMonthlyChange`)

## UI

### Filtro global

No header do `AnalyticsDashboard`, ao lado do subtítulo, grupo de 4 botões pill toggle:

```
1m   3m   6m   12m
```

- Padrão selecionado: **12m**
- Estilo: botão ativo com fundo sólido (`bg-primary text-primary-foreground`), inativo com borda (`border`)

### Subtítulo dinâmico

O subtítulo "Visão completa dos últimos 12 meses" passa a refletir o filtro selecionado: "Visão dos últimos N meses".

## Fluxo de Dados

```
AnalyticsDashboard
  months: 1 | 3 | 6 | 12  ← useState(12)

  useAnalytics(months)          → data (fatia filtrada)
  useAnalytics(12)              → fullData (sempre 12, para projeção)
  useGoalHistory(months)        → goalHistory (para PatrimonioChart)
  useGoalHistory(12)            → fullGoalHistory (para ProjectionChart)
  usePaymentMethodAnalytics(months) → passado para PaymentMethodPieChart

  RevenueExpensesChart      ← data
  PatrimonioChart           ← data, goalHistory(months)
  CategoryPieChart          ← data  (dropdown limitado ao filtro)
  PaymentMethodPieChart     ← paymentData(months)
  MonthComparisonChart      ← data  (dropdowns A/B limitados ao filtro)
  ProjectionChart           ← data (exibição) + fullData + fullGoalHistory (cálculo)
```

## Componentes Afetados

| Componente | Mudança |
|---|---|
| `useAnalytics` | Aceita `months` como parâmetro |
| `AnalyticsDashboard` | Adiciona `useState(12)`, dois calls de `useAnalytics`, UI do filtro, propaga `months` |
| `PatrimonioChart` | Recebe `goalHistory` como prop em vez de chamar o hook internamente |
| `ProjectionChart` | Recebe `data` + `fullData` + `fullGoalHistory` como props |
| `PaymentMethodPieChart` | Recebe `data` como prop em vez de chamar o hook internamente |
| `CategoryPieChart` | Sem mudança de lógica — `data` já filtrada resolve o dropdown |
| `MonthComparisonChart` | Sem mudança de lógica — `data` já filtrada resolve os dropdowns A/B |

## Sem Escopo

- Persistência do filtro em URL ou localStorage
- Filtro em outras páginas além do `/full-dashboard`
- KpiCards: continuam exibindo dados do mês corrente (não são afetados pelo filtro)
