# VR / Voucher — Saldo e Confirmação de Depósito — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Isolar o saldo do VR da conta geral, criar fluxo de confirmação manual de depósito mensal e exibir card dedicado do VR no Caixa.

**Architecture:** Novo `source = 'voucher_deposit'` nas transações identifica depósitos de voucher; campo `voucher_payment_method_id` guarda a referência ao payment method. Saldo calculado on-the-fly no backend. Dashboard retorna lista de vouchers com saldo e gasto. Banner no Caixa e card dedicado no frontend espelham o padrão balance transfer já existente.

**Tech Stack:** Go (chi, GORM, testify), React + TypeScript (TanStack Query, Tailwind, shadcn/ui)

---

## Mapa de arquivos

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `backend/internal/models/transaction.go` | Modificar | Adicionar campo `VoucherPaymentMethodID` e atualizar comentário de Source |
| `backend/migrations/migrate.go` | Modificar | Adicionar migração `migrateVoucherDeposits` |
| `backend/internal/repository/transaction.go` | Modificar | Adicionar `VoucherDepositsPending` e `VoucherSummary`; excluir voucher_deposit do saldo da conta |
| `backend/internal/handlers/transaction.go` | Modificar | Adicionar `VoucherDepositPending` e `ConfirmVoucherDeposit` handlers |
| `backend/internal/handlers/dashboard.go` | Modificar | Incluir `voucher_summary` na resposta do dashboard |
| `backend/cmd/server/main.go` | Modificar | Registrar novas rotas |
| `frontend/src/hooks/useDashboard.ts` | Modificar | Adicionar `voucherSummary` ao `DashboardData` |
| `frontend/src/services/api.ts` | Modificar | Adicionar `confirmVoucherDeposit` |
| `frontend/src/pages/Caixa.tsx` | Modificar | Banner de confirmação + card do VR |

---

## Task 1: Modelo — adicionar `VoucherPaymentMethodID` em Transaction

**Files:**
- Modify: `backend/internal/models/transaction.go`

- [ ] **Step 1: Atualizar o modelo**

Em `backend/internal/models/transaction.go`, atualizar o comentário de Source e adicionar o novo campo logo após `PaymentMethod`:

```go
// Source values: "manual" | "chat" | "photo" | "recurring" | "balance_transfer" | "voucher_deposit"
type Transaction struct {
    // ... campos existentes sem alteração ...

    PaymentMethodID  *uuid.UUID     `gorm:"type:uuid;index"                         json:"payment_method_id,omitempty"`
    PaymentMethod    *PaymentMethod `gorm:"foreignKey:PaymentMethodID;references:ID" json:"-"`

    // VoucherPaymentMethodID links a voucher_deposit transaction to its voucher payment method.
    VoucherPaymentMethodID *uuid.UUID `gorm:"type:uuid;index" json:"voucher_payment_method_id,omitempty"`

    BillingYear      *int           `                                                json:"billing_year,omitempty"`
    // ... restante sem alteração ...
}
```

- [ ] **Step 2: Verificar que compila**

```bash
cd backend && go build ./...
```
Expected: sem erros.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/transaction.go
git commit -m "feat(models): add VoucherPaymentMethodID to Transaction"
```

---

## Task 2: Migration — criar coluna e migrar dados existentes

**Files:**
- Modify: `backend/migrations/migrate.go`

- [ ] **Step 1: Adicionar função `migrateVoucherDeposits` no final do arquivo**

```go
// migrateVoucherDeposits adds the voucher_payment_method_id column and converts
// existing positive-amount transactions on voucher payment methods to voucher_deposit source.
func migrateVoucherDeposits(db *gorm.DB) error {
    // AutoMigrate já adiciona a coluna via GORM — só precisamos do backfill de dados.
    // Converte entradas antigas: amount > 0 com payment_method_id de um voucher.
    sql := `
        UPDATE transactions t
        SET
            source                     = 'voucher_deposit',
            voucher_payment_method_id  = t.payment_method_id,
            payment_method_id          = NULL
        WHERE t.payment_method_id IN (
            SELECT id FROM payment_methods WHERE kind = 'voucher'
        )
        AND t.amount > 0
        AND t.source != 'voucher_deposit'
    `
    if err := db.Exec(sql).Error; err != nil {
        return fmt.Errorf("migrate voucher deposits: %w", err)
    }
    return nil
}
```

- [ ] **Step 2: Chamar a função no fluxo de `Run`**

Localizar onde `migratePaymentMethods` e `migratePhase2` são chamados e adicionar logo depois:

```go
if err := migrateVoucherDeposits(db); err != nil {
    return fmt.Errorf("migrations: migrate voucher deposits: %w", err)
}
```

- [ ] **Step 3: Verificar que compila**

```bash
cd backend && go build ./...
```

- [ ] **Step 4: Rodar a migração localmente**

```bash
cd backend && go run ./cmd/server/main.go &
sleep 2 && kill %1
```

Verificar no banco:

```bash
docker exec financas-postgres-1 bash -c "psql -U financeai -d financeai -c \"
SELECT source, voucher_payment_method_id, payment_method_id, amount, description
FROM transactions
WHERE source = 'voucher_deposit'
LIMIT 5;
\""
```

Expected: rows com `source=voucher_deposit`, `voucher_payment_method_id` preenchido, `payment_method_id=NULL`.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/migrate.go
git commit -m "feat(migrations): add voucher_deposit source and backfill existing VR entries"
```

