# Design: Refinamento da Projeção de Gastos

**Data:** 2026-06-09
**Rota:** `/projecao`
**Arquivo principal:** `frontend/src/pages/ProjecaoDetalhe.tsx`

---

## Objetivo

Tornar a tela de Projeção mais informativa e navegável, substituindo listas planas por acordeões agrupados com contexto (categoria para fixos, nome do cartão para cartão e parcelas).

---

## Decisões de Design

| Pergunta | Decisão |
|---|---|
| Layout dos fixos | Acordeões agrupados por categoria |
| Layout do cartão | Acordeões agrupados por nome do cartão |
| Layout das parcelas | Acordeões agrupados por nome do cartão |
| Estado inicial | Todos os acordeões fechados |
| Itens sem categoria | Agrupados em "Outros" |

---

## Mudanças no Backend

### 1. `ProjectionItem` — novos campos

```go
type ProjectionItem struct {
    Name            string  `json:"name"`
    Amount          float64 `json:"amount"`
    InstallmentInfo string  `json:"installment_info,omitempty"`
    CategoryLabel   string  `json:"category_label,omitempty"` // novo — só para fixos
    CardName        string  `json:"card_name,omitempty"`       // novo — para cartão e parcelas
}
```

### 2. Query de fixos — adicionar categoria

No `ExpenseProjectionDetailByMonths`, a query de `fixedRows` passa a fazer JOIN com `categories`. Usa `c.slug` (sem acentos) para facilitar o mapeamento de ícones no frontend:

```sql
SELECT re.name, re.amount, COALESCE(c.slug, 'outros') AS category_label
FROM recurring_expenses re
LEFT JOIN payment_methods pm ON pm.id = re.payment_method_id
LEFT JOIN categories c ON c.id = re.category_id
WHERE re.account_id = ?
  AND re.active = true
  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
  AND (pm.id IS NULL OR pm.kind <> 'credit_card')
ORDER BY re.amount DESC
```

O struct de `fixedRows` adiciona `CategoryLabel string`.

### 3. Query de parcelas — adicionar nome do cartão

A query de `installRows` passa a fazer JOIN com `payment_methods` e retornar `pm.name`:

```sql
SELECT
    COALESCE(NULLIF(t.merchant, ''), t.description, 'Sem descrição') AS name,
    -t.amount AS amount,
    t.installment_number,
    ip.installments AS total_installments,
    pm.name AS card_name
FROM transactions t
JOIN payment_methods pm ON pm.id = t.payment_method_id
LEFT JOIN installment_purchases ip ON ip.id = t.installment_purchase_id
WHERE t.account_id = ?
  AND pm.kind = 'credit_card'
  AND t.source = 'installment'
  AND t.type != 'invoice_payment'
  AND t.billing_year = ?
  AND t.billing_month = ?
ORDER BY t.amount ASC
```

### 4. Query de cartão (materializados e virtuais) — adicionar nome do cartão

**Materializados:** adicionar `pm.name AS card_name` ao SELECT.

**Virtuais (recurring):** adicionar `pm.name AS card_name` ao SELECT.

### 5. Montagem dos itens

Propagar `CategoryLabel` e `CardName` ao construir os `ProjectionItem` no loop de assembly.

---

## Mudanças no Frontend

### Novo tipo em `useAnalytics.ts`

```ts
export interface ProjectionItem {
  name: string
  amount: number
  installment_info?: string
  category_label?: string  // novo
  card_name?: string        // novo
}
```

### Novo componente `AccordionGroup`

Substitui `ItemList` em `ProjecaoDetalhe.tsx`.

```
AccordionGroup
  props: items, groupBy, color
  - agrupa os itens pelo campo `groupBy` ("category_label" | "card_name")
  - renderiza um bloco colapsável por grupo:
      [ícone] Nome do Grupo   R$ subtotal  ▶/▼
        (expandido)
        Item 1              R$ valor
        Item 2  parcela X/Y R$ valor
  - todos os grupos iniciam fechados (collapsed = true)
  - ao clicar no header do grupo, alterna collapsed
```

### Uso na página

```
Fixos    → <AccordionGroup items={m.items.fixed} groupBy="category_label" color={COLOR_FIXED} />
Cartão   → <AccordionGroup items={m.items.card} groupBy="card_name" color={COLOR_CARD} />
Parcelas → <AccordionGroup items={m.items.installments} groupBy="card_name" color={COLOR_INSTALLMENTS} />
```

### Ícones por categoria (fixos)

Mapeamento estático de `category_label` → emoji, usado no header do grupo:

| slug | ícone |
|---|---|
| casa | 🏠 |
| saude | ❤️ |
| lazer | 🎮 |
| impostos | 📋 |
| transporte | 🚗 |
| educacao | 📚 |
| servicos | ⚙️ |
| outros | 📦 |
| (cartão) | 💳 |

---

## O que NÃO muda

- Estrutura geral da página (tabs por mês, summary cards, total projetado)
- Lógica de cálculo dos totais no backend
- Hook `useExpenseProjectionDetail`
- Tipos `ProjectionItems`, `ExpenseProjectionMonthDetail`

---

## Casos de borda

- **Fixo sem categoria:** `category_label` = `"outros"` (defaultado no COALESCE da query).
- **Cartão com um único cartão:** o acordeão ainda é renderizado (consistência visual).
- **Grupo com um único item:** acordeão se comporta igual, sem tratamento especial.
- **Privacidade (valuesHidden):** subtotais dos grupos também aplicam `maskCurrency`.
