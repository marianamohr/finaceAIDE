# Dashboard Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminar o delay de ~2s no Dashboard combinando 4 requests em 1 endpoint `/dashboard`, migrando para React Query com cache de 60s, prefetchando no boot e invalidando automaticamente após mutações.

**Architecture:** Backend recebe um novo handler `DashboardHandler` que agrega summary + transactions + balance_transfer + recurring_reminders em uma única query paralela. Frontend migra `useDashboard` para `useDashboardQuery` (React Query, staleTime 60s). O `AppBootstrap` prefetcha o dashboard logo após o token ser obtido. Mutações (delete, update, confirm, mark paid) invalidam a query key `["dashboard"]`.

**Tech Stack:** Go (chi, zerolog), React, TypeScript, TanStack React Query v5

---

## Arquivos

### Backend
- **Create:** `backend/internal/handlers/dashboard.go` — novo handler com `GET /dashboard`
- **Modify:** `backend/cmd/server/main.go` — instanciar `DashboardHandler` e registrar rota

### Frontend
- **Modify:** `frontend/src/hooks/useDashboard.ts` — adicionar `useDashboardQuery`, manter `useDashboard` e tipos exportados intactos
- **Modify:** `frontend/src/pages/Dashboard.tsx` — trocar `useDashboard` + fetches avulsos por `useDashboardQuery`
- **Modify:** `frontend/src/App.tsx` — prefetch no `AppBootstrap` após `ready`
- **Modify:** `frontend/src/components/ConfirmationCard/ConfirmationCard.tsx` — invalidar `["dashboard"]` junto com `["transactions"]`
- **Modify:** `frontend/src/components/RecurringRemindersPanel/RecurringRemindersPanel.tsx` — invalidar `["dashboard"]` em markPaid/unmarkPaid

---

## Task 1: Backend — `GET /dashboard` handler

**Files:**
- Create: `backend/internal/handlers/dashboard.go`

- [ ] **Step 1: Criar o handler**

```go
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type DashboardHandler struct {
	txRepo        *repository.TransactionRepository
	recurringRepo *repository.RecurringExpenseRepository
	catRepo       *repository.CategoryRepository
}

func NewDashboardHandler(
	txRepo *repository.TransactionRepository,
	recurringRepo *repository.RecurringExpenseRepository,
	catRepo *repository.CategoryRepository,
) *DashboardHandler {
	return &DashboardHandler{txRepo: txRepo, recurringRepo: recurringRepo, catRepo: catRepo}
}

// GET /dashboard?year=2026&month=5
func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
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

	type result[T any] struct {
		val T
		err error
	}

	summaryCh := make(chan result[any], 1)
	txCh := make(chan result[any], 1)
	btCh := make(chan result[any], 1)
	remCh := make(chan result[any], 1)

	go func() {
		s, err := h.txRepo.SummaryByMonth(accountID, year, month)
		if err != nil {
			summaryCh <- result[any]{err: err}
			return
		}
		summaryCh <- result[any]{val: map[string]any{
			"income":     s.Income,
			"expenses":   s.Expenses,
			"balance":    s.Balance,
			"categories": s.Categories,
			"month":      fmt.Sprintf("%d-%02d", year, month),
		}}
	}()

	go func() {
		txs, err := h.txRepo.ListByAccountMonth(accountID, year, month)
		txCh <- result[any]{val: txs, err: err}
	}()

	go func() {
		amount, err := h.txRepo.BalanceTransferPending(accountID, year, month)
		btCh <- result[any]{val: map[string]any{"amount": amount, "pending": amount > 0}, err: err}
	}()

	go func() {
		items, err := h.recurringRepo.ListByAccount(accountID)
		if err != nil {
			remCh <- result[any]{err: err}
			return
		}
		paidMap, err := h.recurringRepo.FindPaidInMonth(accountID, year, month)
		if err != nil {
			remCh <- result[any]{err: err}
			return
		}
		catIDs := make([]interface{ GetID() interface{} }, 0)
		_ = catIDs
		// batch fetch category slugs
		import_catIDs := make([]interface{}, 0, len(items))
		_ = import_catIDs
		// reuse same logic as RecurringExpenseHandler.Reminders
		var catUUIDs []interface{}
		_ = catUUIDs

		// inline the slug lookup
		slugByID := map[interface{}]string{}
		_ = slugByID

		reminders, err := h.buildReminders(accountID, year, month, items, paidMap)
		if err != nil {
			remCh <- result[any]{err: err}
			return
		}
		remCh <- result[any]{val: reminders}
	}()

	sumRes := <-summaryCh
	txRes := <-txCh
	btRes := <-btCh
	remRes := <-remCh

	for _, r := range []error{sumRes.err, txRes.err, btRes.err, remRes.err} {
		if r != nil {
			log.Error().Err(r).Msg("dashboard: aggregation error")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"summary":              sumRes.val,
		"transactions":         txRes.val,
		"balance_transfer":     btRes.val,
		"recurring_reminders":  remRes.val,
	})
}
```

