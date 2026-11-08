# Consumo e Caixa Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dividir o Dashboard atual em duas páginas separadas: "Consumo" (gastos por categoria pela data da compra) e "Caixa" (fluxo de caixa real com faturas de crédito agrupadas por vencimento).

**Architecture:** Backend ganha um novo endpoint `GET /cashflow` que retorna transações reais (pix/débito) separadas de faturas de crédito agrupadas por `billing_month` e posicionadas pelo `due_day`. Frontend renomeia `Dashboard.tsx` → `Consumo.tsx` (sem aba de payment methods) e cria `Caixa.tsx` nova com lógica de fluxo de caixa.

**Tech Stack:** Go (chi, GORM, PostgreSQL), React, TypeScript, TanStack Query, Tailwind CSS, shadcn/ui, lucide-react.

---

## File Map

### Backend — novos/modificados
| Arquivo | Ação | Responsabilidade |
|---------|------|-----------------|
| `backend/internal/repository/transaction.go` | Modificar | Adicionar `CashflowByMonth` |
| `backend/internal/handlers/cashflow.go` | Criar | Handler `GET /cashflow` |
| `backend/cmd/server/main.go` | Modificar | Registrar rota e instanciar handler |

### Frontend — novos/modificados
| Arquivo | Ação | Responsabilidade |
|---------|------|-----------------|
| `frontend/src/services/api.ts` | Modificar | Adicionar `getCashflow` e tipos |
| `frontend/src/hooks/useCashflow.ts` | Criar | `useCashflowQuery(year, month)` |
| `frontend/src/pages/Consumo.tsx` | Criar (de Dashboard.tsx) | Página de consumo por categoria |
| `frontend/src/pages/Caixa.tsx` | Criar | Página de fluxo de caixa |
| `frontend/src/components/BottomNav/BottomNav.tsx` | Modificar | Adicionar "Consumo" e "Caixa" na nav |
| `frontend/src/App.tsx` | Modificar | Rotas `/consumo`, `/caixa`, redirect `/dashboard` |

---

## Task 1: Método `CashflowByMonth` no repositório

**Files:**
- Modify: `backend/internal/repository/transaction.go`

### Tipos de retorno

```go
// CashflowTransaction representa uma transação real (não-crédito) ou um invoice_payment.
type CashflowTransaction struct {
    ID              string    `json:"id"`
    Amount          float64   `json:"amount"`
    Category        string    `json:"category"`
    Merchant        string    `json:"merchant"`
    Description     string    `json:"description"`
    Date            time.Time `json:"date"`
    PaymentMethodID *string   `json:"payment_method_id,omitempty"`
    Kind            string    `json:"kind"`
    Type            string    `json:"type"`
}

// CashflowInvoiceTransaction representa uma compra individual dentro de uma fatura.
type CashflowInvoiceTransaction struct {
    ID          string    `json:"id"`
    Amount      float64   `json:"amount"`
    Category    string    `json:"category"`
    Merchant    string    `json:"merchant"`
    Description string    `json:"description"`
    Date        time.Time `json:"date"`
}

// CashflowInvoice representa uma fatura de cartão de crédito agrupada.
type CashflowInvoice struct {
    PaymentMethodID string                       `json:"payment_method_id"`
    Name            string                       `json:"name"`
    Color           string                       `json:"color"`
    BillingYear     int                          `json:"billing_year"`
    BillingMonth    int                          `json:"billing_month"`
    DueDay          *int                         `json:"due_day,omitempty"`
    ClosingDay      *int                         `json:"closing_day,omitempty"`
    Total           float64                      `json:"total"`
    IsPaid          bool                         `json:"is_paid"`
    Transactions    []CashflowInvoiceTransaction `json:"transactions"`
}

// CashflowSummary agrega entradas e saídas reais.
type CashflowSummary struct {
    Income      float64 `json:"income"`
    RealOutflow float64 `json:"real_outflow"`
}

// CashflowResult é o retorno completo de CashflowByMonth.
type CashflowResult struct {
    Summary          CashflowSummary       `json:"summary"`
    RealTransactions []CashflowTransaction `json:"real_transactions"`
    Invoices         []CashflowInvoice     `json:"invoices"`
}
```

### Lógica do método

A fatura de um cartão aparece no mês em que o `due_day` cai. Se `due_day >= closing_day` (ex: fecha 1, vence 6 → ambos em junho), a fatura de `billing_month=junho` aparece em junho. Se `due_day < closing_day` (ex: fecha 26, vence 2 → vence no mês seguinte), a fatura de `billing_month=maio` aparece em junho.

```
due_month = billing_month se due_day >= closing_day
due_month = billing_month + 1 se due_day < closing_day
```

- [ ] **Step 1: Escrever os tipos acima no final de `transaction.go`** (após a linha `PaymentMethodSummary`)