---

## Task 3: Repository — excluir voucher_deposit do saldo da conta; adicionar queries de voucher

**Files:**
- Modify: `backend/internal/repository/transaction.go`

- [ ] **Step 1: Escrever o teste para `VoucherSummary`**

Criar arquivo `backend/internal/repository/voucher_test.go`:

```go
package repository_test

import (
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/marianamohr/financas/internal/models"
    "github.com/marianamohr/financas/internal/repository"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestVoucherSummary(t *testing.T) {
    db := setupTestDB(t) // reutiliza helper existente
    txRepo := repository.NewTransactionRepository(db)

    accountID := uuid.New()
    pmID := uuid.New()

    // Insere um payment method voucher
    require.NoError(t, db.Exec(`
        INSERT INTO payment_methods (id, account_id, name, kind, color)
        VALUES (?, ?, 'VR Test', 'voucher', '#000')
    `, pmID, accountID).Error)

    // Depósito: voucher_deposit
    require.NoError(t, db.Create(&models.Transaction{
        AccountID:              accountID,
        UserID:                 uuid.New(),
        Amount:                 650,
        Category:               "vr",
        Description:            "Vale Refeição",
        Date:                   time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC),
        Source:                 "voucher_deposit",
        Type:                   "regular",
        VoucherPaymentMethodID: &pmID,
    }).Error)

    // Despesa no voucher
    require.NoError(t, db.Create(&models.Transaction{
        AccountID:       accountID,
        UserID:          uuid.New(),
        Amount:          -100,
        Category:        "alimentacao",
        Description:     "Almoço",
        Date:            time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
        Source:          "manual",
        Type:            "regular",
        PaymentMethodID: &pmID,
    }).Error)

    summaries, err := txRepo.VoucherSummary(accountID, 2026, 5)
    require.NoError(t, err)
    require.Len(t, summaries, 1)

    s := summaries[0]
    assert.Equal(t, pmID, s.PaymentMethodID)
    assert.Equal(t, "VR Test", s.Name)
    assert.InDelta(t, 550.0, s.Balance, 0.01)   // 650 - 100
    assert.InDelta(t, 100.0, s.MonthSpent, 0.01)
    assert.InDelta(t, 650.0, s.LastDeposit, 0.01)
}

func TestVoucherDepositPending(t *testing.T) {
    db := setupTestDB(t)
    txRepo := repository.NewTransactionRepository(db)

    accountID := uuid.New()
    pmID := uuid.New()

    require.NoError(t, db.Exec(`
        INSERT INTO payment_methods (id, account_id, name, kind, color)
        VALUES (?, ?, 'VR Test', 'voucher', '#000')
    `, pmID, accountID).Error)

    // Sem depósito em maio → deve retornar o pm como pendente
    pending, err := txRepo.VoucherDepositsPending(accountID, 2026, 5)
    require.NoError(t, err)
    require.Len(t, pending, 1)
    assert.Equal(t, pmID, pending[0].PaymentMethodID)

    // Criar depósito → não deve mais estar pendente
    require.NoError(t, db.Create(&models.Transaction{
        AccountID:              accountID,
        UserID:                 uuid.New(),
        Amount:                 650,
        Category:               "vr",
        Description:            "Vale Refeição",
        Date:                   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
        Source:                 "voucher_deposit",
        Type:                   "regular",
        VoucherPaymentMethodID: &pmID,
    }).Error)

    pending2, err := txRepo.VoucherDepositsPending(accountID, 2026, 5)
    require.NoError(t, err)
    assert.Empty(t, pending2)
}
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
cd backend && go test ./internal/repository/... -run TestVoucherSummary -v
cd backend && go test ./internal/repository/... -run TestVoucherDepositPending -v
```

