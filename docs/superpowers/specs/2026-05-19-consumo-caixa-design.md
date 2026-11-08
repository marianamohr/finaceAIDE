# Design: Páginas Consumo e Caixa

**Data:** 2026-05-19  
**Status:** Aprovado

---

## Contexto

O dashboard atual mistura duas visões conceitualmente distintas:

- **Onde estou gastando** (por categoria, data da compra)
- **Como o dinheiro sai da minha conta** (por forma de pagamento, data real da saída)

Transações de cartão de crédito causam confusão: aparecem no mês da compra nas categorias, mas o dinheiro só sai da conta no mês do pagamento da fatura. O objetivo deste redesign é separar essas duas visões em páginas distintas com semântica clara.

---

## Regras de negócio

### Billing month
`billing_month` = mês em que a fatura **fecha** (não o mês do vencimento).  
Calculado por `ComputeBillingMonth(txDate, closingDay)` em `models/payment_method.go`.

### Página Consumo
- Filtra transações pela `date` (data da compra), independente do método de pagamento.
- Crédito, débito, pix — todos entram pelo mês da compra.
- `invoice_payment` é excluído (não é consumo, é movimentação financeira).
- `transfer` é excluído do breakdown de categorias (já implementado).

### Página Caixa
- Mostra como o dinheiro **sai da conta**.
- PIX, débito, dinheiro, voucher: aparecem pelo `date` (saem na hora da compra).
- Cartão de crédito: agrupado por fatura. A fatura aparece no **mês do vencimento**.
  - Mês do vencimento da fatura = mês seguinte ao `billing_month` quando `due_day < closing_day`, caso contrário é o próprio `billing_month`.
  - Exemplo Nubank (fecha 26, vence 2): compra 19/mai → `billing_month=mai` → vence 02/jun → aparece em **junho**.
  - Exemplo Cartão Bia (fecha 1, vence 6): compra 19/mai → `billing_month=jun` → vence 06/jun → aparece em **junho**.
- Receitas (amount > 0) aparecem normalmente pelo `date`.
- `invoice_payment` **aparece** no Caixa como saída real (é quando o dinheiro de fato sai).
- Compras de crédito individuais **não aparecem** diretamente — ficam dentro da fatura ao expandir.

#### Cards de resumo — Caixa
- **Entradas**: soma de transações com `amount > 0`, excluindo `type = invoice_payment`, filtrando por `date` no mês.
- **Saídas reais**: soma de transações com `amount < 0` onde `payment_method.kind != 'credit_card'`, filtrando por `date` no mês. Inclui `invoice_payment` (pagamento de fatura é saída real).

#### Status de fatura
- **Aberta / Estimado**: fatura ainda não fechou (hoje < `closing_day` do mês do billing).
- **Aguardando pagamento**: fatura fechou mas não existe `invoice_payment` para aquele `payment_method_id + billing_year + billing_month`.
- **Pago ✓**: existe `invoice_payment` correspondente.

---

## Arquitetura

### Backend

#### Endpoint existente `GET /dashboard`
Continua sendo usado pela página Consumo sem mudanças no contrato.  
Verificar que `SummaryByMonth` exclui `invoice_payment` das despesas (já excluído via `excludedFromCategoryBreakdown`).

#### Novo endpoint `GET /cashflow?year=&month=`

Retorna:
```json
{
  "summary": {
    "income": 5000.00,
    "real_outflow": 200.00
  },
  "real_transactions": [
    {
      "id": "...",
      "amount": -200.00,
      "category": "alimentacao",
      "merchant": "Supermercado",
      "date": "2026-06-01",
      "payment_method_id": "...",
      "kind": "debit",
      "type": "expense"
    }
  ],
  "invoices": [
    {
      "payment_method_id": "...",
      "name": "Nubank",
      "color": "#8B5CF6",
      "kind": "credit_card",
      "billing_year": 2026,
      "billing_month": 5,
      "due_day": 2,
      "closing_day": 26,
      "total": -500.00,
      "is_paid": true,
      "transactions": [
        {
          "id": "...",
          "amount": -200.00,
          "category": "alimentacao",
          "merchant": "Restaurante",
          "date": "2026-05-15"
        }
      ]
    }
  ]
}
```

