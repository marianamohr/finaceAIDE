# Projeção de Gastos — Página de Detalhe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Criar uma página `/projecao` com detalhe itemizado (fixos, parcelas, cartão) dos gastos projetados por mês, acessível via botão "Ver detalhes" no gráfico do Painel.

**Architecture:** Novo endpoint `GET /transactions/expense-projection/detail?months=6` retorna todos os meses com itens individuais em uma única requisição. Frontend consome esse endpoint via hook dedicado e renderiza uma página com abas por mês.

**Tech Stack:** Go (backend), React + TypeScript, Radix UI Tabs, react-router-dom

---

## Arquivos envolvidos

| Ação | Arquivo |
|------|---------|
| Modify | `backend/internal/repository/transaction.go` |
| Create | `backend/internal/repository/expense_projection_detail_integration_test.go` |
| Modify | `backend/internal/handlers/transaction.go` |
| Modify | `backend/cmd/server/main.go` |
| Modify | `frontend/src/hooks/useAnalytics.ts` |
| Create | `frontend/src/pages/ProjecaoDetalhe.tsx` |
| Modify | `frontend/src/App.tsx` |
| Modify | `frontend/src/components/analytics/AnalyticsDashboard.tsx` |

---

## Task 1: Backend — tipos e método de repositório

**Files:**
- Modify: `backend/internal/repository/transaction.go` (após linha 745, onde termina `ExpenseProjectionMonth`)
- Create: `backend/internal/repository/expense_projection_detail_integration_test.go`

### Passo 1.1 — Escrever o teste de integração (falha esperada)

Crie o arquivo `backend/internal/repository/expense_projection_detail_integration_test.go`:

```go
//go:build integration

package repository_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func projectionDetailDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := projectionDB(t)
	require.NoError(t, db.AutoMigrate(&models.InstallmentPurchase{}))
	return db
}

func TestExpenseProjectionDetailByMonths(t *testing.T) {
	db := projectionDetailDB(t)
	repo := repository.NewTransactionRepository(db)

	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	year, month := next.Year(), int(next.Month())

	t.Run("fixed expense appears in fixed items", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		mkRecurring(t, db, accountID, 150, nil, nil, nil)
		// override Name para validar
		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		m := got[0]
		assert.Equal(t, year, m.Year)
		assert.Equal(t, month, m.Month)
		assert.InDelta(t, 150.0, m.Fixed, 0.001)
		require.Len(t, m.Items.Fixed, 1)
		assert.InDelta(t, 150.0, m.Items.Fixed[0].Amount, 0.001)
		assert.Empty(t, m.Items.Installments)
		assert.Empty(t, m.Items.Card)
	})

	t.Run("installment tx appears with installment_info", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)

		// Cria InstallmentPurchase
		ip := &models.InstallmentPurchase{
			ID:              uuid.New(),
			AccountID:       accountID,
			PaymentMethodID: card.ID,
			Description:     "Notebook",
			Merchant:        "Notebook",
			TotalAmount:     3000,
			Installments:    10,
			PurchaseDate:    time.Now().UTC(),
		}
		require.NoError(t, db.Create(ip).Error)
		t.Cleanup(func() { db.Unscoped().Delete(ip) })

		num := 3
		tx := &models.Transaction{
			ID:                    uuid.New(),
			UserID:                uuid.New(),
			AccountID:             accountID,
			Amount:                -300,
			Category:              "outros",
			Date:                  time.Now().UTC(),
			Source:                "installment",
			Type:                  "regular",
			PaymentMethodID:       &card.ID,
			InstallmentPurchaseID: &ip.ID,
			InstallmentNumber:     &num,
			BillingYear:           &year,
			BillingMonth:          &month,
			Locale:                "BR",
			Currency:              "BRL",
			Merchant:              "Notebook",
		}
		require.NoError(t, db.Create(tx).Error)
		t.Cleanup(func() { db.Unscoped().Delete(tx) })

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		m := got[0]
		require.Len(t, m.Items.Installments, 1)
		item := m.Items.Installments[0]
		assert.Equal(t, "Notebook", item.Name)
		assert.InDelta(t, 300.0, item.Amount, 0.001)
		assert.Equal(t, "3/10", item.InstallmentInfo)
	})

	t.Run("virtual card subscription appears in card items", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
		mkRecurring(t, db, accountID, 45, &card.ID, nil, nil)

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		m := got[0]
		assert.Empty(t, m.Items.Fixed)
		assert.Empty(t, m.Items.Installments)
		require.Len(t, m.Items.Card, 1)
		assert.InDelta(t, 45.0, m.Items.Card[0].Amount, 0.001)
	})

	t.Run("totals match sum of items", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
		mkRecurring(t, db, accountID, 100, nil, nil, nil)           // fixed
		mkRecurring(t, db, accountID, 50, &card.ID, nil, nil)        // virtual card
		mkBillingTx(t, db, accountID, -200, &card.ID, "installment", "regular", year, month, nil)

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		m := got[0]
		itemsFixed := 0.0
		for _, it := range m.Items.Fixed { itemsFixed += it.Amount }
		itemsInst := 0.0
		for _, it := range m.Items.Installments { itemsInst += it.Amount }
		itemsCard := 0.0
		for _, it := range m.Items.Card { itemsCard += it.Amount }
		assert.InDelta(t, m.Fixed, itemsFixed, 0.001, "fixed total must match items sum")
		assert.InDelta(t, m.Installments, itemsInst, 0.001, "installments total must match items sum")
		assert.InDelta(t, m.Card, itemsCard, 0.001, "card total must match items sum")
	})
}
```