Expected: FAIL — funções não existem ainda.

- [ ] **Step 3: Adicionar struct `VoucherSummary` e métodos no repository**

Em `backend/internal/repository/transaction.go`, adicionar após `BalanceTransferPending`:

```go
// VoucherSummary holds display data for a single voucher payment method.
type VoucherSummary struct {
    PaymentMethodID uuid.UUID `json:"payment_method_id"`
    Name            string    `json:"name"`
    Color           string    `json:"color"`
    Balance         float64   `json:"balance"`       // acumulado histórico
    MonthSpent      float64   `json:"month_spent"`   // gastos no mês solicitado
    LastDeposit     float64   `json:"last_deposit"`  // valor do depósito mais recente
}

// VoucherSummary returns balance and spending info for every voucher payment method
// in the account. Balance is the full historical accumulated balance.
func (r *TransactionRepository) VoucherSummary(accountID uuid.UUID, year, month int) ([]VoucherSummary, error) {
    start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
    end := start.AddDate(0, 1, 0)

    var rows []struct {
        ID          uuid.UUID
        Name        string
        Color       string
        TotalDeposit float64
        TotalSpent   float64
        MonthSpent   float64
        LastDeposit  float64
    }
    err := r.db.Raw(`
        SELECT
            pm.id,
            pm.name,
            pm.color,
            COALESCE(SUM(d.amount), 0)                                             AS total_deposit,
            COALESCE(ABS(SUM(e.amount)), 0)                                        AS total_spent,
            COALESCE(ABS(SUM(CASE WHEN e.date >= ? AND e.date < ? THEN e.amount ELSE 0 END)), 0) AS month_spent,
            COALESCE((
                SELECT amount FROM transactions
                WHERE source = 'voucher_deposit' AND voucher_payment_method_id = pm.id
                ORDER BY date DESC LIMIT 1
            ), 0)                                                                   AS last_deposit
        FROM payment_methods pm
        LEFT JOIN transactions d ON d.voucher_payment_method_id = pm.id
                                 AND d.source = 'voucher_deposit'
        LEFT JOIN transactions e ON e.payment_method_id = pm.id
                                 AND e.amount < 0
        WHERE pm.account_id = ?
          AND pm.kind = 'voucher'
          AND pm.active = true
        GROUP BY pm.id, pm.name, pm.color
    `, start, end, accountID).Scan(&rows).Error
    if err != nil {
        return nil, fmt.Errorf("transaction repository: voucher summary: %w", err)
    }

    result := make([]VoucherSummary, 0, len(rows))
    for _, row := range rows {
        result = append(result, VoucherSummary{
            PaymentMethodID: row.ID,
            Name:            row.Name,
            Color:           row.Color,
            Balance:         row.TotalDeposit - row.TotalSpent,
            MonthSpent:      row.MonthSpent,
            LastDeposit:     row.LastDeposit,
        })
    }
    return result, nil
}

// VoucherDepositPending holds info about a voucher awaiting deposit confirmation.
type VoucherDepositPending struct {
    PaymentMethodID uuid.UUID `json:"payment_method_id"`
    Name            string    `json:"name"`
    LastDeposit     float64   `json:"last_deposit"`
}

// VoucherDepositsPending returns vouchers with no deposit confirmed for the given month.
func (r *TransactionRepository) VoucherDepositsPending(accountID uuid.UUID, year, month int) ([]VoucherDepositPending, error) {
    start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
    end := start.AddDate(0, 1, 0)

    var rows []struct {
        ID          uuid.UUID
        Name        string
        LastDeposit float64
    }
    err := r.db.Raw(`
        SELECT
            pm.id,
            pm.name,
            COALESCE((
                SELECT amount FROM transactions
                WHERE source = 'voucher_deposit' AND voucher_payment_method_id = pm.id
                ORDER BY date DESC LIMIT 1
            ), 0) AS last_deposit
        FROM payment_methods pm
        WHERE pm.account_id = ?
          AND pm.kind = 'voucher'
          AND pm.active = true
          AND NOT EXISTS (
              SELECT 1 FROM transactions
              WHERE source = 'voucher_deposit'
                AND voucher_payment_method_id = pm.id
                AND date >= ? AND date < ?
          )
    `, accountID, start, end).Scan(&rows).Error
    if err != nil {
        return nil, fmt.Errorf("transaction repository: voucher deposits pending: %w", err)
    }

    result := make([]VoucherDepositPending, 0, len(rows))
    for _, row := range rows {
        result = append(result, VoucherDepositPending{
            PaymentMethodID: row.ID,
            Name:            row.Name,
            LastDeposit:     row.LastDeposit,
        })
    }
    return result, nil
}
```