Adicionar ao final de `backend/internal/repository/transaction.go`:

```go
// CashflowTransaction is a real (non-credit) transaction in the cash-flow view.
type CashflowTransaction struct {
	ID              string     `json:"id"`
	Amount          float64    `json:"amount"`
	Category        string     `json:"category"`
	Merchant        string     `json:"merchant"`
	Description     string     `json:"description"`
	Date            time.Time  `json:"date"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
	Kind            string     `json:"kind"`
	Type            string     `json:"type"`
}

// CashflowInvoiceTransaction is one purchase inside a credit-card invoice.
type CashflowInvoiceTransaction struct {
	ID          uuid.UUID `json:"id"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category"`
	Merchant    string    `json:"merchant"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
}

// CashflowInvoice is an aggregated credit-card invoice placed in the due month.
type CashflowInvoice struct {
	PaymentMethodID uuid.UUID                    `json:"payment_method_id"`
	Name            string                       `json:"name"`
	Color           string                       `json:"color"`
	BillingYear     int                          `json:"billing_year"`
	BillingMonth    int                          `json:"billing_month"`
	DueDay          *int                         `json:"due_day,omitempty"`
	ClosingDay      *int                         `json:"closing_day,omitempty"`
	Total           float64                      `json:"total"`
	IsPaid          bool                         `json:"is_paid"`
	Transactions    []CashflowInvoiceTransaction `json:"transactions"`
}

// CashflowSummary holds income and real outflow for a month.
type CashflowSummary struct {
	Income      float64 `json:"income"`
	RealOutflow float64 `json:"real_outflow"`
}

// CashflowResult is the full return of CashflowByMonth.
type CashflowResult struct {
	Summary          CashflowSummary       `json:"summary"`
	RealTransactions []CashflowTransaction `json:"real_transactions"`
	Invoices         []CashflowInvoice     `json:"invoices"`
}
```

- [ ] **Step 2: Implementar `CashflowByMonth`**

Adicionar o método ao final de `backend/internal/repository/transaction.go`, após os tipos:

```go
// CashflowByMonth returns real transactions (non-credit) and grouped credit invoices
// whose due date falls in the requested year/month.
func (r *TransactionRepository) CashflowByMonth(accountID uuid.UUID, year, month int) (*CashflowResult, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	// ── 1. Real transactions: non-credit payment methods + invoice_payments ──
	type rawTx struct {
		ID              uuid.UUID  `gorm:"column:id"`
		Amount          float64    `gorm:"column:amount"`
		Category        string     `gorm:"column:category"`
		Merchant        string     `gorm:"column:merchant"`
		Description     string     `gorm:"column:description"`
		Date            time.Time  `gorm:"column:date"`
		PaymentMethodID *uuid.UUID `gorm:"column:payment_method_id"`
		Kind            string     `gorm:"column:kind"`
		Type            string     `gorm:"column:type"`
	}
	var rawTxs []rawTx
	err := r.db.Raw(`
		SELECT t.id, t.amount, t.category, t.merchant, t.description, t.date,
		       t.payment_method_id, COALESCE(pm.kind, '') AS kind, t.type
		FROM transactions t
		LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND t.date >= ? AND t.date < ?
		  AND t.type != 'transfer'
		  AND (pm.kind IS NULL OR pm.kind != 'credit_card' OR t.type = 'invoice_payment')
		ORDER BY t.date DESC
	`, accountID, start, end).Scan(&rawTxs).Error
	if err != nil {
		return nil, fmt.Errorf("cashflow: real transactions: %w", err)
	}

	realTxs := make([]CashflowTransaction, len(rawTxs))
	var income, realOutflow float64
	for i, tx := range rawTxs {
		realTxs[i] = CashflowTransaction{
			ID:              tx.ID.String(),
			Amount:          tx.Amount,
			Category:        tx.Category,
			Merchant:        tx.Merchant,
			Description:     tx.Description,
			Date:            tx.Date,
			PaymentMethodID: tx.PaymentMethodID,
			Kind:            tx.Kind,
			Type:            tx.Type,
		}
		if tx.Amount > 0 && tx.Type != "invoice_payment" {
			income += tx.Amount
		} else if tx.Amount < 0 {
			realOutflow += tx.Amount
		}
	}

	// ── 2. Credit invoices whose due_month == requested month ──
	type pmRow struct {
		ID         uuid.UUID `gorm:"column:id"`
		Name       string    `gorm:"column:name"`
		Color      string    `gorm:"column:color"`
		ClosingDay *int      `gorm:"column:closing_day"`
		DueDay     *int      `gorm:"column:due_day"`
	}
	var pms []pmRow
	err = r.db.Raw(`
		SELECT id, name, color, closing_day, due_day
		FROM payment_methods
		WHERE account_id = ? AND kind = 'credit_card' AND active = true
	`, accountID).Scan(&pms).Error
	if err != nil {
		return nil, fmt.Errorf("cashflow: payment methods: %w", err)
	}

	var invoices []CashflowInvoice
	for _, pm := range pms {
		if pm.DueDay == nil || pm.ClosingDay == nil {
			continue
		}
		// Determine which billing months have their due_month == requested month.
		// due_month = billing_month if due_day >= closing_day, else billing_month+1.
		// So: billing_month = requested_month if due_day >= closing_day,
		//     billing_month = requested_month-1 if due_day < closing_day.
		var candidates []struct{ billingYear, billingMonth int }
		if *pm.DueDay >= *pm.ClosingDay {
			// fatura do próprio mês vence neste mês
			candidates = append(candidates, struct{ billingYear, billingMonth int }{year, month})
		} else {
			// fatura do mês anterior vence neste mês
			prev := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
			candidates = append(candidates, struct{ billingYear, billingMonth int }{prev.Year(), int(prev.Month())})
		}

		for _, c := range candidates {
			type invoiceTxRow struct {
				ID           uuid.UUID `gorm:"column:id"`
				Amount       float64   `gorm:"column:amount"`
				Category     string    `gorm:"column:category"`
				Merchant     string    `gorm:"column:merchant"`
				Description  string    `gorm:"column:description"`
				Date         time.Time `gorm:"column:date"`
			}
			var txRows []invoiceTxRow
			err = r.db.Raw(`
				SELECT id, amount, category, merchant, description, date
				FROM transactions
				WHERE account_id = ?
				  AND payment_method_id = ?
				  AND billing_year = ? AND billing_month = ?
				  AND type != 'invoice_payment'
				ORDER BY date DESC
			`, accountID, pm.ID, c.billingYear, c.billingMonth).Scan(&txRows).Error
			if err != nil {
				return nil, fmt.Errorf("cashflow: invoice transactions: %w", err)
			}
			if len(txRows) == 0 {
				continue
			}

			var total float64
			invTxs := make([]CashflowInvoiceTransaction, len(txRows))
			for i, tx := range txRows {
				invTxs[i] = CashflowInvoiceTransaction{
					ID:          tx.ID,
					Amount:      tx.Amount,
					Category:    tx.Category,
					Merchant:    tx.Merchant,
					Description: tx.Description,
					Date:        tx.Date,
				}
				total += tx.Amount
			}

			// Check if invoice is paid
			var count int64
			r.db.Model(&models.Transaction{}).
				Where("account_id = ? AND payment_method_id = ? AND type = 'invoice_payment' AND billing_year = ? AND billing_month = ?",
					accountID, pm.ID, c.billingYear, c.billingMonth).
				Count(&count)

			invoices = append(invoices, CashflowInvoice{
				PaymentMethodID: pm.ID,
				Name:            pm.Name,
				Color:           pm.Color,
				BillingYear:     c.billingYear,
				BillingMonth:    c.billingMonth,
				DueDay:          pm.DueDay,
				ClosingDay:      pm.ClosingDay,
				Total:           total,
				IsPaid:          count > 0,
				Transactions:    invTxs,
			})
		}
	}

	return &CashflowResult{
		Summary:          CashflowSummary{Income: income, RealOutflow: realOutflow},
		RealTransactions: realTxs,
		Invoices:         invoices,
	}, nil
}
```

- [ ] **Step 3: Verificar que o import de `models` já existe em `transaction.go`**

O arquivo já importa `"github.com/marianamohr/financas/internal/models"` — confirmar na linha de imports. Se não existir, adicionar.

- [ ] **Step 4: Compilar para verificar erros**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 5: Commit**

```bash
cd .
git add backend/internal/repository/transaction.go
git commit -m "feat(cashflow): add CashflowByMonth repository method"
```

---

## Task 2: Handler `GET /cashflow`

**Files:**
- Create: `backend/internal/handlers/cashflow.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Criar `backend/internal/handlers/cashflow.go`**

```go
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/repository"
)