- [ ] **Passo 1.2 — Rodar o teste para confirmar que falha**

```bash
cd backend
go test -tags integration -run TestExpenseProjectionDetailByMonths ./internal/repository/ -v 2>&1 | tail -20
```

Esperado: `FAIL` com `undefined: repository.ExpenseProjectionDetailByMonths` ou similar.

- [ ] **Passo 1.3 — Adicionar os tipos em `backend/internal/repository/transaction.go`**

Logo após a definição de `ExpenseProjectionMonth` (linha ~745), adicione:

```go
// ProjectionItem is a single named expense in a projection bucket.
type ProjectionItem struct {
	Name            string  `json:"name"`
	Amount          float64 `json:"amount"`
	InstallmentInfo string  `json:"installment_info,omitempty"`
}

// ProjectionItems holds the itemized breakdown for each projection bucket.
type ProjectionItems struct {
	Fixed        []ProjectionItem `json:"fixed"`
	Installments []ProjectionItem `json:"installments"`
	Card         []ProjectionItem `json:"card"`
}

// ExpenseProjectionMonthDetail extends ExpenseProjectionMonth with per-item lists.
type ExpenseProjectionMonthDetail struct {
	ExpenseProjectionMonth
	Items ProjectionItems `json:"items"`
}
```

- [ ] **Passo 1.4 — Implementar `ExpenseProjectionDetailByMonths` em `backend/internal/repository/transaction.go`**

Adicione o método após `ExpenseProjectionByMonths`:

```go
// ExpenseProjectionDetailByMonths is like ExpenseProjectionByMonths but also
// returns the individual line items that make up each bucket's total.
func (r *TransactionRepository) ExpenseProjectionDetailByMonths(accountID uuid.UUID, n int) ([]ExpenseProjectionMonthDetail, error) {
	now := time.Now().UTC()
	result := make([]ExpenseProjectionMonthDetail, 0, n)

	for i := 1; i <= n; i++ {
		d := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		year, month := d.Year(), int(d.Month())

		// ---- totals (same queries as ExpenseProjectionByMonths) ----
		var cardRow struct {
			Installments float64
			Card         float64
		}
		if err := r.db.Raw(`
			SELECT
				COALESCE(-SUM(t.amount) FILTER (WHERE t.source =  'installment'), 0) AS installments,
				COALESCE(-SUM(t.amount) FILTER (WHERE t.source <> 'installment'), 0) AS card
			FROM transactions t
			JOIN payment_methods pm ON pm.id = t.payment_method_id
			WHERE t.account_id = ?
			  AND pm.kind = 'credit_card'
			  AND t.type != 'invoice_payment'
			  AND t.billing_year = ?
			  AND t.billing_month = ?
		`, accountID, year, month).Scan(&cardRow).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail cards total: %w", err)
		}

		var recRow struct {
			Fixed       float64
			CardVirtual float64
		}
		if err := r.db.Raw(`
			SELECT
				COALESCE(SUM(re.amount) FILTER (
					WHERE pm.id IS NULL OR pm.kind <> 'credit_card'
				), 0) AS fixed,
				COALESCE(SUM(re.amount) FILTER (
					WHERE pm.kind = 'credit_card'
					  AND NOT EXISTS (
						SELECT 1 FROM transactions tx
						WHERE tx.recurring_expense_id = re.id
						  AND tx.billing_year = ?
						  AND tx.billing_month = ?
					)
				), 0) AS card_virtual
			FROM recurring_expenses re
			LEFT JOIN payment_methods pm ON pm.id = re.payment_method_id
			WHERE re.account_id = ?
			  AND re.active = true
			  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
			  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
		`, year, month, accountID, year, month, year, month).Scan(&recRow).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail recurring total: %w", err)
		}

		card := cardRow.Card + recRow.CardVirtual
		label := fmt.Sprintf("%s/%02d", ptMonths[month], year%100)

		// ---- fixed items ----
		var fixedRows []struct {
			Name   string
			Amount float64
		}
		if err := r.db.Raw(`
			SELECT re.name, re.amount
			FROM recurring_expenses re
			LEFT JOIN payment_methods pm ON pm.id = re.payment_method_id
			WHERE re.account_id = ?
			  AND re.active = true
			  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
			  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
			  AND (pm.id IS NULL OR pm.kind <> 'credit_card')
			ORDER BY re.amount DESC
		`, accountID, year, month, year, month).Scan(&fixedRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail fixed items: %w", err)
		}

		// ---- installment items ----
		var installRows []struct {
			Name              string
			Amount            float64
			InstallmentNumber *int
			TotalInstallments *int
		}
		if err := r.db.Raw(`
			SELECT
				COALESCE(NULLIF(t.merchant, ''), t.description, 'Sem descrição') AS name,
				-t.amount AS amount,
				t.installment_number,
				ip.installments AS total_installments
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
		`, accountID, year, month).Scan(&installRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail installment items: %w", err)
		}

		// ---- card items: materialized ----
		var cardMatRows []struct {
			Name   string
			Amount float64
		}
		if err := r.db.Raw(`
			SELECT
				COALESCE(NULLIF(t.merchant, ''), t.description, 'Sem descrição') AS name,
				-t.amount AS amount
			FROM transactions t
			JOIN payment_methods pm ON pm.id = t.payment_method_id
			WHERE t.account_id = ?
			  AND pm.kind = 'credit_card'
			  AND t.source <> 'installment'
			  AND t.type != 'invoice_payment'
			  AND t.billing_year = ?
			  AND t.billing_month = ?
			ORDER BY t.amount ASC
		`, accountID, year, month).Scan(&cardMatRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail card materialized items: %w", err)
		}

		// ---- card items: virtual (subscriptions not yet materialized) ----
		var cardVirtRows []struct {
			Name   string
			Amount float64
		}
		if err := r.db.Raw(`
			SELECT re.name, re.amount
			FROM recurring_expenses re
			JOIN payment_methods pm ON pm.id = re.payment_method_id
			WHERE re.account_id = ?
			  AND re.active = true
			  AND pm.kind = 'credit_card'
			  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
			  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
			  AND NOT EXISTS (
				SELECT 1 FROM transactions tx
				WHERE tx.recurring_expense_id = re.id
				  AND tx.billing_year = ?
				  AND tx.billing_month = ?
			)
			ORDER BY re.amount DESC
		`, accountID, year, month, year, month, year, month).Scan(&cardVirtRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail card virtual items: %w", err)
		}

		// ---- assemble items ----
		fixedItems := make([]ProjectionItem, 0, len(fixedRows))
		for _, r := range fixedRows {
			fixedItems = append(fixedItems, ProjectionItem{Name: r.Name, Amount: r.Amount})
		}

		installItems := make([]ProjectionItem, 0, len(installRows))
		for _, r := range installRows {
			item := ProjectionItem{Name: r.Name, Amount: r.Amount}
			if r.InstallmentNumber != nil && r.TotalInstallments != nil {
				item.InstallmentInfo = fmt.Sprintf("%d/%d", *r.InstallmentNumber, *r.TotalInstallments)
			}
			installItems = append(installItems, item)
		}

		cardItems := make([]ProjectionItem, 0, len(cardMatRows)+len(cardVirtRows))
		for _, r := range cardMatRows {
			cardItems = append(cardItems, ProjectionItem{Name: r.Name, Amount: r.Amount})
		}
		for _, r := range cardVirtRows {
			cardItems = append(cardItems, ProjectionItem{Name: r.Name, Amount: r.Amount})
		}

		result = append(result, ExpenseProjectionMonthDetail{
			ExpenseProjectionMonth: ExpenseProjectionMonth{
				Year:         year,
				Month:        month,
				Label:        label,
				Fixed:        recRow.Fixed,
				Installments: cardRow.Installments,
				Card:         card,
				Total:        recRow.Fixed + cardRow.Installments + card,
			},
			Items: ProjectionItems{
				Fixed:        fixedItems,
				Installments: installItems,
				Card:         cardItems,
			},
		})
	}

	return result, nil
}
```