- [ ] **Step 4: Excluir `voucher_deposit` do saldo da conta**

Em `SummaryByMonth`, a query do `balanceRow` precisa excluir transações com `source = 'voucher_deposit'` e despesas de voucher (`payment_method_id` de um voucher). Localizar a query de `balanceRow` (linha ~216) e substituir:

```go
err = r.db.Raw(`
    SELECT COALESCE(SUM(t.amount), 0) AS total
    FROM transactions t
    LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
    WHERE t.account_id = ?
      AND t.date >= ? AND t.date < ?
      AND t.type != 'invoice_payment'
      AND t.source != 'voucher_deposit'
      AND (pm.kind IS NULL OR pm.kind != 'voucher')
`, accountID, start, end).Scan(&balanceRow).Error
```

Fazer o mesmo ajuste na query de `realOutflowRow` (~linha 230) — adicionar as mesmas exclusões:

```go
err = r.db.Raw(`
    SELECT COALESCE(SUM(t.amount), 0) AS total
    FROM transactions t
    LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
    WHERE t.account_id = ?
      AND t.date >= ? AND t.date < ?
      AND t.type != 'transfer'
      AND t.amount < 0
      AND t.source != 'voucher_deposit'
      AND (pm.kind IS NULL OR pm.kind NOT IN ('credit_card', 'voucher') OR t.type = 'invoice_payment')
`, accountID, start, end).Scan(&realOutflowRow).Error
```

Também excluir `voucher_deposit` do `income` na query de category breakdown (~linha 190):

```go
err := r.db.Raw(`
    SELECT COALESCE(c.slug, t.category) AS category, SUM(t.amount) AS total
    FROM transactions t
    LEFT JOIN categories c ON c.id = t.category_id
    LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
    WHERE t.account_id = ?
      AND t.date >= ? AND t.date < ?
      AND t.type NOT IN ?
      AND t.source != 'voucher_deposit'
      AND (pm.kind IS NULL OR pm.kind != 'voucher')
    GROUP BY COALESCE(c.slug, t.category)
`, accountID, start, end, excludedFromCategoryBreakdown).Scan(&rows).Error
```

- [ ] **Step 5: Rodar os testes**

```bash
cd backend && go test ./internal/repository/... -run TestVoucherSummary -v
cd backend && go test ./internal/repository/... -run TestVoucherDepositPending -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/repository/
git commit -m "feat(repository): add VoucherSummary, VoucherDepositsPending; exclude voucher from account balance"
```

---

## Task 4: Handlers — rotas de voucher deposit

**Files:**
- Modify: `backend/internal/handlers/transaction.go`

- [ ] **Step 1: Adicionar os dois handlers no final de `transaction.go`**