Nota: o goroutine da recurring está incompleto acima. Substituir pela implementação correta no Step 2.

- [ ] **Step 2: Implementar o handler correto (sem código de rascunho)**

Criar o arquivo `backend/internal/handlers/dashboard.go` com este conteúdo final:

```go
package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type DashboardHandler struct {
	txRepo        *repository.TransactionRepository
	recurringRepo *repository.RecurringExpenseRepository
	catRepo       *repository.CategoryRepository
}

func NewDashboardHandler(
	txRepo *repository.TransactionRepository,
	recurringRepo *repository.RecurringExpenseRepository,
	catRepo *repository.CategoryRepository,
) *DashboardHandler {
	return &DashboardHandler{txRepo: txRepo, recurringRepo: recurringRepo, catRepo: catRepo}
}

// GET /dashboard?year=2026&month=5
// Returns summary, transactions, balance_transfer and recurring_reminders in one response.
func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
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

	type anyResult struct {
		val any
		err error
	}

	summaryCh := make(chan anyResult, 1)
	txCh := make(chan anyResult, 1)
	btCh := make(chan anyResult, 1)
	remCh := make(chan anyResult, 1)

	go func() {
		s, err := h.txRepo.SummaryByMonth(accountID, year, month)
		if err != nil {
			summaryCh <- anyResult{err: err}
			return
		}
		summaryCh <- anyResult{val: map[string]any{
			"income":     s.Income,
			"expenses":   s.Expenses,
			"balance":    s.Balance,
			"categories": s.Categories,
			"month":      fmt.Sprintf("%d-%02d", year, month),
		}}
	}()

	go func() {
		txs, err := h.txRepo.ListByAccountMonth(accountID, year, month)
		txCh <- anyResult{val: txs, err: err}
	}()

	go func() {
		amount, err := h.txRepo.BalanceTransferPending(accountID, year, month)
		btCh <- anyResult{val: map[string]any{"amount": amount, "pending": amount > 0}, err: err}
	}()

	go func() {
		items, err := h.recurringRepo.ListByAccount(accountID)
		if err != nil {
			remCh <- anyResult{err: err}
			return
		}
		paidMap, err := h.recurringRepo.FindPaidInMonth(accountID, year, month)
		if err != nil {
			remCh <- anyResult{err: err}
			return
		}

		catIDs := make([]uuid.UUID, 0, len(items))
		for _, item := range items {
			if item.CategoryID != nil {
				catIDs = append(catIDs, *item.CategoryID)
			}
		}
		slugByID := make(map[uuid.UUID]string, len(catIDs))
		if len(catIDs) > 0 {
			cats, err := h.catRepo.FindByIDs(catIDs)
			if err != nil {
				remCh <- anyResult{err: err}
				return
			}
			for _, c := range cats {
				slugByID[c.ID] = c.Slug
			}
		}

		result := make([]models.RecurringExpenseWithStatus, 0, len(items))
		for _, item := range items {
			status := models.RecurringExpenseWithStatus{
				ID:         item.ID,
				Name:       item.Name,
				Amount:     item.Amount,
				CategoryID: item.CategoryID,
				DueDay:     item.DueDay,
			}
			if item.CategoryID != nil {
				status.CategorySlug = slugByID[*item.CategoryID]
			}
			if tx, paid := paidMap[item.ID]; paid {
				status.Paid = true
				status.TransactionID = &tx.ID
			}
			result = append(result, status)
		}

		sort.Slice(result, func(i, j int) bool {
			if result[i].Paid != result[j].Paid {
				return !result[i].Paid
			}
			di, dj := result[i].DueDay, result[j].DueDay
			if di == nil && dj == nil {
				return result[i].Name < result[j].Name
			}
			if di == nil {
				return false
			}
			if dj == nil {
				return true
			}
			if *di != *dj {
				return *di < *dj
			}
			return result[i].Name < result[j].Name
		})

		remCh <- anyResult{val: result}
	}()

	sumRes := <-summaryCh
	txRes := <-txCh
	btRes := <-btCh
	remRes := <-remCh

	for _, res := range []anyResult{sumRes, txRes, btRes, remRes} {
		if res.err != nil {
			log.Error().Err(res.err).Msg("dashboard: aggregation error")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"summary":             sumRes.val,
		"transactions":        txRes.val,
		"balance_transfer":    btRes.val,
		"recurring_reminders": remRes.val,
	})
}
```

