# Design: Página de Detalhe — Projeção de Gastos

**Data:** 2026-06-08

## Contexto

O dashboard (Painel) exibe um gráfico de barras empilhadas chamado "Projeção de Gastos" (`ExpenseProjectionChart`), que mostra os totais projetados dos próximos meses divididos em três buckets: Fixos, Parcelas e Cartão. A visão atual é apenas agregada — não é possível ver quais itens compõem cada valor.

O objetivo é adicionar uma página de detalhe acessível pelo Painel, que mostra os itens individuais por mês de forma clara e navegável.

## Decisões de Design

- **Como abrir:** Botão "Ver detalhes →" abaixo do `ExpenseProjectionChart` no dashboard, navegando para `/projecao`.
- **Navegação entre meses:** Abas com scroll horizontal (uma aba por mês projetado, até 6).
- **Voltar:** Botão "← Painel" no cabeçalho da página, navegando para `/painel`.
- **API:** Novo endpoint separado retornando todos os meses com itens em uma única requisição.

## Backend

### Novo endpoint

`GET /transactions/expense-projection/detail?months=6`

Retorna os mesmos totais do endpoint existente (`/expense-projection`) mais as listas de itens por bucket.

### Estrutura de resposta

```json
{
  "months": [
    {
      "year": 2025,
      "month": 7,
      "label": "Jul/25",
      "fixed": 1665,
      "installments": 400,
      "card": 335,
      "total": 2400,
      "items": {
        "fixed": [
          { "name": "Aluguel", "amount": 1500 },
          { "name": "Netflix", "amount": 45 },
          { "name": "Energia", "amount": 120 }
        ],
        "installments": [
          { "name": "iPhone 14", "amount": 400, "installment_info": "3/12" }
        ],
        "card": [
          { "name": "Spotify", "amount": 25 },
          { "name": "Amazon Prime", "amount": 19 }
        ]
      }
    }
  ]
}
```

**`installment_info`** é opcional — preenchido apenas quando a transação tem essa informação disponível.

### Implementação no repositório

Novo método `ExpenseProjectionDetailByMonths(accountID uuid.UUID, n int)` em `backend/internal/repository/transaction.go`.

Segue a mesma lógica de `ExpenseProjectionByMonths`, porém em vez de apenas acumular totais, também coleta os registros individuais:

- **fixed:** query em `recurring_expenses` (ativas no mês, sem cartão de crédito) — retorna `description`/`name` e `amount`
- **installments:** query em `transactions` com JOIN em `installment_purchases` (cartão de crédito, `source = 'installment'`, `billing_year/billing_month` do mês projetado) — retorna `merchant`/`description` e `amount`; `installment_info` = `"<installment_number>/<installments>"` (ex: `"3/12"`) derivado de `transactions.installment_number` e `installment_purchases.installments`
- **card:** transações de cartão materializadas (`source != 'installment'`) + `recurring_expenses` virtuais de cartão não materializadas para o mês

### Rota

```
GET /transactions/expense-projection/detail
```

Registrada ao lado da rota existente em `backend/cmd/server/main.go`.

## Frontend

### Nova rota

`/projecao` adicionada em `frontend/src/App.tsx`, protegida por `ProtectedRoute` e dentro do `AppShell`.

### Nova página

`frontend/src/pages/ProjecaoDetalhe.tsx`

**Estrutura da UI:**
1. **Cabeçalho:** botão "← Painel" (navega para `/painel`) + título "Projeção de Gastos"
2. **Abas:** uma por mês projetado, com scroll horizontal. Aba ativa destacada com borda inferior azul.
3. **Cards de resumo:** três cards lado a lado (Fixos em verde, Parcelas em roxo, Cartão em ciano) + linha de total do mês
4. **Listas de itens por bucket:**
   - Cada bucket tem cabeçalho colorido (cor do bucket) + lista em card com separadores
   - Parcelas exibem subtexto "parcela X de Y" quando `installment_info` disponível
   - Buckets sem itens não são renderizados

### Novo hook

`useExpenseProjectionDetail(months = 6)` em `frontend/src/hooks/useAnalytics.ts`

Chama `GET /transactions/expense-projection/detail?months=N`, retorna `{ data, loading }`.

### Botão no dashboard

`ExpenseProjectionChart` em `AnalyticsDashboard.tsx` ganha um botão/link "Ver detalhes →" no rodapé do card, navegando para `/projecao`. Sem outras alterações no componente existente.

## O que não muda

- Endpoint `/transactions/expense-projection` existente — sem alterações
- Componente `ExpenseProjectionChart` — apenas recebe o botão adicional
- Demais páginas e rotas