```go
// VoucherDepositPending godoc
// GET /transactions/voucher-deposit/pending?year=2026&month=5
// Returns vouchers without a confirmed deposit in the given month.
func (h *TransactionHandler) VoucherDepositPending(w http.ResponseWriter, r *http.Request) {
    accountID := auth.AccountIDFromContext(r.Context())

    now := time.Now().UTC()
    year, month := now.Year(), int(now.Month())
    if y := r.URL.Query().Get("year"); y != "" {
        fmt.Sscanf(y, "%d", &year) //nolint:errcheck
    }
    if m := r.URL.Query().Get("month"); m != "" {
        fmt.Sscanf(m, "%d", &month) //nolint:errcheck
    }

    pending, err := h.repo.VoucherDepositsPending(accountID, year, month)
    if err != nil {
        log.Error().Err(err).Msg("transaction: voucher deposit pending")
        respondError(w, http.StatusInternalServerError, "internal error")
        return
    }
    respondJSON(w, http.StatusOK, pending)
}

// ConfirmVoucherDeposit godoc
// POST /transactions/voucher-deposit
// Creates a voucher_deposit transaction for the given voucher and month.
func (h *TransactionHandler) ConfirmVoucherDeposit(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    accountID := auth.AccountIDFromContext(ctx)
    userID := auth.UserIDFromContext(ctx)

    var req struct {
        PaymentMethodID uuid.UUID `json:"payment_method_id"`
        Amount          float64   `json:"amount"`
        Year            int       `json:"year"`
        Month           int       `json:"month"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
        req.Amount <= 0 || req.Year == 0 || req.Month == 0 || req.PaymentMethodID == uuid.Nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    monthLabel := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.UTC).
        Format("January 2006")

    tx := &models.Transaction{
        AccountID:              accountID,
        UserID:                 userID,
        Amount:                 req.Amount,
        Category:               "vr",
        Description:            fmt.Sprintf("Depósito VR — %s", monthLabel),
        Date:                   time.Date(req.Year, time.Month(req.Month), 1, 12, 0, 0, 0, time.UTC),
        Source:                 "voucher_deposit",
        Type:                   "regular",
        VoucherPaymentMethodID: &req.PaymentMethodID,
    }
    if err := h.repo.Create(tx); err != nil {
        log.Error().Err(err).Msg("transaction: confirm voucher deposit")
        respondError(w, http.StatusInternalServerError, "internal error")
        return
    }
    respondJSON(w, http.StatusCreated, tx)
}
```

- [ ] **Step 2: Verificar que compila**

```bash
cd backend && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/transaction.go
git commit -m "feat(handlers): add VoucherDepositPending and ConfirmVoucherDeposit"
```

---

## Task 5: Registrar rotas e incluir voucher_summary no dashboard

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/handlers/dashboard.go`

- [ ] **Step 1: Registrar as novas rotas em `main.go`**

Dentro do bloco `r.Route("/transactions", ...)`, adicionar após as rotas de balance-transfer existentes:

```go
r.Get("/voucher-deposit/pending", txHandler.VoucherDepositPending)
r.Post("/voucher-deposit", txHandler.ConfirmVoucherDeposit)
```

- [ ] **Step 2: Adicionar `voucher_summary` e `voucher_pending` à resposta do dashboard**

Em `dashboard.go`, adicionar dois channels e goroutines paralelos aos existentes:

```go
voucherCh := make(chan anyResult, 1)
voucherPendingCh := make(chan anyResult, 1)

go func() {
    items, err := h.txRepo.VoucherSummary(accountID, year, month)
    if err != nil {
        log.Error().Err(err).Msg("dashboard: voucher summary error")
        voucherCh <- anyResult{val: []repository.VoucherSummary{}}
        return
    }
    if items == nil {
        items = []repository.VoucherSummary{}
    }
    voucherCh <- anyResult{val: items}
}()

go func() {
    items, err := h.txRepo.VoucherDepositsPending(accountID, year, month)
    if err != nil {
        log.Error().Err(err).Msg("dashboard: voucher pending error")
        voucherPendingCh <- anyResult{val: []repository.VoucherDepositPending{}}
        return
    }
    if items == nil {
        items = []repository.VoucherDepositPending{}
    }
    voucherPendingCh <- anyResult{val: items}
}()
```

Adicionar receives junto dos demais e incluir no respondJSON:

```go
voucherRes := <-voucherCh
voucherPendingRes := <-voucherPendingCh
// ...
"voucher_summary": voucherRes.val,
"voucher_pending": voucherPendingRes.val,
```

- [ ] **Step 3: Verificar que compila**

```bash
cd backend && go build ./...
```

- [ ] **Step 4: Testar o endpoint manualmente** (servidor deve estar rodando)

```bash
curl -s -H "Authorization: Bearer <token>" \
  "http://localhost:8080/dashboard?year=2026&month=5" | jq '.voucher_summary'
```

Expected: array com pelo menos um objeto `{ payment_method_id, name, balance, month_spent, last_deposit }`.

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/server/main.go backend/internal/handlers/dashboard.go
git commit -m "feat(dashboard): add voucher_summary to dashboard response; register voucher-deposit routes"
```

---

## Task 6: Frontend — tipos e api service

**Files:**
- Modify: `frontend/src/hooks/useDashboard.ts`
- Modify: `frontend/src/services/api.ts`

- [ ] **Step 1: Adicionar tipos e atualizar `DashboardData` em `useDashboard.ts`**

Logo após a interface `PaymentMethodSummary`:

```typescript
export interface VoucherSummary {
  payment_method_id: string;
  name: string;
  color: string;
  balance: number;
  month_spent: number;
  last_deposit: number;
}