- [ ] **Step 3: Registrar rota e instanciar handler em main.go**

Em `backend/cmd/server/main.go`, após a linha que instancia `recurringHandler` (~linha 202), adicionar:

```go
dashboardHandler := handlers.NewDashboardHandler(txRepo, recurringRepo, catRepo)
```

Dentro do grupo protegido (após `r.Use(jwtManager.Middleware)`), adicionar a rota (pode ser antes de `/transactions`):

```go
r.Get("/dashboard", dashboardHandler.Dashboard)
```

- [ ] **Step 4: Compilar o backend**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 5: Testar o endpoint manualmente (opcional — requer servidor rodando)**

```bash
# Trocar TOKEN por um token válido
curl -s "http://localhost:8080/dashboard?year=2026&month=5" \
  -H "Authorization: Bearer TOKEN" | jq 'keys'
```

Esperado: `["balance_transfer", "recurring_reminders", "summary", "transactions"]`

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/dashboard.go backend/cmd/server/main.go
git commit -m "feat(backend): add GET /dashboard aggregation endpoint

Returns summary, transactions, balance_transfer and recurring_reminders
in a single response using parallel goroutines."
```

---

## Task 2: Frontend — hook `useDashboardQuery`

**Files:**
- Modify: `frontend/src/hooks/useDashboard.ts`
- Modify: `frontend/src/services/api.ts`

- [ ] **Step 1: Adicionar tipo `DashboardData` e função `api.getDashboard` em `api.ts`**

Em `frontend/src/services/api.ts`, após os imports existentes, adicionar o tipo e a função ao objeto `api`:

Adicionar import no topo se ainda não existir (os tipos `DashboardSummary`, `Transaction` e `RecurringExpenseWithStatus` já existem em outros lugares — criar tipos locais no hook é mais simples):

No objeto `api` (ao final, antes do fechamento `}`), adicionar:

```typescript
getDashboard: (year: number, month: number) =>
  request<{
    summary: {
      income: number;
      expenses: number;
      balance: number;
      categories: Record<string, number>;
      month: string;
    };
    transactions: Array<{
      id: string;
      amount: number;
      category: string;
      merchant: string;
      description: string;
      date: string;
      source: string;
      image_url: string;
    }>;
    balance_transfer: { amount: number; pending: boolean };
    recurring_reminders: Array<{
      id: string;
      name: string;
      amount: number;
      category_id?: string;
      category_slug?: string;
      due_day?: number;
      paid: boolean;
      transaction_id?: string;
    }>;
  }>(`/dashboard?year=${year}&month=${month}`),