type CashflowHandler struct {
	txRepo *repository.TransactionRepository
}

func NewCashflowHandler(txRepo *repository.TransactionRepository) *CashflowHandler {
	return &CashflowHandler{txRepo: txRepo}
}

// GET /cashflow?year=2026&month=6
func (h *CashflowHandler) Cashflow(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())
	if y := r.URL.Query().Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		}
	}
	if m := r.URL.Query().Get("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v >= 1 && v <= 12 {
			month = v
		}
	}

	result, err := h.txRepo.CashflowByMonth(accountID, year, month)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 2: Registrar handler em `main.go`**

Em `backend/cmd/server/main.go`, após a linha `dashboardHandler := handlers.NewDashboardHandler(...)`:

```go
cashflowHandler := handlers.NewCashflowHandler(txRepo)
```

Na seção de rotas protegidas, após `r.Get("/dashboard", dashboardHandler.Dashboard)`:

```go
r.Get("/cashflow", cashflowHandler.Cashflow)
```

- [ ] **Step 3: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 4: Testar manualmente o endpoint**

Inicie o servidor e teste:

```bash
curl -s -H "Authorization: Bearer <token>" \
  "http://localhost:8080/cashflow?year=2026&month=6" | jq .
```

Esperado: JSON com `summary`, `real_transactions` e `invoices`.