export interface VoucherPending {
  payment_method_id: string;
  name: string;
  last_deposit: number;
}
```

Em `DashboardData`, adicionar:

```typescript
voucherSummary: VoucherSummary[];
voucherPending: VoucherPending[];
```

Na função `useDashboardQuery`, dentro do `queryFn`, mapear os campos:

```typescript
voucherSummary: data.voucher_summary ?? [],
voucherPending: data.voucher_pending ?? [],
```

- [ ] **Step 2: Adicionar `confirmVoucherDeposit` em `api.ts`**

Após `confirmBalanceTransfer`:

```typescript
confirmVoucherDeposit: (paymentMethodId: string, amount: number, year: number, month: number) =>
  request<void>("/transactions/voucher-deposit", {
    method: "POST",
    body: JSON.stringify({ payment_method_id: paymentMethodId, amount, year, month }),
  }),
```

- [ ] **Step 3: Verificar que compila**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

Expected: sem erros de TypeScript.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/hooks/useDashboard.ts frontend/src/services/api.ts
git commit -m "feat(frontend): add VoucherSummary type and confirmVoucherDeposit api method"
```

---

## Task 7: Frontend — banner de confirmação e card do VR no Caixa

**Files:**
- Modify: `frontend/src/pages/Caixa.tsx`

- [ ] **Step 1: Adicionar estado para o banner de voucher**

Junto dos outros estados no componente `Caixa`, adicionar:

```typescript
const [voucherDismissed, setVoucherDismissed] = useState<Record<string, boolean>>({});
const [confirmingVoucher, setConfirmingVoucher] = useState<string | null>(null);
const [voucherAmounts, setVoucherAmounts] = useState<Record<string, string>>({});
```

- [ ] **Step 2: Extrair dados de voucher do dashData**

Junto de onde `balanceTransfer` é extraído:

```typescript
const voucherSummary = dashData?.voucherSummary ?? [];
const voucherPending = dashData?.voucherPending ?? [];
```

- [ ] **Step 3: Handler de confirmação de voucher no Caixa**

```typescript
const handleConfirmVoucher = async (paymentMethodId: string) => {
  const amount = parseFloat(voucherAmounts[paymentMethodId] ?? "0");
  if (!amount || amount <= 0) return;
  setConfirmingVoucher(paymentMethodId);
  try {
    await api.confirmVoucherDeposit(paymentMethodId, amount, selectedYear, selectedMonth);
    invalidateDashboard();
  } finally {
    setConfirmingVoucher(null);
  }
};
```

- [ ] **Step 5: Inicializar `voucherAmounts` com os valores de `last_deposit`**

```typescript
useEffect(() => {
  if (!dashData?.voucherPending) return;
  setVoucherAmounts((prev) => {
    const next = { ...prev };
    for (const v of dashData.voucherPending) {
      if (!(v.payment_method_id in next)) {
        next[v.payment_method_id] = v.last_deposit > 0 ? String(v.last_deposit) : "";
      }
    }
    return next;
  });
}, [dashData?.voucherPending]);
```

- [ ] **Step 6: Renderizar banners de voucher pendente**

Logo após o banner de balance transfer existente (`{balanceTransfer && (...) }`), adicionar:

```tsx
{dashData?.voucherPending?.map((v) => !voucherDismissed[v.payment_method_id] && (
  <div key={v.payment_method_id} className="mx-4 mb-2 rounded-xl bg-primary/10 border border-primary/20 p-4 space-y-2">
    <p className="text-sm font-medium">Depósito do {v.name}</p>
    <p className="text-xs text-muted-foreground">Qual foi o valor recebido em {MONTHS_PT[selectedMonth - 1]}?</p>
    <div className="flex gap-2 items-center">
      <Input
        className="h-8 text-xs flex-1"
        type="number"
        value={voucherAmounts[v.payment_method_id] ?? ""}
        onChange={(e) =>
          setVoucherAmounts((prev) => ({ ...prev, [v.payment_method_id]: e.target.value }))
        }
        placeholder="0,00"
      />
      <Button size="sm" variant="ghost" className="h-8 text-xs shrink-0"
        onClick={() => setVoucherDismissed((prev) => ({ ...prev, [v.payment_method_id]: true }))}>
        Ignorar
      </Button>
      <Button size="sm" className="h-8 text-xs shrink-0"
        onClick={() => handleConfirmVoucher(v.payment_method_id)}
        disabled={confirmingVoucher === v.payment_method_id}>
        {confirmingVoucher === v.payment_method_id ? "..." : "Confirmar"}
      </Button>
    </div>
  </div>
))}
```

