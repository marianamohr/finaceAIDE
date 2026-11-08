# Transaction Type Badge

**Date:** 2026-05-24
**Status:** Approved

## Overview

Adicionar um badge visual em cada transação para indicar se ela é uma assinatura de cartão, gasto fixo recorrente pago manualmente, ou uma compra avulsa. O objetivo é dar contexto rápido ao usuário sobre a natureza de cada gasto sem precisar abrir o detalhe.

## Lógica de Classificação

Toda a lógica vive no frontend — nenhuma mudança de API é necessária. Os campos `source`, `billing_year` e `billing_month` já chegam em cada transação pelo endpoint `/dashboard`.

| Condição | Tipo | Badge |
|---|---|---|
| `source === "recurring"` **e** `billing_year` preenchido | Assinatura de cartão | 💳 Assinatura |
| `source === "recurring"` **e** `billing_year` ausente | Gasto fixo manual | 🔁 Recorrente |
| qualquer outro `source` | Compra avulsa | 📌 Avulso |

## Componente Utilitário

Uma função pura `getTransactionBadge(source, billingYear)` retorna `{ icon, label, color } | null`. Fica em `src/lib/transactionBadge.ts`.

```ts
type TransactionBadge = { icon: string; label: string; color: string }

export function getTransactionBadge(source: string, billingYear?: number): TransactionBadge {
  if (source === "recurring") {
    return billingYear != null
      ? { icon: "💳", label: "Assinatura", color: "#6366f1" }
      : { icon: "🔁", label: "Recorrente", color: "#10b981" }
  }
  return { icon: "📌", label: "Avulso", color: "#64748b" }
}
```

Retorna sempre um badge (os 3 casos têm badge, inclusive avulso).

## Interface `Transaction`

Adicionar `billing_year?: number` ao tipo `Transaction` em `useDashboard.ts` — o campo já existe na resposta do backend mas não estava tipado no frontend.

## Onde Aparece

### 1. TransactionCard

Badge compacto (ícone + label) exibido à esquerda do valor. Fica na mesma linha do valor, alinhado à direita, antes do número.

```
[Merchant · categoria · data]    [💳 Assinatura]  [−R$ 55,90]
```

Implementado diretamente dentro de `TransactionCard.tsx`, chamando `getTransactionBadge(source, billing_year)`.

### 2. Sheet de detalhe (Consumo.tsx)

**No título:** badge inline após o nome do merchant/categoria no `SheetTitle`.

**Como linha de detalhe:** nova linha entre "Categoria" e "Valor":
```
Categoria    Lazer
Tipo         💳 Assinatura      ← nova linha
Valor        −R$ 55,90
Data         01/05/2026
```

### 3. Aba Caixa

Caixa tem dois renders inline (não usa `TransactionCard`):

- **`realTxs`** — transações não-crédito renderizadas em lista. Badge adicionado inline ao lado do valor, usando `source` e `billing_year` do tipo `CashflowTransaction` (que já expõe `type` e `kind`; precisará de `source` adicionado ao tipo e à query do backend).
- **Faturas de cartão** — lista dentro do accordion de cada fatura (`invoice.transactions`). Essas transações são de cartão de crédito, portanto sempre "Assinatura" se `source === "recurring"`, ou "Avulso" caso contrário. Badge adicionado inline.

> **Nota:** `CashflowTransaction` em `useCashflow.ts` precisa ter o campo `source` adicionado, e o backend (`CashflowByMonth`) já retorna `source` — só falta expô-lo no tipo frontend.

## Visual do Badge

- Pill pequena: ícone + texto, `font-size: 10px`, `border-radius: 6px`, `padding: 2px 7px`
- Cor de fundo com opacidade (`/20`) e texto na cor sólida, para não pesar visualmente no card
- Cores:
  - Assinatura: `#6366f1` (roxo)
  - Recorrente: `#10b981` (verde)
  - Avulso: `#64748b` (cinza)

## Arquivos Afetados

- `src/lib/transactionBadge.ts` — novo utilitário (função pura)
- `src/hooks/useDashboard.ts` — adicionar `billing_year?: number` ao tipo `Transaction`
- `src/components/TransactionCard/TransactionCard.tsx` — renderizar badge
- `src/pages/Consumo.tsx` — badge no título e linha de detalhe do sheet
- Aba Caixa — qualquer componente de lista que use `TransactionCard`

## O que Não Muda

- Nenhuma mudança de backend ou API
- Nenhuma mudança no modelo de dados
- Nenhuma nova rota ou query