```

- [ ] **Step 2: Adicionar `useDashboardQuery` em `useDashboard.ts`**

Em `frontend/src/hooks/useDashboard.ts`, adicionar no topo:

```typescript
import { useQuery, useQueryClient } from "@tanstack/react-query";
```

Ao final do arquivo (após `usePrefetchNextMonth`), adicionar:

```typescript
export interface DashboardData {
  summary: DashboardSummary;
  transactions: Transaction[];
  balanceTransfer: { amount: number; pending: boolean };
  recurringReminders: Array<{
    id: string;
    name: string;
    amount: number;
    category_id?: string;
    category_slug?: string;
    due_day?: number;
    paid: boolean;
    transaction_id?: string;
  }>;
}

export function useDashboardQuery(year: number, month: number) {
  return useQuery({
    queryKey: ["dashboard", year, month],
    queryFn: async () => {
      const data = await api.getDashboard(year, month);
      return {
        summary: data.summary,
        transactions: data.transactions,
        balanceTransfer: data.balance_transfer,
        recurringReminders: data.recurring_reminders,
      } satisfies DashboardData;
    },
    staleTime: 60 * 1000,
  });
}

export function useInvalidateDashboard() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: ["dashboard"] });
}
```

- [ ] **Step 3: Verificar TypeScript**

```bash
cd frontend && node_modules/.bin/tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/hooks/useDashboard.ts frontend/src/services/api.ts
git commit -m "feat(frontend): add useDashboardQuery hook and getDashboard api call"
```

---

## Task 3: Frontend — migrar Dashboard.tsx para `useDashboardQuery`

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

- [ ] **Step 1: Trocar imports e hooks de fetching**

Substituir a linha de import do hook:

```typescript
// Antes:
import { useDashboard, usePrefetchNextMonth, Transaction } from "@/hooks/useDashboard";

// Depois:
import { useDashboardQuery, useInvalidateDashboard, usePrefetchNextMonth, Transaction } from "@/hooks/useDashboard";
```

- [ ] **Step 2: Substituir useDashboard e fetches avulsos no corpo do componente**

Remover as linhas 88-99 (useDashboard + balanceTransfer useEffect) e substituir por:

```typescript
const invalidateDashboard = useInvalidateDashboard();
const { data: dashData, isLoading: loading } = useDashboardQuery(selectedYear, selectedMonth);

const summary = dashData?.summary ?? null;
const transactions = dashData?.transactions ?? [];
const balanceTransfer = dashData?.balanceTransfer?.pending ? dashData.balanceTransfer : null;
```

Remover também o `useState` de `balanceTransfer` (linha 85) e o `useState` de `confirmingTransfer` não é afetado.

- [ ] **Step 3: Substituir deleteTransaction e updateTransaction**

As funções `deleteTransaction` e `updateTransaction` vinham do `useDashboard`. Agora devem chamar a API diretamente e invalidar o cache. Adicionar estas funções dentro do componente `Dashboard`:

```typescript
const deleteTransaction = async (id: string) => {
  await api.delete(`/transactions/${id}`);
  invalidateDashboard();
};

const updateTransaction = async (id: string, data: Partial<Omit<Transaction, "id" | "source" | "image_url">>) => {
  await api.put(`/transactions/${id}`, {
    amount: data.amount,
    category: data.category,
    merchant: data.merchant,
    description: data.description,
    date: data.date ? data.date.slice(0, 10) : undefined,
    source: "manual",
  });
  invalidateDashboard();
};
```

- [ ] **Step 4: Passar `recurringReminders` para RecurringRemindersPanel**

O painel de lembretes agora recebe os dados do dashboard em vez de fazer o próprio fetch. Verificar a assinatura atual de `RecurringRemindersPanel` e atualizar a chamada:

```tsx
// Antes (em Dashboard.tsx):
{selectedOffset === 0 && (
  <RecurringRemindersPanel year={selectedYear} month={selectedMonth} />
)}