- [ ] **Step 7: Renderizar cards de voucher**

Após os cards de resumo (Entradas / Saídas reais), antes da lista de transações, adicionar:

```tsx
{(dashData?.voucherSummary ?? []).map((v) => {
  const pct = v.last_deposit > 0 ? v.balance / v.last_deposit : 1;
  const colorClass =
    pct > 0.3
      ? "bg-emerald-500/10 border-emerald-500/20 text-emerald-600"
      : pct > 0.1
      ? "bg-amber-500/10 border-amber-500/20 text-amber-600"
      : "bg-destructive/10 border-destructive/20 text-destructive";

  return (
    <div key={v.payment_method_id} className={cn("rounded-xl border p-3 space-y-1", colorClass)}>
      <p className="text-xs font-semibold">{v.name}</p>
      <div className="flex justify-between text-xs">
        <span className="text-muted-foreground">Saldo</span>
        <span className="font-semibold tabular-nums">{fmt(v.balance)}</span>
      </div>
      <div className="flex justify-between text-xs">
        <span className="text-muted-foreground">Gasto em {MONTHS_PT[selectedMonth - 1]}</span>
        <span className="tabular-nums">{fmt(v.month_spent)}</span>
      </div>
    </div>
  );
})}
```

- [ ] **Step 8: Verificar que compila**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

Expected: sem erros.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/pages/Caixa.tsx frontend/src/hooks/useDashboard.ts frontend/src/services/api.ts backend/internal/handlers/dashboard.go
git commit -m "feat(caixa): add voucher deposit banner and VR balance card"
```

---

## Task 8: Perfil — remover credit_limit e saldo de vouchers da UI

**Files:**
- Modify: `frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx`

- [ ] **Step 1: Ocultar `credit_limit` para vouchers**

Em `PaymentMethodManager.tsx`, localizar onde `credit_limit` é renderizado/editado. Está dentro de um bloco `{isCreditCard && ...}` ou similar. Garantir que o campo só aparece quando `kind === 'credit_card'`.

Localizar a condição atual:

```typescript
const isCreditCard = form.kind === "credit_card";
```

Verificar que o campo `credit_limit` no formulário já usa `isCreditCard` como guard. Se não usar, envolver o campo com `{isCreditCard && (...)}`  para que nunca apareça para vouchers.

Também localizar onde o saldo (`balance`) é exibido no card do payment method e garantir que para vouchers ele não seja exibido (o saldo agora vive no Caixa):

```typescript
// Linha ~71 atual:
const hasBalance = pm.balance !== undefined && (pm.kind === "voucher" || pm.kind === "cash")
// Alterar para:
const hasBalance = pm.balance !== undefined && pm.kind === "cash"
```

- [ ] **Step 2: Verificar que compila**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx
git commit -m "feat(profile): hide credit_limit and balance for voucher payment methods"
```

---

## Task 9: Teste de integração — fluxo completo

**Files:**
- Modify: `backend/internal/repository/voucher_test.go` (já criado na Task 3)

- [ ] **Step 1: Rodar todos os testes do backend**

```bash
cd backend && go test ./... -timeout 60s
```

Expected: PASS em todos.

- [ ] **Step 2: Smoke test manual no frontend**

1. Abrir `http://localhost:5173/caixa`
2. Navegar para o mês atual — confirmar que banner de depósito do VR aparece
3. Editar o valor e clicar em Confirmar — banner deve sumir, card do VR deve aparecer com saldo atualizado
4. Verificar que o saldo disponível da conta (topo do Caixa) não inclui o VR
5. Navegar para um mês anterior sem depósito confirmado — banner deve reaparecer
6. Abrir Perfil → Métodos de pagamento → verificar que VR não mostra `credit_limit` nem saldo

- [ ] **Step 3: Commit final se tudo ok**

```bash
git add .
git commit -m "chore: final integration pass — voucher deposit flow complete"
```