- [ ] **Passo 1.5 — Rodar os testes e confirmar que passam**

Primeiro crie o banco de teste se não existir:
```bash
docker exec -it agendaae-postgres-1 psql -U financeai -c "CREATE DATABASE financeai_test;" 2>/dev/null || true
```

Depois rode os testes:
```bash
cd backend
go test -tags integration -run TestExpenseProjectionDetailByMonths ./internal/repository/ -v 2>&1 | tail -30
```

Esperado: todos os subtests `PASS`.

- [ ] **Passo 1.6 — Commit**

```bash
git add backend/internal/repository/transaction.go \
        backend/internal/repository/expense_projection_detail_integration_test.go
git commit -m "feat(projection): tipos e método ExpenseProjectionDetailByMonths"
```

---

## Task 2: Backend — handler e rota

**Files:**
- Modify: `backend/internal/handlers/transaction.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Passo 2.1 — Adicionar handler em `backend/internal/handlers/transaction.go`**

Logo após o método `ExpenseProjection` (linha ~487), adicione:

```go
// GET /transactions/expense-projection/detail?months=6
// Returns projected spending for the next N months with itemized lists per bucket.
func (h *TransactionHandler) ExpenseProjectionDetail(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	months := 6
	if m := r.URL.Query().Get("months"); m != "" {
		fmt.Sscanf(m, "%d", &months) //nolint:errcheck
	}
	if months < 1 || months > 12 {
		months = 6
	}

	data, err := h.repo.ExpenseProjectionDetailByMonths(accountID, months)
	if err != nil {
		log.Error().Err(err).Msg("transaction: expense projection detail")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"months": data})
}
```

- [ ] **Passo 2.2 — Registrar a rota em `backend/cmd/server/main.go`**

Localize a linha:
```go
r.Get("/expense-projection", txHandler.ExpenseProjection)
```

Adicione logo abaixo:
```go
r.Get("/expense-projection/detail", txHandler.ExpenseProjectionDetail)
```

- [ ] **Passo 2.3 — Compilar para garantir que não há erros**

```bash
cd backend
go build ./... 2>&1
```

Esperado: nenhuma saída (sem erros).

- [ ] **Passo 2.4 — Commit**

```bash
git add backend/internal/handlers/transaction.go \
        backend/cmd/server/main.go
git commit -m "feat(projection): handler e rota GET /expense-projection/detail"
```

---

## Task 3: Frontend — hook

**Files:**
- Modify: `frontend/src/hooks/useAnalytics.ts`

- [ ] **Passo 3.1 — Adicionar tipos e hook em `frontend/src/hooks/useAnalytics.ts`**

Após a interface `ExpenseProjectionMonth` (linha ~71), adicione:

```ts
export interface ProjectionItem {
  name: string
  amount: number
  installment_info?: string
}

export interface ProjectionItems {
  fixed: ProjectionItem[]
  installments: ProjectionItem[]
  card: ProjectionItem[]
}

export interface ExpenseProjectionMonthDetail extends ExpenseProjectionMonth {
  items: ProjectionItems
}
```

Após o hook `useExpenseProjection` (linha ~84), adicione:

```ts
export function useExpenseProjectionDetail(months = 6) {
  const [data, setData] = useState<ExpenseProjectionMonthDetail[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<{ months: ExpenseProjectionMonthDetail[] }>(`/transactions/expense-projection/detail?months=${months}`)
      .then((d) => { setData(d?.months ?? []); setLoading(false) })
      .catch(() => { setData([]); setLoading(false) })
  }, [months])

  return { data, loading }
}
```

- [ ] **Passo 3.2 — Verificar tipos**

```bash
cd frontend
npx tsc --noEmit 2>&1 | head -20
```

Esperado: nenhum erro de tipo relacionado ao novo hook.

- [ ] **Passo 3.3 — Commit**

```bash
git add frontend/src/hooks/useAnalytics.ts
git commit -m "feat(projection): hook useExpenseProjectionDetail"
```

---

## Task 4: Frontend — página ProjecaoDetalhe e rota

**Files:**
- Create: `frontend/src/pages/ProjecaoDetalhe.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Passo 4.1 — Criar `frontend/src/pages/ProjecaoDetalhe.tsx`**