// Depois:
{selectedOffset === 0 && (
  <RecurringRemindersPanel
    year={selectedYear}
    month={selectedMonth}
    initialData={dashData?.recurringReminders}
  />
)}
```

**Nota:** O `RecurringRemindersPanel` precisará aceitar `initialData` — isso é feito na Task 5.

- [ ] **Step 5: Verificar TypeScript**

```bash
cd frontend && node_modules/.bin/tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "feat(frontend): migrate Dashboard to useDashboardQuery"
```

---

## Task 4: Frontend — prefetch no boot

**Files:**
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Adicionar prefetch em AppBootstrap**

Em `frontend/src/App.tsx`, importar `useQueryClient`:

```typescript
import { useQueryClient } from "@tanstack/react-query";
```

No `AppBootstrap`, após o `useEffect` do `getCurrency`, adicionar:

```typescript
const qc = useQueryClient();

useEffect(() => {
  if (!isAuthenticated || !ready) return;
  const now = new Date();
  const year = now.getFullYear();
  const month = now.getMonth() + 1;
  qc.prefetchQuery({
    queryKey: ["dashboard", year, month],
    queryFn: () => api.getDashboard(year, month),
    staleTime: 60 * 1000,
  });
}, [isAuthenticated, ready, qc]);
```

- [ ] **Step 2: Verificar TypeScript**

```bash
cd frontend && node_modules/.bin/tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(frontend): prefetch dashboard data on auth boot"
```

---

## Task 5: Frontend — invalidar dashboard após ConfirmationCard e RecurringRemindersPanel

**Files:**
- Modify: `frontend/src/components/ConfirmationCard/ConfirmationCard.tsx`
- Modify: `frontend/src/components/RecurringRemindersPanel/RecurringRemindersPanel.tsx`

- [ ] **Step 1: Ler RecurringRemindersPanel para entender a assinatura atual**

```bash
cat frontend/src/components/RecurringRemindersPanel/RecurringRemindersPanel.tsx
```

- [ ] **Step 2: Adicionar invalidação em ConfirmationCard.tsx**

Encontrar onde `queryClient.invalidateQueries({ queryKey: ["transactions"] })` é chamado e adicionar logo depois:

```typescript
queryClient.invalidateQueries({ queryKey: ["dashboard"] })
```

- [ ] **Step 3: Atualizar RecurringRemindersPanel para aceitar `initialData` e invalidar cache**

Adicionar prop `initialData` ao componente. Se `initialData` for fornecido, usar esses dados como estado inicial (evita fetch duplicado). Em `markPaid` e `unmarkPaid`, além do dispatch do evento existente, chamar `invalidateDashboard()`.

Importar no topo:

```typescript
import { useInvalidateDashboard } from "@/hooks/useDashboard";
```

Adicionar `initialData` à prop e usar `useInvalidateDashboard`:

```typescript
interface RecurringRemindersPanelProps {
  year: number;
  month: number;
  initialData?: Array<{
    id: string;
    name: string;
    amount: number;
    category_id?: string;
    category_slug?: string;
    due_day?: number;
    paid: boolean;
    transaction_id?: string;
  }>;
}
```

Dentro do hook de estado (ou useRecurringReminders), inicializar com `initialData` quando disponível e chamar `invalidateDashboard()` após cada mutação.

- [ ] **Step 4: Verificar TypeScript**

```bash
cd frontend && node_modules/.bin/tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/ConfirmationCard/ConfirmationCard.tsx \
        frontend/src/components/RecurringRemindersPanel/RecurringRemindersPanel.tsx
git commit -m "feat(frontend): invalidate dashboard cache after mutations"
```

---

## Verificação end-to-end

1. **Boot do PWA**: Abrir o app. No Network, a sequência deve ser:
   - `refresh (200)` → `currency (200)` → `dashboard (200)` (um único request)
   - Ao navegar para Dashboard: **sem novos requests** se < 60s desde o boot (dados do cache)

2. **Criação de transação via chat**: Confirmar uma transação pelo `ConfirmationCard`. Dashboard deve atualizar automaticamente sem reload.

3. **Deletar/editar transação no Dashboard**: A lista deve atualizar após a operação.

4. **Marcar gasto fixo como pago**: O painel de lembretes deve atualizar e o dashboard deve refletir a nova transação.

5. **Trocar de mês**: Cada mês faz um request separado e é cacheado independentemente por `["dashboard", year, month]`.

6. **Segundo acesso ao mesmo mês dentro de 60s**: Zero requests ao backend — dados do React Query cache.