- [ ] **Step 5: Commit**

```bash
cd .
git add backend/internal/handlers/cashflow.go backend/cmd/server/main.go
git commit -m "feat(cashflow): add GET /cashflow endpoint"
```

---

## Task 3: Tipos e `getCashflow` no frontend

**Files:**
- Modify: `frontend/src/services/api.ts`
- Create: `frontend/src/hooks/useCashflow.ts`

- [ ] **Step 1: Adicionar tipos e método `getCashflow` em `api.ts`**

Após a interface `PaymentMethod` (linha ~171), adicionar:

```ts
export interface CashflowTransaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  payment_method_id?: string;
  kind: string;
  type: string;
}

export interface CashflowInvoiceTransaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
}

export interface CashflowInvoice {
  payment_method_id: string;
  name: string;
  color: string;
  billing_year: number;
  billing_month: number;
  due_day?: number;
  closing_day?: number;
  total: number;
  is_paid: boolean;
  transactions: CashflowInvoiceTransaction[];
}

export interface CashflowData {
  summary: {
    income: number;
    real_outflow: number;
  };
  real_transactions: CashflowTransaction[];
  invoices: CashflowInvoice[];
}
```

No objeto `api`, após `getDashboard`, adicionar:

```ts
getCashflow: (year: number, month: number) =>
  request<CashflowData>(`/cashflow?year=${year}&month=${month}`),
```

- [ ] **Step 2: Criar `frontend/src/hooks/useCashflow.ts`**

```ts
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, CashflowData } from "@/services/api";

export type { CashflowData, CashflowInvoice, CashflowTransaction, CashflowInvoiceTransaction } from "@/services/api";

export function useCashflowQuery(year: number, month: number) {
  return useQuery({
    queryKey: ["cashflow", year, month],
    queryFn: () => api.getCashflow(year, month),
    staleTime: 60 * 1000,
  });
}

export function useInvalidateCashflow() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: ["cashflow"] });
}
```

- [ ] **Step 3: Verificar tipos TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros relacionados aos novos tipos.

- [ ] **Step 4: Commit**

```bash
cd .
git add frontend/src/services/api.ts frontend/src/hooks/useCashflow.ts
git commit -m "feat(cashflow): add frontend types, getCashflow and useCashflowQuery"
```

---

## Task 4: Página `Consumo.tsx`

**Files:**
- Create: `frontend/src/pages/Consumo.tsx` (baseado em `Dashboard.tsx`, removendo aba payment methods)

A página Consumo é o Dashboard atual com as seguintes mudanças:
1. Remover o `activeTab` / toggle entre "Por categoria" e "Por forma de pagamento"
2. Remover toda a seção de `PaymentMethodSummaryCard`, `CardDetailDrawer`, `InvoicePaymentModal`
3. Substituir os 3 cards (Receitas/Despesas/Saldo) por 1 card único "Total consumido"
4. Manter: seletor de mês, top categorias com barras, lista de transações paginada, modais de edição/exclusão, balance transfer banner, lembretes de gastos fixos

- [ ] **Step 1: Copiar `Dashboard.tsx` para `Consumo.tsx`**

```bash
cp frontend/src/pages/Dashboard.tsx \
   frontend/src/pages/Consumo.tsx
```

- [ ] **Step 2: Editar `Consumo.tsx` — remover imports não usados**

Remover da lista de imports:
- `Sheet, SheetContent, SheetHeader, SheetTitle` (usado no `CardDetailDrawer`)
- `ArrowDownLeft, ArrowUpRight, Wallet` (usado nos 3 cards antigos — verificar se `Trash2, ChevronDown, ChevronUp, Pencil` ainda são usados)

Manter lucide-react: `Trash2, ChevronDown, ChevronUp, Pencil`.

- [ ] **Step 3: Remover componentes de payment methods de `Consumo.tsx`**

Remover inteiramente as funções/componentes:
- `PaymentMethodSummaryCard` (linhas ~79-151 em Dashboard.tsx)
- `InvoicePaymentModal` (linhas ~153-236)
- `CardDetailDrawer` (linhas ~238-369)

- [ ] **Step 4: Substituir os 3 summary cards por 1 card único**

Localizar o bloco `summaryCards` e o JSX que os renderiza. Substituir por:

```tsx
{/* Card único: total consumido */}
<div className="rounded-xl bg-card p-4 space-y-1">
  <p className="text-[10px] text-muted-foreground">Total consumido em {offsetToYearMonth(selectedOffset).label}</p>
  <p className="text-2xl font-bold font-heading tabular-nums">
    {fmt(Math.abs(summary?.expenses ?? 0))}
  </p>
  <p className="text-[10px] text-muted-foreground">
    débito + crédito pela data da compra
  </p>
</div>
```

- [ ] **Step 5: Remover estado e lógica de payment methods**

Remover:
- `const [activeTab, setActiveTab] = useState<"categories" | "payment-methods">("categories");`
- `const { paymentMethods, loading: loadingPaymentMethods } = usePaymentMethods();`
- `const [selectedCardPM, setSelectedCardPM] = useState<PaymentMethodSummary | null>(null);`
- `const [payingInvoicePM, setPayingInvoicePM] = useState<PaymentMethodSummary | null>(null);`
- `findInvoicePayment`, `findCarryover`, `handlePayInvoice`, `handleUnpayInvoice`
- `const paymentMethodSummary = dashData?.payment_method_summary ?? [];`
- Import de `usePaymentMethods`

- [ ] **Step 6: Remover o tab toggle e a seção de payment methods do JSX**

Remover o bloco `{/* Tab toggle */}` e o bloco condicional `{activeTab === "payment-methods" && ...}`. Manter apenas a seção de categorias.

- [ ] **Step 7: Remover modais de payment methods do JSX**

Remover `{selectedCardPM && ...}` e `{payingInvoicePM && ...}` do JSX.

- [ ] **Step 8: Verificar tipos TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 9: Commit**

```bash
cd .
git add frontend/src/pages/Consumo.tsx
git commit -m "feat(consumo): add Consumo page (Dashboard sem payment methods)"
```

---

## Task 5: Página `Caixa.tsx`

**Files:**
- Create: `frontend/src/pages/Caixa.tsx`

- [ ] **Step 1: Criar `frontend/src/pages/Caixa.tsx`**