```tsx
import { useNavigate } from "react-router-dom"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { useExpenseProjectionDetail, ProjectionItem } from "@/hooks/useAnalytics"
import { useCurrencyStore, formatCurrency, maskCurrency } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"

function useFmt() {
  const { currency } = useCurrencyStore()
  const valuesHidden = usePrivacyStore((s) => s.valuesHidden)
  return (value: number) =>
    valuesHidden ? maskCurrency(value, currency) : formatCurrency(value, currency)
}

function ItemList({ items, color }: { items: ProjectionItem[]; color: string }) {
  const fmt = useFmt()
  if (items.length === 0) return null
  return (
    <div className="rounded-lg overflow-hidden border border-border">
      {items.map((item, i) => (
        <div
          key={i}
          className="flex items-center justify-between px-4 py-3 bg-card border-b border-border last:border-b-0"
        >
          <div>
            <p className="text-sm text-foreground">{item.name}</p>
            {item.installment_info && (
              <p className="text-xs text-muted-foreground">parcela {item.installment_info}</p>
            )}
          </div>
          <span className="text-sm font-semibold" style={{ color }}>
            {fmt(item.amount)}
          </span>
        </div>
      ))}
    </div>
  )
}

const COLOR_FIXED = "#6ee7b7"
const COLOR_INSTALLMENTS = "#c4b5fd"
const COLOR_CARD = "#67e8f9"

export default function ProjecaoDetalhe() {
  const navigate = useNavigate()
  const fmt = useFmt()
  const { data, loading } = useExpenseProjectionDetail(6)

  if (loading) {
    return (
      <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
        <div className="lg:hidden">
          <PageHeader title="Projeção de Gastos" showBack onBack={() => navigate("/painel")} />
        </div>
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm text-muted-foreground">Carregando...</p>
        </div>
        <BottomNav />
      </div>
    )
  }

  if (data.length === 0) {
    return (
      <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
        <div className="lg:hidden">
          <PageHeader title="Projeção de Gastos" showBack onBack={() => navigate("/painel")} />
        </div>
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm text-muted-foreground">Sem dados de projeção.</p>
        </div>
        <BottomNav />
      </div>
    )
  }

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <div className="lg:hidden">
        <PageHeader title="Projeção de Gastos" showBack onBack={() => navigate("/painel")} />
      </div>

      <div className="flex-1 overflow-y-auto">
        <Tabs defaultValue={data[0].label} className="w-full">
          <div className="sticky top-0 z-10 bg-background border-b border-border">
            <TabsList className="w-full h-auto justify-start rounded-none bg-transparent p-0 overflow-x-auto flex">
              {data.map((m) => (
                <TabsTrigger
                  key={m.label}
                  value={m.label}
                  className="shrink-0 rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-transparent px-4 py-3 text-sm"
                >
                  {m.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>

          {data.map((m) => (
            <TabsContent key={m.label} value={m.label} className="mt-0 p-4 space-y-4">
              {/* Summary cards */}
              <div className="grid grid-cols-3 gap-2">
                <div className="bg-card rounded-lg p-3 text-center border border-border">
                  <p className="text-xs text-muted-foreground uppercase mb-1">Fixos</p>
                  <p className="text-sm font-bold" style={{ color: COLOR_FIXED }}>{fmt(m.fixed)}</p>
                </div>
                <div className="bg-card rounded-lg p-3 text-center border border-border">
                  <p className="text-xs text-muted-foreground uppercase mb-1">Parcelas</p>
                  <p className="text-sm font-bold" style={{ color: COLOR_INSTALLMENTS }}>{fmt(m.installments)}</p>
                </div>
                <div className="bg-card rounded-lg p-3 text-center border border-border">
                  <p className="text-xs text-muted-foreground uppercase mb-1">Cartão</p>
                  <p className="text-sm font-bold" style={{ color: COLOR_CARD }}>{fmt(m.card)}</p>
                </div>
              </div>

              {/* Total row */}
              <div className="bg-card rounded-lg px-4 py-3 flex justify-between items-center border border-border">
                <p className="text-sm text-muted-foreground">Total projetado</p>
                <p className="text-base font-bold">{fmt(m.total)}</p>
              </div>

              {/* Fixed items */}
              {m.items.fixed.length > 0 && (
                <div className="space-y-2">
                  <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_FIXED }}>Fixos</p>
                  <ItemList items={m.items.fixed} color={COLOR_FIXED} />
                </div>
              )}

              {/* Installment items */}
              {m.items.installments.length > 0 && (
                <div className="space-y-2">
                  <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_INSTALLMENTS }}>Parcelas</p>
                  <ItemList items={m.items.installments} color={COLOR_INSTALLMENTS} />
                </div>
              )}

              {/* Card items */}
              {m.items.card.length > 0 && (
                <div className="space-y-2">
                  <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_CARD }}>Cartão</p>
                  <ItemList items={m.items.card} color={COLOR_CARD} />
                </div>
              )}
            </TabsContent>
          ))}
        </Tabs>
      </div>

      <BottomNav />
    </div>
  )
}
```