**Lógica de filtragem das faturas:**  
Para o mês `year/month` solicitado, busca faturas cujo **mês de vencimento** cai naquele mês.  
Mês de vencimento = `billing_month` se `due_day >= closing_day`, senão `billing_month + 1`.  
Na query: calcular `due_month` no Go após buscar os payment_methods do account, depois filtrar transações com `billing_year/billing_month` correspondentes.

**Lógica de `real_transactions`:**  
`SELECT * FROM transactions WHERE account_id = ? AND date >= start AND date < end AND (payment_method_id IS NULL OR pm.kind != 'credit_card') AND type != 'transfer'`  
Inclui `invoice_payment` (é saída real).

**Cálculo de `summary.real_outflow`:**  
Soma de `amount < 0` de `real_transactions`.

### Frontend

#### Rotas
| Antes | Depois |
|-------|--------|
| `/dashboard` | `/consumo` (redirect de `/dashboard` → `/consumo`) |
| — | `/caixa` (nova) |

#### Bottom nav
```
Consumo (BarChart3, /consumo) · Caixa (Wallet, /caixa) · Chat · Perfil
```
Itens opcionais existentes (Limites, Reservas, Tools) permanecem condicionais como hoje.

#### Página `Consumo.tsx` (renomeada de `Dashboard.tsx`)
- Remove cards de Receitas/Saldo — substitui por card único "Total consumido em {mês}".
- Remove aba "por forma de pagamento" (a visão de payment methods vai para Caixa).
- Mantém: seletor de mês, top 5 categorias com barras, lista de transações paginada, lembretes de gastos fixos, balance transfer banner, modais de edição/exclusão.
- Continua usando `useDashboardQuery` (mesmo endpoint `/dashboard`).

#### Página `Caixa.tsx` (nova)
- Hook `useCashflowQuery(year, month)` → `GET /cashflow?year=&month=`.
- Cards: Entradas (verde / `text-primary`) + Saídas reais (vermelho / `text-destructive`).
- Lista de movimentações:
  - Receitas e transações reais (pix/débito) ordenadas por `date desc`.
  - Divider visual separando transações reais de faturas.
  - Faturas de crédito agrupadas, cada uma como `InvoiceCard` expansível.
- `InvoiceCard`: mostra nome, cor do cartão, vencimento, total, badge de status. Ao clicar, expande lista de compras. Botão "Pagar" quando status = aguardando (reutiliza lógica de `payInvoice` existente).
- Seletor de mês idêntico ao da página Consumo.

#### Componente `InvoiceCard`
Props:
```ts
interface InvoiceCardProps {
  invoice: CashflowInvoice;
  onPay: () => void;
  onUnpay: () => Promise<void>;
  fmt: (v: number) => string;
}
```
Estados de badge:
- `is_paid = true` → badge verde "Pago ✓"
- `is_paid = false` + fatura fechada → badge vermelho "Aguardando" + botão "Pagar"
- `is_paid = false` + fatura aberta → badge âmbar "Estimado"

Fatura fechada = hoje >= `closing_day` do mês do `billing_month`.

---

## Arquivos afetados

### Backend (novos / modificados)
- `backend/internal/handlers/cashflow.go` — novo handler `CashflowHandler`
- `backend/internal/repository/transaction.go` — novo método `CashflowByMonth`
- `backend/cmd/server/main.go` — registrar rota `GET /cashflow`

### Frontend (novos / modificados)
- `frontend/src/pages/Dashboard.tsx` → `frontend/src/pages/Consumo.tsx`
- `frontend/src/pages/Caixa.tsx` — nova página
- `frontend/src/hooks/useDashboard.ts` — manter sem mudanças
- `frontend/src/hooks/useCashflow.ts` — novo hook
- `frontend/src/services/api.ts` — adicionar `getCashflow(year, month)`
- `frontend/src/components/BottomNav/BottomNav.tsx` — atualizar nav items
- `frontend/src/App.tsx` — atualizar rotas + redirect `/dashboard` → `/consumo`

---

## Fora de escopo

- Migração de dados existentes (nenhuma mudança no schema do banco)
- Modo desktop / `full-dashboard` — não afetado neste ciclo
- RAG context para chat — não afetado