```tsx
import { useState, useEffect } from "react";
import { BottomNav } from "@/components/BottomNav/BottomNav";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { cn } from "@/lib/utils";
import { useCashflowQuery, useInvalidateCashflow, CashflowInvoice } from "@/hooks/useCashflow";
import { usePaymentMethods } from "@/hooks/usePaymentMethods";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { ArrowDownLeft, ArrowUpRight, ChevronDown, ChevronUp } from "lucide-react";
import { useCurrencyStore, formatCurrency } from "@/store/currency";

function useFmt() {
  const { currency } = useCurrencyStore();
  return (value: number) => formatCurrency(value, currency);
}

function offsetToYearMonth(offset: number): { year: number; month: number; label: string } {
  const now = new Date();
  const d = new Date(now.getFullYear(), now.getMonth() + offset, 1);
  return {
    year: d.getFullYear(),
    month: d.getMonth() + 1,
    label: d.toLocaleDateString("pt-BR", { month: "short", year: "2-digit" }),
  };
}

const MONTHS_PT = ["Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
                   "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"];

interface InvoiceCardProps {
  invoice: CashflowInvoice;
  onPay: () => void;
  fmt: (v: number) => string;
}

function InvoiceCard({ invoice, onPay, fmt }: InvoiceCardProps) {
  const [expanded, setExpanded] = useState(false);
  const today = new Date();

  // Fatura fechada = hoje >= closing_day do mês do billing
  const closingDate = invoice.closing_day
    ? new Date(invoice.billing_year, invoice.billing_month - 1, invoice.closing_day)
    : null;
  const isClosed = closingDate ? today >= closingDate : true;

  const absTotal = Math.abs(invoice.total);

  return (
    <div>
      <div
        className="flex items-start gap-2 py-2 border-b border-border cursor-pointer"
        onClick={() => setExpanded((e) => !e)}
      >
        <div
          className="w-7 h-7 rounded-lg flex items-center justify-center shrink-0 mt-0.5"
          style={{ background: `${invoice.color}22` }}
        >
          <div className="w-3 h-3 rounded-full" style={{ background: invoice.color }} />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-xs font-semibold">
            {invoice.name} — Fatura {MONTHS_PT[invoice.billing_month - 1]}/{invoice.billing_year}
          </p>
          <p className="text-[10px] text-muted-foreground mt-0.5">
            {invoice.due_day
              ? <span className={cn(isClosed && !invoice.is_paid ? "text-destructive" : "text-warning")}>
                  Vence dia {invoice.due_day}
                </span>
              : null}
            {" · toque para ver compras"}
          </p>
          {!invoice.is_paid && !isClosed && (
            <span className="inline-flex items-center gap-1 text-[9px] text-warning bg-warning/10 rounded px-1.5 py-0.5 mt-1">
              📅 Estimado · fatura ainda aberta
            </span>
          )}
          {!invoice.is_paid && isClosed && (
            <span className="inline-flex items-center gap-1 text-[9px] text-destructive bg-destructive/10 rounded px-1.5 py-0.5 mt-1">
              Aguardando pagamento
            </span>
          )}
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          <span className={cn(
            "text-xs font-bold tabular-nums",
            invoice.is_paid ? "text-foreground" : isClosed ? "text-destructive" : "text-warning"
          )}>
            {invoice.is_paid ? "−" : ""}{fmt(absTotal)}
          </span>
          {invoice.is_paid ? (
            <span className="text-[9px] text-primary font-semibold">Pago ✓</span>
          ) : isClosed ? (
            <button
              onClick={(e) => { e.stopPropagation(); onPay(); }}
              className="text-[9px] bg-primary text-primary-foreground rounded px-2 py-0.5 font-semibold"
            >
              Pagar
            </button>
          ) : (
            <span className="text-[9px] text-warning bg-warning/10 rounded px-1.5 py-0.5">A vencer</span>
          )}
          {expanded ? <ChevronUp className="w-3 h-3 text-muted-foreground" /> : <ChevronDown className="w-3 h-3 text-muted-foreground" />}
        </div>
      </div>

      {expanded && (
        <div className="ml-9 bg-secondary rounded-lg px-2 py-1 mb-1">
          {invoice.transactions.map((tx) => (
            <div key={tx.id} className="flex justify-between items-center py-1 border-b border-border last:border-0">
              <div className="min-w-0">
                <p className="text-[11px] truncate">{tx.merchant || tx.description || tx.category}</p>
                <p className="text-[10px] text-muted-foreground capitalize">{tx.category}</p>
              </div>
              <span className="text-[11px] font-semibold tabular-nums ml-2 shrink-0">
                {fmt(Math.abs(tx.amount))}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default function Caixa() {
  const fmt = useFmt();
  const [selectedOffset, setSelectedOffset] = useState(0);
  const [minOffset, setMinOffset] = useState(-12);
  const [maxOffset, setMaxOffset] = useState(2);

  const { year: selectedYear, month: selectedMonth } = offsetToYearMonth(selectedOffset);
  const { data, isLoading } = useCashflowQuery(selectedYear, selectedMonth);
  const invalidate = useInvalidateCashflow();
  const { payInvoice } = usePaymentMethods();

  const [payingInvoice, setPayingInvoice] = useState<CashflowInvoice | null>(null);

  const summary = data?.summary;
  const realTxs = data?.real_transactions ?? [];
  const invoices = data?.invoices ?? [];

  const windowOffsets = [
    selectedOffset - 2, selectedOffset - 1, selectedOffset,
    selectedOffset + 1, selectedOffset + 2,
  ];

  const handlePayInvoice = async () => {
    if (!payingInvoice) return;
    await payInvoice(
      payingInvoice.payment_method_id,
      payingInvoice.billing_year,
      payingInvoice.billing_month,
      Math.abs(payingInvoice.total),
    );
    setPayingInvoice(null);
    invalidate();
  };

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Caixa" />

      {/* Month pills */}
      <div className="flex gap-1.5 px-4 pb-2">
        {windowOffsets.map((offset) => {
          const { year, month, label } = offsetToYearMonth(offset);
          const isSelected = offset === selectedOffset;
          const isLeftEdge = offset === windowOffsets[0];
          const isRightEdge = offset === windowOffsets[4];

          return (
            <button
              key={`${year}-${month}`}
              onClick={() => {
                if (isLeftEdge && offset <= minOffset) setMinOffset((p) => p - 1);
                if (isRightEdge && offset >= maxOffset) setMaxOffset((p) => p + 1);
                setSelectedOffset(offset);
              }}
              className={cn(
                "flex-1 flex flex-col items-center py-1 rounded-full text-xs font-medium transition-colors",
                isSelected
                  ? "bg-primary text-primary-foreground"
                  : "bg-secondary text-secondary-foreground hover:bg-secondary/80",
              )}
            >
              <span>{label}</span>
              {offset === 0 && !isSelected && (
                <span className="w-1 h-1 rounded-full bg-primary mt-0.5" />
              )}
            </button>
          );
        })}
      </div>

      <div className="flex-1 overflow-y-auto px-4 pb-4 space-y-4">
        {/* Summary cards */}
        <div className="grid grid-cols-2 gap-2">
          <div className="rounded-xl bg-card p-3 space-y-1">
            <div className="w-7 h-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center">
              <ArrowDownLeft className="w-3.5 h-3.5" />
            </div>
            <p className="text-[10px] text-muted-foreground">Entradas</p>
            <p className="text-sm font-semibold tabular-nums font-heading text-primary">
              {fmt(summary?.income ?? 0)}
            </p>
          </div>
          <div className="rounded-xl bg-card p-3 space-y-1">
            <div className="w-7 h-7 rounded-lg bg-destructive/10 text-destructive flex items-center justify-center">
              <ArrowUpRight className="w-3.5 h-3.5" />
            </div>
            <p className="text-[10px] text-muted-foreground">Saídas reais</p>
            <p className="text-sm font-semibold tabular-nums font-heading text-destructive">
              {fmt(Math.abs(summary?.real_outflow ?? 0))}
            </p>
          </div>
        </div>

        {/* Movimentações */}
        <div className="rounded-xl bg-card px-3 py-2">
          <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide mb-2">
            Movimentações
          </p>

          {isLoading && (
            <p className="text-xs text-muted-foreground text-center py-4">Carregando...</p>
          )}

          {/* Real transactions (pix/débito/receitas/invoice_payment) */}
          {realTxs.map((tx) => (
            <div key={tx.id} className="flex items-start gap-2 py-2 border-b border-border last:border-0">
              <div className={cn(
                "w-7 h-7 rounded-lg flex items-center justify-center text-sm shrink-0",
                tx.amount > 0 ? "bg-primary/10" : "bg-secondary",
              )}>
                {tx.amount > 0 ? "💰" : tx.kind === "pix" ? "💸" : "💳"}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-xs font-semibold">{tx.merchant || tx.description || tx.category}</p>
                <p className="text-[10px] text-muted-foreground mt-0.5 capitalize">
                  {new Date(tx.date).toLocaleDateString("pt-BR", { day: "2-digit", month: "short" })}
                  {tx.kind ? ` · ${tx.kind}` : ""}
                </p>
              </div>
              <span className={cn(
                "text-xs font-bold tabular-nums shrink-0",
                tx.amount > 0 ? "text-primary" : "text-destructive"
              )}>
                {tx.amount > 0 ? "+" : "−"}{fmt(Math.abs(tx.amount))}
              </span>
            </div>
          ))}

          {/* Divider antes das faturas */}
          {invoices.length > 0 && realTxs.length > 0 && (
            <div className="my-2 border-t border-dashed border-border" />
          )}

          {/* Faturas de crédito */}
          {invoices.map((inv) => (
            <InvoiceCard
              key={`${inv.payment_method_id}-${inv.billing_year}-${inv.billing_month}`}
              invoice={inv}
              onPay={() => setPayingInvoice(inv)}
              fmt={fmt}
            />
          ))}

          {!isLoading && realTxs.length === 0 && invoices.length === 0 && (
            <p className="text-xs text-muted-foreground text-center py-4">
              Nenhuma movimentação neste mês.
            </p>
          )}
        </div>
      </div>

      {/* Modal de pagamento de fatura */}
      {payingInvoice && (
        <Sheet open onOpenChange={(open) => { if (!open) setPayingInvoice(null); }}>
          <SheetContent side="bottom" className="rounded-t-2xl pb-8">
            <SheetHeader className="mb-4">
              <SheetTitle>Pagar fatura {payingInvoice.name}</SheetTitle>
            </SheetHeader>
            <p className="text-xs text-muted-foreground mb-4">
              Fatura {MONTHS_PT[payingInvoice.billing_month - 1]}/{payingInvoice.billing_year} —
              total {fmt(Math.abs(payingInvoice.total))}
            </p>
            <div className="flex gap-2">
              <Button variant="outline" className="flex-1" onClick={() => setPayingInvoice(null)}>
                Cancelar
              </Button>
              <Button className="flex-1" onClick={handlePayInvoice}>
                Confirmar pagamento
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      )}

      <BottomNav />
    </div>
  );
}
```