- [ ] **Passo 4.2 — Adicionar a rota em `frontend/src/App.tsx`**

Localize:
```tsx
import GastosFixosLembretes from "./pages/GastosFixosLembretes";
```

Adicione logo abaixo:
```tsx
import ProjecaoDetalhe from "./pages/ProjecaoDetalhe";
```

Localize:
```tsx
<Route path="/gastos-fixos/lembretes" element={<ProtectedRoute><AppShell><GastosFixosLembretes /></AppShell></ProtectedRoute>} />
```

Adicione logo abaixo:
```tsx
<Route path="/projecao" element={<ProtectedRoute><AppShell><ProjecaoDetalhe /></AppShell></ProtectedRoute>} />
```

- [ ] **Passo 4.3 — Verificar tipos**

```bash
cd frontend
npx tsc --noEmit 2>&1 | head -20
```

Esperado: sem erros de tipo.

- [ ] **Passo 4.4 — Commit**

```bash
git add frontend/src/pages/ProjecaoDetalhe.tsx \
        frontend/src/App.tsx
git commit -m "feat(projection): página /projecao com abas por mês"
```

---

## Task 5: Frontend — botão "Ver detalhes" no gráfico

**Files:**
- Modify: `frontend/src/components/analytics/AnalyticsDashboard.tsx`

- [ ] **Passo 5.1 — Adicionar import de `useNavigate` em `AnalyticsDashboard.tsx`**

Localize a linha 1:
```tsx
import { useState, useRef, useEffect } from "react"
```

Substitua por:
```tsx
import { useState, useRef, useEffect } from "react"
import { useNavigate } from "react-router-dom"
```

- [ ] **Passo 5.2 — Adicionar o botão em `ExpenseProjectionChart`**

Localize (dentro de `ExpenseProjectionChart`):
```tsx
function ExpenseProjectionChart({ data }: { data: ExpenseProjectionMonth[] }) {
  const fmt = useFmt()
```

Substitua por:
```tsx
function ExpenseProjectionChart({ data }: { data: ExpenseProjectionMonth[] }) {
  const fmt = useFmt()
  const navigate = useNavigate()
```

Localize o fechamento do `Card` no retorno do `ExpenseProjectionChart` (caso sem dados):
```tsx
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Sem gastos fixos, parcelas ou assinaturas de cartão para projetar.
          </p>
        </CardContent>
      </Card>
```

Não precisa de botão aqui (sem dados = sem motivo para navegar).

Localize o fechamento do `Card` no retorno principal do `ExpenseProjectionChart`:
```tsx
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}
```

Substitua por:
```tsx
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
```

- [ ] **Passo 5.3 — Verificar tipos e buildar**

```bash
cd frontend
npx tsc --noEmit 2>&1 | head -20
```

Esperado: sem erros.

- [ ] **Passo 5.4 — Testar manualmente no browser**

Com o backend e frontend rodando:
1. Acesse `/painel` — confirme que o gráfico de Projeção de Gastos tem o link "Ver detalhes →"
2. Clique em "Ver detalhes →" — deve navegar para `/projecao`
3. Na página `/projecao`: verifique que as abas aparecem com os meses projetados
4. Clique em cada aba — confirme que os itens de Fixos, Parcelas e Cartão aparecem
5. Clique em "← Painel" (botão de voltar no header) — deve retornar para `/painel`

- [ ] **Passo 5.5 — Commit**

```bash
git add frontend/src/components/analytics/AnalyticsDashboard.tsx
git commit -m "feat(projection): botão 'Ver detalhes' no gráfico de projeção"
```