- [ ] **Step 2: Verificar tipos TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 3: Commit**

```bash
cd .
git add frontend/src/pages/Caixa.tsx
git commit -m "feat(caixa): add Caixa page with cashflow view"
```

---

## Task 6: Rotas e BottomNav

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/BottomNav/BottomNav.tsx`

- [ ] **Step 1: Atualizar `App.tsx`**

Substituir o import de Dashboard e a rota `/dashboard`:

```tsx
// Remover:
import Dashboard from "./pages/Dashboard";

// Adicionar:
import Consumo from "./pages/Consumo";
import Caixa from "./pages/Caixa";
```

Substituir as rotas:

```tsx
// Remover:
<Route path="/dashboard" element={<ProtectedRoute><AppShell><Dashboard /></AppShell></ProtectedRoute>} />

// Adicionar:
<Route path="/dashboard" element={<Navigate to="/consumo" replace />} />
<Route path="/consumo" element={<ProtectedRoute><AppShell><Consumo /></AppShell></ProtectedRoute>} />
<Route path="/caixa" element={<ProtectedRoute><AppShell><Caixa /></AppShell></ProtectedRoute>} />
```

Atualizar o prefetch em `AppBootstrap` — substituir queryKey `"dashboard"` para também pré-buscar a nova rota (manter o prefetch de dashboard pois Consumo ainda usa o mesmo endpoint):

```tsx
// Manter como está — getDashboard ainda serve Consumo
qc.prefetchQuery({
  queryKey: ["dashboard", year, month],
  queryFn: () => api.getDashboard(year, month),
  staleTime: 60 * 1000,
});
```

- [ ] **Step 2: Atualizar `BottomNav.tsx`**

Substituir a entrada do Dashboard por Consumo e adicionar Caixa:

```tsx
// Remover:
import { MessageSquare, BarChart3, BarChart2, Wrench, PiggyBank, Target, CalendarCheck } from "lucide-react";
// ...
{ path: "/dashboard", icon: BarChart3, label: "Dashboard" },

// Adicionar no import:
import { MessageSquare, BarChart3, Wallet, BarChart2, Wrench, PiggyBank, Target, CalendarCheck } from "lucide-react";

// Substituir no array navItems:
{ path: "/consumo", icon: BarChart3, label: "Consumo" },
{ path: "/caixa", icon: Wallet, label: "Caixa" },
```

- [ ] **Step 3: Verificar tipos TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 4: Build de verificação**

```bash
cd frontend && npm run build
```

Esperado: build sem erros.

- [ ] **Step 5: Commit**

```bash
cd .
git add frontend/src/App.tsx frontend/src/components/BottomNav/BottomNav.tsx
git commit -m "feat(nav): add /consumo and /caixa routes, redirect /dashboard"
```

---

## Task 7: Teste de integração manual

- [ ] **Step 1: Iniciar backend e frontend**

```bash
# Terminal 1
cd backend && go run ./cmd/server

# Terminal 2
cd frontend && npm run dev
```

- [ ] **Step 2: Verificar página Consumo**

1. Navegar para `/consumo`
2. Confirmar card único "Total consumido" no topo
3. Confirmar top 5 categorias com barras
4. Confirmar que não há aba "por forma de pagamento"
5. Confirmar que transações de crédito aparecem pelo mês da compra

- [ ] **Step 3: Verificar página Caixa**

1. Navegar para `/caixa`
2. Confirmar 2 cards: Entradas (verde) e Saídas reais (vermelho, sem crédito)
3. Confirmar que transações pix/débito aparecem na lista
4. Confirmar que faturas de crédito aparecem agrupadas com status correto
5. Clicar numa fatura → confirmar que expande com as compras
6. Clicar em "Pagar" → confirmar modal de confirmação
7. Após pagar → confirmar badge "Pago ✓" e fatura some de "Saídas reais" dos cards

- [ ] **Step 4: Verificar redirect**

Navegar para `/dashboard` → deve redirecionar para `/consumo` automaticamente.

- [ ] **Step 5: Verificar bottom nav**

- Nav mostra: Consumo · Caixa · Chat · Perfil
- Item ativo destacado em verde quando na página correspondente

---

## Self-Review

**Spec coverage:**

| Requisito | Tarefa |
|-----------|--------|
| Consumo filtra por `date`, exclui `invoice_payment` | Task 4 (usa `SummaryByMonth` existente que já exclui) |
| Caixa: pix/débito por `date` | Task 1 (`CashflowByMonth` query real_transactions) |
| Caixa: crédito agrupado por fatura no mês do vencimento | Task 1 (lógica due_month) |
| Caixa: `is_paid` via `invoice_payment` existente | Task 1 (count query) |
| Cards Consumo: total único | Task 4 |
| Cards Caixa: Entradas + Saídas reais (sem crédito) | Task 5 |
| Fatura expansível com compras individuais | Task 5 (`InvoiceCard`) |
| Badge: Estimado / Aguardando / Pago ✓ | Task 5 (`InvoiceCard` lógica isClosed) |
| Botão Pagar na fatura | Task 5 |
| Rotas `/consumo`, `/caixa`, redirect `/dashboard` | Task 6 |
| Bottom nav atualizada | Task 6 |
| Endpoint `GET /cashflow` | Task 2 |

Todos os requisitos cobertos.

**Placeholders:** nenhum encontrado.

**Consistência de tipos:**
- `CashflowInvoice.payment_method_id` é `string` no frontend e `uuid.UUID` no backend (serializado como string no JSON) — ✓
- `useCashflowQuery` usa `CashflowData` de `api.ts` — ✓
- `InvoiceCard` recebe `CashflowInvoice` de `useCashflow.ts` que re-exporta de `api.ts` — ✓
- `payInvoice` em `usePaymentMethods` recebe `(id: string, year: number, month: number, amount: number)` — compatível com `handlePayInvoice` em Task 5 — ✓
