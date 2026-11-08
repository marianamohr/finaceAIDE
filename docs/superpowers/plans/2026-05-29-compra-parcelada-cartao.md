# Compra Avulsa Parcelada no Cartão — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permitir registrar uma compra única parcelada no cartão de crédito (ex.: TV R$3.000 em 10x), gerando N transações em faturas consecutivas, agrupadas por uma compra "pai".

**Architecture:** Nova entidade `installment_purchases` (a compra pai) + duas colunas em `transactions` (`installment_purchase_id`, `installment_number`). Um serviço puro fatia o total em centavos (resto na última parcela) e calcula os meses de fatura via o já existente `models.ComputeBillingMonth`. As N transações são materializadas *upfront* com `date` = 1º dia do mês de billing (mesma convenção de `EnsureCardTransactionsForMonth`), para que tanto o resumo por `date` quanto a fatura por `billing_month` fiquem corretos. Entrada pela frase do usuário no chat (tool nova) ou na instrução da foto (OCR ecoa o N). O parcelamento via prompt anterior é aposentado.

**Tech Stack:** Go (chi, GORM, zerolog), PostgreSQL, testify. Spec: `docs/superpowers/specs/2026-05-29-compra-parcelada-cartao-design.md`.

---

## File Structure

**Criados:**
- `backend/internal/models/installment_purchase.go` — struct `InstallmentPurchase` + `TableName()`.
- `backend/internal/services/installments/installments.go` — funções puras: `SplitCents`, `BillingMonths`, `GenerateTransactions`.
- `backend/internal/services/installments/installments_test.go` — testes de unidade.
- `backend/internal/repository/installment_purchase.go` — CRUD + materialização atômica.
- `backend/internal/repository/installment_purchase_integration_test.go` — integração.
- `backend/internal/handlers/installment_purchase.go` — handlers HTTP.
- `backend/internal/agent/tools_installment.go` — write tool `create_installment_purchase`.

**Modificados:**
- `backend/internal/models/transaction.go` — 2 colunas + comentário de `source`.
- `backend/migrations/migrate.go` — registra o model + migra colunas/índice em `transactions`.
- `backend/cmd/server/main.go` — wiring de repo, handler, rotas e tool.
- `backend/internal/services/rag/context.go` — remove parcelamento via prompt + `InstallmentDates`.
- `backend/internal/agent/tools_write.go` — ajusta descrição de `replace_transaction`.
- `backend/internal/services/vision/ocr.go` — campo `parcelas` (ecoado da instrução).
- `backend/internal/handlers/upload.go` — cria compra parcelada quando `parcelas > 1`.
- `frontend/...` — rótulo "k/N" no card da transação (Fase 4).

**Convenções do repo:**
- IDs e FKs são colunas `type:uuid` **sem** constraint de FK no GORM (igual a `RecurringExpense.PaymentMethodID`).
- Testes de integração começam com `//go:build integration` e usam `TEST_DATABASE_URL` (default `postgres://financeai:<senha>@localhost:5432/financeai_test?sslmode=disable`).
- Rodar unidade: `cd backend && go test ./internal/services/installments/...`
- Rodar integração: `cd backend && go test -tags=integration ./...`

---

## FASE 1 — Núcleo do domínio (API completa, sem agente/OCR)

Ao fim da Fase 1 existe uma API REST testável que cria/lista/edita/exclui compras parceladas a partir de um `payment_method_id` já resolvido. É software funcional por si só e base para as fases seguintes.

### Task 1: Modelo `InstallmentPurchase` + colunas na transação

**Files:**
- Create: `backend/internal/models/installment_purchase.go`
- Modify: `backend/internal/models/transaction.go`

- [ ] **Step 1: Criar o model**

`backend/internal/models/installment_purchase.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// InstallmentPurchase é a "compra pai" de uma compra avulsa parcelada no cartão
// de crédito. Cada parcela é uma Transaction com source="installment" apontando
// para esta compra via InstallmentPurchaseID.
type InstallmentPurchase struct {
	ID              uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID       uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"`
	PaymentMethodID uuid.UUID  `gorm:"not null;type:uuid"                             json:"payment_method_id"` // deve ser credit_card
	CategoryID      *uuid.UUID `gorm:"type:uuid"                                      json:"category_id,omitempty"`
	Description     string     `gorm:"type:text;not null"                            json:"description"`
	Merchant        string     `gorm:"type:text;default:''"                          json:"merchant"`
	TotalAmount     float64    `gorm:"type:numeric(12,2);not null"                   json:"total_amount"` // positivo
	Installments    int        `gorm:"not null"                                      json:"installments"` // N >= 2
	PurchaseDate    time.Time  `gorm:"not null"                                      json:"purchase_date"`
	CreatedAt       time.Time  `                                                     json:"created_at"`
	UpdatedAt       time.Time  `                                                     json:"updated_at"`
}

func (InstallmentPurchase) TableName() string { return "installment_purchases" }
```

- [ ] **Step 2: Adicionar colunas em `transactions`**

Em `backend/internal/models/transaction.go`, após a linha `RecurringExpenseID *uuid.UUID ...` (linha ~26), adicione:

```go
	InstallmentPurchaseID *uuid.UUID `gorm:"type:uuid;index" json:"installment_purchase_id,omitempty"`
	InstallmentNumber     *int       `                       json:"installment_number,omitempty"`
```

E na linha 9, adicione `"installment"` ao comentário de valores de Source:

```go
// Source values: "manual" | "chat" | "photo" | "recurring" | "installment" | "balance_transfer" | "voucher_deposit"
```

- [ ] **Step 3: Compilar**

Run: `cd backend && go build ./...`
Expected: compila sem erros.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/models/installment_purchase.go backend/internal/models/transaction.go
git commit -m "feat(models): InstallmentPurchase + colunas de parcela na transação"
```

---

### Task 2: Serviço puro `SplitCents` (divisão com resto na última)

**Files:**
- Create: `backend/internal/services/installments/installments.go`
- Test: `backend/internal/services/installments/installments_test.go`

- [ ] **Step 1: Escrever o teste que falha**

`backend/internal/services/installments/installments_test.go`:

```go
package installments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCents(t *testing.T) {
	tests := []struct {
		name  string
		total int64
		n     int
		want  []int64
	}{
		{"divisão exata", 30000, 10, []int64{3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000}},
		{"resto na última (1000/3)", 100000, 3, []int64{33333, 33333, 33334}},
		{"n=2 ímpar", 1001, 2, []int64{500, 501}},
		{"n=1 devolve total", 12345, 1, []int64{12345}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitCents(tt.total, tt.n)
			require.Len(t, got, tt.n)
			assert.Equal(t, tt.want, got)

			var sum int64
			for _, c := range got {
				sum += c
			}
			assert.Equal(t, tt.total, sum, "a soma das parcelas deve bater com o total")
		})
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/services/installments/...`
Expected: FAIL — `undefined: SplitCents`.

- [ ] **Step 3: Implementar**

`backend/internal/services/installments/installments.go`:

```go
// Package installments contém a lógica pura de fatiamento de uma compra
// parcelada em N transações de fatura. Sem dependência de banco — testável
// isoladamente.
package installments

// SplitCents divide totalCents em n parcelas inteiras de centavos, colocando o
// resto da divisão na ÚLTIMA parcela. A soma das parcelas é sempre == totalCents.
// Pré-condição: n >= 1 (o caller valida n >= 2 nas regras de negócio).
func SplitCents(totalCents int64, n int) []int64 {
	parts := make([]int64, n)
	base := totalCents / int64(n)
	for i := 0; i < n-1; i++ {
		parts[i] = base
	}
	parts[n-1] = totalCents - base*int64(n-1)
	return parts
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/services/installments/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/installments/
git commit -m "feat(installments): SplitCents com resto na última parcela"
```

---

### Task 3: Serviço puro `BillingMonths` (meses consecutivos com rollover)

**Files:**
- Modify: `backend/internal/services/installments/installments.go`
- Modify: `backend/internal/services/installments/installments_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Adicione em `installments_test.go`:

```go
import "time" // adicione ao bloco de imports existente

func TestBillingMonths(t *testing.T) {
	tests := []struct {
		name       string
		firstYear  int
		firstMonth time.Month
		n          int
		want       []BillingMonth
	}{
		{
			"sem rollover",
			2026, time.March, 3,
			[]BillingMonth{{2026, time.March}, {2026, time.April}, {2026, time.May}},
		},
		{
			"rollover de ano",
			2026, time.November, 3,
			[]BillingMonth{{2026, time.November}, {2026, time.December}, {2027, time.January}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BillingMonths(tt.firstYear, tt.firstMonth, tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/services/installments/...`
Expected: FAIL — `undefined: BillingMonths` e `undefined: BillingMonth`.

- [ ] **Step 3: Implementar**

Adicione em `installments.go` (e `import "time"` no topo):

```go
import "time"

// BillingMonth identifica a fatura de uma parcela.
type BillingMonth struct {
	Year  int
	Month time.Month
}

// BillingMonths devolve n meses de fatura consecutivos a partir do primeiro,
// com rollover de ano. Ancora no dia 1 para evitar overflow de dia do AddDate.
func BillingMonths(firstYear int, firstMonth time.Month, n int) []BillingMonth {
	out := make([]BillingMonth, n)
	anchor := time.Date(firstYear, firstMonth, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		m := anchor.AddDate(0, i, 0)
		out[i] = BillingMonth{Year: m.Year(), Month: m.Month()}
	}
	return out
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/services/installments/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/installments/
git commit -m "feat(installments): BillingMonths com rollover de ano"
```

---

### Task 4: `GenerateTransactions` (monta as N parcelas)

**Files:**
- Modify: `backend/internal/services/installments/installments.go`
- Modify: `backend/internal/services/installments/installments_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Adicione em `installments_test.go` (e imports `"github.com/google/uuid"` e `"github.com/marianamohr/financas/internal/models"`):

```go
func TestGenerateTransactions(t *testing.T) {
	purchaseDate := time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC)
	pmID := uuid.New()
	catID := uuid.New()
	userID := uuid.New()
	p := &models.InstallmentPurchase{
		ID:              uuid.New(),
		AccountID:       uuid.New(),
		PaymentMethodID: pmID,
		CategoryID:      &catID,
		Description:     "TV Samsung",
		Merchant:        "Magazine",
		TotalAmount:     1000.00,
		Installments:    3,
		PurchaseDate:    purchaseDate,
	}

	// closingDay=20 → compra em 10/mai cai na fatura de maio (10 <= 20).
	txs := GenerateTransactions(p, 20, userID, "eletronicos")

	require.Len(t, txs, 3)

	// Valores: resto na última (333.33, 333.33, 333.34) — negativos (despesa).
	assert.InDelta(t, -333.33, txs[0].Amount, 0.001)
	assert.InDelta(t, -333.33, txs[1].Amount, 0.001)
	assert.InDelta(t, -333.34, txs[2].Amount, 0.001)

	// Meses de fatura: maio, junho, julho de 2026.
	assert.Equal(t, 2026, *txs[0].BillingYear)
	assert.Equal(t, 5, *txs[0].BillingMonth)
	assert.Equal(t, 7, *txs[2].BillingMonth)

	// date = 1º dia do mês de billing.
	assert.Equal(t, time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC), txs[0].Date)

	// Metadados de parcela.
	assert.Equal(t, p.ID, *txs[0].InstallmentPurchaseID)
	assert.Equal(t, 1, *txs[0].InstallmentNumber)
	assert.Equal(t, 3, *txs[2].InstallmentNumber)
	assert.Equal(t, "installment", txs[0].Source)
	assert.Equal(t, pmID, *txs[0].PaymentMethodID)
	assert.Equal(t, "eletronicos", txs[0].Category)
	assert.Equal(t, "TV Samsung (1/3)", txs[0].Description)
	assert.Equal(t, "TV Samsung (3/3)", txs[2].Description)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/services/installments/...`
Expected: FAIL — `undefined: GenerateTransactions`.

- [ ] **Step 3: Implementar**

Adicione em `installments.go` (imports: `"fmt"`, `"math"`, `"github.com/google/uuid"`, `"github.com/marianamohr/financas/internal/models"`):

```go
// GenerateTransactions materializa as N parcelas de uma compra parcelada.
// closingDay é o dia de fechamento do cartão (usado para a 1ª fatura).
// categorySlug preenche o campo legado Transaction.Category.
//
// Cada transação: amount negativo (despesa), date = 1º dia do mês de billing
// (mesma convenção de EnsureCardTransactionsForMonth), billing_year/month da
// parcela, e os metadados de parcela.
func GenerateTransactions(p *models.InstallmentPurchase, closingDay int, userID uuid.UUID, categorySlug string) []*models.Transaction {
	totalCents := int64(math.Round(p.TotalAmount * 100))
	parts := SplitCents(totalCents, p.Installments)

	firstYear, firstMonth := models.ComputeBillingMonth(p.PurchaseDate, closingDay)
	months := BillingMonths(firstYear, firstMonth, p.Installments)

	txs := make([]*models.Transaction, p.Installments)
	for i := 0; i < p.Installments; i++ {
		year := months[i].Year
		month := int(months[i].Month)
		num := i + 1
		amount := -float64(parts[i]) / 100

		tx := &models.Transaction{
			UserID:                userID,
			AccountID:             p.AccountID,
			Amount:                amount,
			Category:              categorySlug,
			CategoryID:            p.CategoryID,
			Merchant:              p.Merchant,
			Description:           fmt.Sprintf("%s (%d/%d)", p.Description, num, p.Installments),
			Date:                  time.Date(year, months[i].Month, 1, 0, 0, 0, 0, time.UTC),
			Source:                "installment",
			Type:                  "regular",
			PaymentMethodID:       &p.PaymentMethodID,
			BillingYear:           &year,
			BillingMonth:          &month,
			InstallmentPurchaseID: &p.ID,
			InstallmentNumber:     &num,
			Locale:                "BR",
			Currency:              "BRL",
		}
		txs[i] = tx
	}
	return txs
}
```

Nota: `year`/`month` são variáveis novas a cada iteração do `for` (Go 1.22+ dá um novo binding por iteração), seguro para `&year`/`&month`.

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/services/installments/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/installments/
git commit -m "feat(installments): GenerateTransactions materializa as N parcelas"
```

---

### Task 5: Migration (tabela + colunas + índices)

**Files:**
- Modify: `backend/migrations/migrate.go`

- [ ] **Step 1: Registrar o model na AutoMigrate inicial**

Em `backend/migrations/migrate.go`, no primeiro bloco `db.AutoMigrate(...)` (linha ~18-37), adicione após `&models.RecurringExpense{},`:

```go
		&models.RecurringExpense{},
		&models.InstallmentPurchase{},
```

- [ ] **Step 2: Adicionar função de migração das colunas de transação**

Após a função `migratePhase2` (termina ~linha 552), adicione:

```go
// migrateInstallments adiciona as colunas de parcela em transactions.
// Ambas nullable — linhas existentes não são afetadas. Idempotente.
func migrateInstallments(db *gorm.DB) error {
	steps := []struct {
		desc string
		sql  string
	}{
		{
			"add installment_purchase_id to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS installment_purchase_id uuid`,
		},
		{
			"add installment_number to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS installment_number int`,
		},
		{
			"create index on transactions.installment_purchase_id",
			`CREATE INDEX IF NOT EXISTS idx_transactions_installment ON transactions (installment_purchase_id) WHERE installment_purchase_id IS NOT NULL`,
		},
	}
	for _, step := range steps {
		if err := db.Exec(step.sql).Error; err != nil {
			return fmt.Errorf("migrate installments: %s: %w", step.desc, err)
		}
	}
	return nil
}
```

- [ ] **Step 3: Chamar a nova migração**

Logo após a chamada `migratePhase2(db)` (linha ~45-47), adicione:

```go
	if err := migrateInstallments(db); err != nil {
		return fmt.Errorf("migrations: migrate installments: %w", err)
	}
```

- [ ] **Step 4: Compilar e migrar contra o banco local**

Run:
```bash
cd backend && go build ./... && go run ./cmd/migrate
```
Expected: build ok; migração roda sem erro. (Banco local em docker: user/pass/db = financeai.)

- [ ] **Step 5: Verificar o schema**

Run:
```bash
docker exec -i $(docker ps -qf "ancestor=postgres" | head -1) psql -U financeai -d financeai -c "\d installment_purchases" -c "\d transactions" | grep -i installment
```
Expected: tabela `installment_purchases` existe; `transactions` tem `installment_purchase_id` e `installment_number`.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/migrate.go
git commit -m "feat(migrations): tabela installment_purchases + colunas de parcela"
```

---

### Task 6: Repository — materialização atômica e CRUD

**Files:**
- Create: `backend/internal/repository/installment_purchase.go`
- Test: `backend/internal/repository/installment_purchase_integration_test.go`

- [ ] **Step 1: Escrever o teste de integração que falha**

`backend/internal/repository/installment_purchase_integration_test.go`:

```go
//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallmentPurchaseRepository_CreateAndDelete(t *testing.T) {
	db := newRepoTestDB(t) // helper já existente nos testes de integração deste pacote
	repo := NewInstallmentPurchaseRepository(db)
	txRepo := NewTransactionRepository(db)

	accountID := uuid.New()
	p := &models.InstallmentPurchase{
		AccountID:       accountID,
		PaymentMethodID: uuid.New(),
		Description:     "TV",
		TotalAmount:     900.00,
		Installments:    3,
		PurchaseDate:    time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC),
	}
	num1 := 1
	txs := []*models.Transaction{
		{AccountID: accountID, UserID: uuid.New(), Amount: -300, Category: "eletronicos", Date: time.Now(), Source: "installment", InstallmentNumber: &num1},
		{AccountID: accountID, UserID: uuid.New(), Amount: -300, Category: "eletronicos", Date: time.Now(), Source: "installment"},
		{AccountID: accountID, UserID: uuid.New(), Amount: -300, Category: "eletronicos", Date: time.Now(), Source: "installment"},
	}

	require.NoError(t, repo.Create(p, txs))
	assert.NotEqual(t, uuid.Nil, p.ID)

	// As 3 transações existem e apontam para o pai.
	all, err := txRepo.ListByAccount(accountID)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	for _, tx := range all {
		require.NotNil(t, tx.InstallmentPurchaseID)
		assert.Equal(t, p.ID, *tx.InstallmentPurchaseID)
	}

	// Delete remove pai + todas as parcelas.
	require.NoError(t, repo.Delete(p.ID, accountID))
	_, err = repo.FindByID(p.ID, accountID)
	assert.ErrorIs(t, err, ErrNotFound)
	remaining, err := txRepo.ListByAccount(accountID)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}
```

Nota: se o helper de DB de teste deste pacote tiver outro nome, use o existente (ver `transaction_integration_test.go`). Garanta que o schema de teste rode as migrations (ou crie a tabela no setup, como os outros testes do pacote fazem).

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test -tags=integration ./internal/repository/ -run TestInstallmentPurchaseRepository`
Expected: FAIL — `undefined: NewInstallmentPurchaseRepository`.

- [ ] **Step 3: Implementar o repository**

`backend/internal/repository/installment_purchase.go`:

```go
package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type InstallmentPurchaseRepository struct {
	db *gorm.DB
}

func NewInstallmentPurchaseRepository(db *gorm.DB) *InstallmentPurchaseRepository {
	return &InstallmentPurchaseRepository{db: db}
}

// Create insere a compra pai e suas parcelas em uma única transação de banco.
// Seta InstallmentPurchaseID em cada parcela com o ID gerado do pai.
func (r *InstallmentPurchaseRepository) Create(p *models.InstallmentPurchase, txs []*models.Transaction) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Create(p).Error; err != nil {
			return fmt.Errorf("installment purchase repository: create parent: %w", err)
		}
		for _, tx := range txs {
			tx.InstallmentPurchaseID = &p.ID
			if err := db.Create(tx).Error; err != nil {
				return fmt.Errorf("installment purchase repository: create installment: %w", err)
			}
		}
		return nil
	})
}

func (r *InstallmentPurchaseRepository) FindByID(id, accountID uuid.UUID) (*models.InstallmentPurchase, error) {
	var p models.InstallmentPurchase
	err := r.db.Where("id = ? AND account_id = ?", id, accountID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("installment purchase repository: find: %w", err)
	}
	return &p, nil
}

func (r *InstallmentPurchaseRepository) ListByAccount(accountID uuid.UUID) ([]models.InstallmentPurchase, error) {
	var items []models.InstallmentPurchase
	if err := r.db.Where("account_id = ?", accountID).Order("purchase_date desc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("installment purchase repository: list: %w", err)
	}
	return items, nil
}

// ListTransactions devolve as parcelas (transações) de uma compra, ordenadas por número.
func (r *InstallmentPurchaseRepository) ListTransactions(purchaseID, accountID uuid.UUID) ([]models.Transaction, error) {
	var txs []models.Transaction
	err := r.db.
		Where("installment_purchase_id = ? AND account_id = ?", purchaseID, accountID).
		Order("installment_number asc").
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("installment purchase repository: list transactions: %w", err)
	}
	return txs, nil
}

// Delete remove a compra pai e todas as suas parcelas em uma transação de banco.
func (r *InstallmentPurchaseRepository) Delete(id, accountID uuid.UUID) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Where("installment_purchase_id = ? AND account_id = ?", id, accountID).
			Delete(&models.Transaction{}).Error; err != nil {
			return fmt.Errorf("installment purchase repository: delete installments: %w", err)
		}
		result := db.Where("id = ? AND account_id = ?", id, accountID).
			Delete(&models.InstallmentPurchase{})
		if result.Error != nil {
			return fmt.Errorf("installment purchase repository: delete parent: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Regenerate atualiza os campos da compra pai, apaga as parcelas antigas e insere
// as novas — tudo em uma transação de banco (tudo-ou-nada).
func (r *InstallmentPurchaseRepository) Regenerate(p *models.InstallmentPurchase, txs []*models.Transaction, accountID uuid.UUID) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		result := db.Model(&models.InstallmentPurchase{}).
			Where("id = ? AND account_id = ?", p.ID, accountID).
			Save(p)
		if result.Error != nil {
			return fmt.Errorf("installment purchase repository: regenerate save parent: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		if err := db.Where("installment_purchase_id = ? AND account_id = ?", p.ID, accountID).
			Delete(&models.Transaction{}).Error; err != nil {
			return fmt.Errorf("installment purchase repository: regenerate delete old: %w", err)
		}
		for _, tx := range txs {
			tx.InstallmentPurchaseID = &p.ID
			if err := db.Create(tx).Error; err != nil {
				return fmt.Errorf("installment purchase repository: regenerate create: %w", err)
			}
		}
		return nil
	})
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test -tags=integration ./internal/repository/ -run TestInstallmentPurchaseRepository`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repository/installment_purchase.go backend/internal/repository/installment_purchase_integration_test.go
git commit -m "feat(repository): CRUD atômico de compra parcelada"
```

---

### Task 7: Handler HTTP + rotas

**Files:**
- Create: `backend/internal/handlers/installment_purchase.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Implementar o handler**

`backend/internal/handlers/installment_purchase.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/installments"
	"github.com/rs/zerolog/log"
)

type InstallmentPurchaseHandler struct {
	repo    *repository.InstallmentPurchaseRepository
	pmRepo  *repository.PaymentMethodRepository
	catRepo *repository.CategoryRepository
}

func NewInstallmentPurchaseHandler(
	repo *repository.InstallmentPurchaseRepository,
	pmRepo *repository.PaymentMethodRepository,
	catRepo *repository.CategoryRepository,
) *InstallmentPurchaseHandler {
	return &InstallmentPurchaseHandler{repo: repo, pmRepo: pmRepo, catRepo: catRepo}
}

type installmentPurchaseRequest struct {
	PaymentMethodID uuid.UUID `json:"payment_method_id"`
	Category        string    `json:"category"`      // slug
	Description     string    `json:"description"`
	Merchant        string    `json:"merchant"`
	TotalAmount     float64   `json:"total_amount"`  // positivo
	Installments    int       `json:"installments"`  // >= 2
	Date            string    `json:"date"`          // YYYY-MM-DD
}

// buildPurchaseAndTxs valida o request, resolve cartão e categoria, e devolve a
// compra pai + as parcelas geradas. Erro com mensagem amigável em validação.
func (h *InstallmentPurchaseHandler) buildPurchaseAndTxs(
	accountID, userID uuid.UUID, req installmentPurchaseRequest,
) (*models.InstallmentPurchase, []*models.Transaction, string) {
	if req.TotalAmount <= 0 {
		return nil, nil, "total_amount deve ser maior que zero"
	}
	if req.Installments < 2 {
		return nil, nil, "installments deve ser >= 2"
	}
	if req.Description == "" {
		return nil, nil, "description é obrigatório"
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, nil, "date deve estar no formato YYYY-MM-DD"
	}

	pm, err := h.pmRepo.FindByID(req.PaymentMethodID, accountID)
	if err != nil {
		return nil, nil, "forma de pagamento não encontrada"
	}
	if pm.Kind != models.KindCreditCard || pm.ClosingDay == nil {
		return nil, nil, "compra parcelada exige um cartão de crédito com dia de fechamento"
	}

	cat, err := h.catRepo.FindBySlug(req.Category)
	if err != nil {
		return nil, nil, "categoria inválida: " + req.Category
	}

	p := &models.InstallmentPurchase{
		AccountID:       accountID,
		PaymentMethodID: pm.ID,
		CategoryID:      &cat.ID,
		Description:     req.Description,
		Merchant:        req.Merchant,
		TotalAmount:     req.TotalAmount,
		Installments:    req.Installments,
		PurchaseDate:    date,
	}
	txs := installments.GenerateTransactions(p, *pm.ClosingDay, userID, cat.Slug)
	return p, txs, ""
}

// POST /installment-purchases
func (h *InstallmentPurchaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())

	var req installmentPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	p, txs, msg := h.buildPurchaseAndTxs(accountID, userID, req)
	if msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.repo.Create(p, txs); err != nil {
		log.Error().Err(err).Msg("installment purchase: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, p)
}

// GET /installment-purchases
func (h *InstallmentPurchaseHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	items, err := h.repo.ListByAccount(accountID)
	if err != nil {
		log.Error().Err(err).Msg("installment purchase: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, items)
}

// GET /installment-purchases/{id}
func (h *InstallmentPurchaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}
	p, err := h.repo.FindByID(id, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "compra parcelada não encontrada")
			return
		}
		log.Error().Err(err).Msg("installment purchase: get")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	txs, err := h.repo.ListTransactions(id, accountID)
	if err != nil {
		log.Error().Err(err).Msg("installment purchase: get transactions")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"purchase": p, "installments": txs})
}

// PUT /installment-purchases/{id}
func (h *InstallmentPurchaseHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if _, err := h.repo.FindByID(id, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "compra parcelada não encontrada")
			return
		}
		log.Error().Err(err).Msg("installment purchase: update find")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req installmentPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	p, txs, msg := h.buildPurchaseAndTxs(accountID, userID, req)
	if msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}
	p.ID = id // regenerar sobre o mesmo pai
	if err := h.repo.Regenerate(p, txs, accountID); err != nil {
		log.Error().Err(err).Msg("installment purchase: regenerate")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, p)
}

// DELETE /installment-purchases/{id}
func (h *InstallmentPurchaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if err := h.repo.Delete(id, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "compra parcelada não encontrada")
			return
		}
		log.Error().Err(err).Msg("installment purchase: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
```

- [ ] **Step 2: Wire repo, handler e rotas no main.go**

Em `backend/cmd/server/main.go`:

(a) Onde os repos são construídos (perto de `recurringRepo := ...`), adicione:
```go
	installmentRepo := repository.NewInstallmentPurchaseRepository(db)
```

(b) Onde os handlers são construídos (perto de `recurringHandler := ...`, linha ~206):
```go
	installmentHandler := handlers.NewInstallmentPurchaseHandler(installmentRepo, pmRepo, catRepo)
```

(c) Dentro do grupo de rotas autenticadas (perto de `r.Route("/recurring-expenses", ...)`, linha ~344), adicione:
```go
		r.Route("/installment-purchases", func(r chi.Router) {
			r.Get("/", installmentHandler.List)
			r.Post("/", installmentHandler.Create)
			r.Get("/{id}", installmentHandler.Get)
			r.Put("/{id}", installmentHandler.Update)
			r.Delete("/{id}", installmentHandler.Delete)
		})
```

- [ ] **Step 3: Compilar**

Run: `cd backend && go build ./...`
Expected: compila sem erros.

- [ ] **Step 4: Teste manual de fumaça (com servidor + cartão cadastrado)**

Run (substitua TOKEN, PM_ID de um cartão de crédito com closing_day, e a URL):
```bash
curl -s -X POST localhost:8080/installment-purchases \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"payment_method_id":"PM_ID","category":"eletronicos","description":"TV","total_amount":1000,"installments":3,"date":"2026-05-10"}' | jq
```
Expected: 201 com a compra; `GET /installment-purchases/{id}` mostra 3 parcelas com billing meses maio/junho/julho e valores 333.33/333.33/333.34 (negativos).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/installment_purchase.go backend/cmd/server/main.go
git commit -m "feat(api): rotas CRUD de compra parcelada"
```

---

## FASE 2 — Entrada via chat (agente)

### Task 8: Write tool `create_installment_purchase`

A tool resolve o **cartão de crédito** seguindo o padrão do app (o modelo não escolhe cartão por ID). Precedência: se houver exatamente 1 cartão de crédito ativo, usa ele; senão devolve uma mensagem listando os cartões para o agente perguntar, e o usuário responde com o nome — a tool casa por nome (case-insensitive, match único). O campo `card_name` é opcional e usado quando há mais de um cartão.

**Files:**
- Create: `backend/internal/agent/tools_installment.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Implementar a tool**

`backend/internal/agent/tools_installment.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/installments"
)

type createInstallmentPurchaseTool struct {
	repo    *repository.InstallmentPurchaseRepository
	pmRepo  *repository.PaymentMethodRepository
	catRepo *repository.CategoryRepository
}

func NewCreateInstallmentPurchaseTool(
	repo *repository.InstallmentPurchaseRepository,
	pmRepo *repository.PaymentMethodRepository,
	catRepo *repository.CategoryRepository,
) WriteTool {
	return &createInstallmentPurchaseTool{repo: repo, pmRepo: pmRepo, catRepo: catRepo}
}

func (t *createInstallmentPurchaseTool) Name() string { return "create_installment_purchase" }

func (t *createInstallmentPurchaseTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "create_installment_purchase",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Registra uma compra ÚNICA PARCELADA no cartão de crédito " +
			"(ex.: \"comprei uma TV de 3000 em 10x\"). Gera N parcelas em faturas consecutivas. " +
			"Use SOMENTE para compras parceladas no crédito — não use para gasto fixo/assinatura nem compra à vista. " +
			"Se o usuário tiver mais de um cartão de crédito e não disser qual, informe card_name com o nome que ele citar.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"total_amount": map[string]any{"type": "number", "description": "Valor TOTAL da compra (positivo)"},
				"installments": map[string]any{"type": "integer", "description": "Número de parcelas (>= 2)"},
				"category":     map[string]any{"type": "string", "description": "slug da categoria"},
				"description":  map[string]any{"type": "string"},
				"merchant":     map[string]any{"type": "string"},
				"date":         map[string]any{"type": "string", "description": "ISO 8601 (YYYY-MM-DD); data da compra"},
				"card_name":    map[string]any{"type": "string", "description": "Nome do cartão de crédito, se o usuário citou"},
			},
			"required": []string{"total_amount", "installments", "category", "date"},
		}),
	}
}

type installmentToolInput struct {
	TotalAmount  float64 `json:"total_amount"`
	Installments int     `json:"installments"`
	Category     string  `json:"category"`
	Description  string  `json:"description"`
	Merchant     string  `json:"merchant"`
	Date         string  `json:"date"`
	CardName     string  `json:"card_name"`
}

func (t *createInstallmentPurchaseTool) Preview(input json.RawMessage) (string, error) {
	var p installmentToolInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_installment_purchase: parse preview: %w", err)
	}
	label := p.Description
	if label == "" {
		label = p.Merchant
	}
	return fmt.Sprintf("Compra parcelada: R$ %.2f em %dx — %s", math.Abs(p.TotalAmount), p.Installments, label), nil
}

// resolveCreditCard escolhe um cartão de crédito ativo. Devolve (pm, "", nil) quando
// resolvido; (nil, msg, nil) quando precisa que o usuário escolha; (nil, "", err) em erro.
func (t *createInstallmentPurchaseTool) resolveCreditCard(accountID uuid.UUID, cardName string) (*models.PaymentMethod, string, error) {
	pms, err := t.pmRepo.ListActive(accountID)
	if err != nil {
		return nil, "", err
	}
	var cards []models.PaymentMethod
	for _, pm := range pms {
		if pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
			cards = append(cards, pm)
		}
	}
	if len(cards) == 0 {
		return nil, "", fmt.Errorf("nenhum cartão de crédito ativo com dia de fechamento — cadastre um antes de parcelar")
	}
	if cardName != "" {
		var matches []models.PaymentMethod
		for _, c := range cards {
			if strings.Contains(strings.ToLower(c.Name), strings.ToLower(cardName)) {
				matches = append(matches, c)
			}
		}
		if len(matches) == 1 {
			return &matches[0], "", nil
		}
	}
	if len(cards) == 1 {
		return &cards[0], "", nil
	}
	names := make([]string, len(cards))
	for i, c := range cards {
		names[i] = c.Name
	}
	return nil, fmt.Sprintf("Em qual cartão foi a compra parcelada? Opções: %s", strings.Join(names, ", ")), nil
}

func (t *createInstallmentPurchaseTool) Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p installmentToolInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_installment_purchase: parse input: %w", err)
	}
	if p.TotalAmount <= 0 {
		return "", fmt.Errorf("total_amount deve ser maior que zero")
	}
	if p.Installments < 2 {
		return "", fmt.Errorf("installments deve ser >= 2")
	}
	date, err := time.Parse("2006-01-02", p.Date)
	if err != nil {
		return "", fmt.Errorf("date deve estar no formato YYYY-MM-DD")
	}
	cat, err := t.catRepo.FindBySlug(p.Category)
	if err != nil {
		return "", fmt.Errorf("categoria desconhecida: %q", p.Category)
	}

	card, ask, err := t.resolveCreditCard(accountID, p.CardName)
	if err != nil {
		return "", err
	}
	if ask != "" {
		return ask, nil // agente repassa a pergunta ao usuário; re-invoca com card_name
	}

	desc := p.Description
	if desc == "" {
		desc = p.Merchant
	}
	purchase := &models.InstallmentPurchase{
		AccountID:       accountID,
		PaymentMethodID: card.ID,
		CategoryID:      &cat.ID,
		Description:     desc,
		Merchant:        p.Merchant,
		TotalAmount:     p.TotalAmount,
		Installments:    p.Installments,
		PurchaseDate:    date,
	}
	userID := auth.UserIDFromContext(ctx)
	txs := installments.GenerateTransactions(purchase, *card.ClosingDay, userID, cat.Slug)
	if err := t.repo.Create(purchase, txs); err != nil {
		return "", fmt.Errorf("create_installment_purchase: persistir: %w", err)
	}
	return fmt.Sprintf("Compra parcelada criada: R$ %.2f em %dx no %s (parcela ~R$ %.2f).",
		p.TotalAmount, p.Installments, card.Name, p.TotalAmount/float64(p.Installments)), nil
}
```

- [ ] **Step 2: Registrar a tool no main.go**

Em `backend/cmd/server/main.go`, no slice `writeTools := []agent.WriteTool{...}` (linha ~178), adicione:
```go
		agent.NewCreateInstallmentPurchaseTool(installmentRepo, pmRepo, catRepo),
```

- [ ] **Step 3: Compilar**

Run: `cd backend && go build ./...`
Expected: compila sem erros.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/agent/tools_installment.go backend/cmd/server/main.go
git commit -m "feat(agent): tool create_installment_purchase"
```

---

### Task 9: Aposentar o parcelamento via prompt

**Files:**
- Modify: `backend/internal/services/rag/context.go`
- Modify: `backend/internal/agent/tools_write.go`

- [ ] **Step 1: Remover o helper `InstallmentDates`**

Em `backend/internal/services/rag/context.go`, remova a função `InstallmentDates` (linhas ~77-92, o bloco completo com o comentário doc).

- [ ] **Step 2: Remover as instruções de parcelamento do prompt**

No mesmo arquivo, no `buildPrompt`:

(a) Remova as linhas do bloco "Quando o usuário mencionar parcelado" e o exemplo que usa `InstallmentDates` (linhas ~204-216 — da `sb.WriteString("- Quando o usuário mencionar parcelado...")` até o `))` que fecha o `fmt.Sprintf` do exemplo, inclusive a atribuição `installDates := InstallmentDates(now, 3)`).

(b) Substitua, no mesmo lugar, por uma única instrução apontando para a nova tool:
```go
	sb.WriteString("- Compra ÚNICA PARCELADA no cartão de crédito (ex.: \"comprei uma TV de 3000 em 10x\"): chame create_installment_purchase (NÃO crie várias create_transaction).\n")
```

(c) Na linha ~199 (`Para parcelamento: replacements[0].date = ...`), remova essa linha — `replace_transaction` deixa de ser caminho de parcelamento.

(d) Na linha ~193 (`Se o usuário disser que uma transação JÁ REGISTRADA estava errada (valor, data, categoria) ou foi parcelada:`), remova o trecho `ou foi parcelada`.

- [ ] **Step 3: Ajustar a descrição de `replace_transaction`**

Em `backend/internal/agent/tools_write.go` (linha ~787), troque:
```go
			"Use para: corrigir valor, data ou categoria; parcelar retroativamente. " +
```
por:
```go
			"Use para: corrigir valor, data ou categoria de uma transação existente. " +
```

- [ ] **Step 4: Compilar**

Run: `cd backend && go build ./...`
Expected: compila sem erros (sem referências órfãs a `InstallmentDates`). Se algum teste referenciar `InstallmentDates`, atualize-o/remova-o.

- [ ] **Step 5: Verificar que nada mais usa `InstallmentDates`**

Run: `cd backend && grep -rn "InstallmentDates" .`
Expected: nenhum resultado.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/rag/context.go backend/internal/agent/tools_write.go
git commit -m "refactor(agent): aposenta parcelamento via prompt em favor de create_installment_purchase"
```

---

## FASE 3 — Entrada via foto (instrução capta N)

### Task 10: OCR ecoa `parcelas` da instrução

**Files:**
- Modify: `backend/internal/services/vision/ocr.go`

- [ ] **Step 1: Adicionar o campo à struct de saída do OCR**

Em `backend/internal/services/vision/ocr.go`, na struct do item de OCR (onde está `FormaPagamentoDetectada *string` ~linha 84), adicione:
```go
	// Parcelas é o número de parcelas, ecoado da instrução do usuário (não lido do recibo).
	Parcelas *int `json:"parcelas,omitempty"`
```

- [ ] **Step 2: Instruir o modelo a ecoar o N da instrução**

No prompt do OCR (perto da instrução 11/12, ~linhas 43-55), adicione uma linha:
```go
	sb.WriteString("13. Se a instrução do usuário mencionar parcelamento (ex.: \"parcelado em 10x\", \"em 10 vezes\"), preencha \"parcelas\" com esse número inteiro. NÃO procure parcelamento na imagem do recibo — a fonte é a frase do usuário. Se não houver, devolva null.\n")
```
(Ajuste o número da instrução conforme a numeração real do arquivo. Inclua `parcelas` no exemplo de formato JSON da instrução 5, ~linha 37, adicionando `, "parcelas": int|null`.)

- [ ] **Step 3: Compilar**

Run: `cd backend && go build ./...`
Expected: compila sem erros.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/vision/ocr.go
git commit -m "feat(ocr): captura nº de parcelas ecoado da instrução do usuário"
```

---

### Task 11: Upload cria compra parcelada quando `parcelas > 1`

**Files:**
- Modify: `backend/internal/handlers/upload.go`

Contexto: hoje o `upload.go` monta um `draft *models.Transaction` por item e o cria/clarifica. Quando o item vier com `Parcelas > 1` E o cartão estiver resolvido (`draft.PaymentMethodID != nil`, cartão de crédito com `closing_day`), em vez de criar uma transação avulsa, criamos uma compra parcelada. Para o MVP, só tratamos o caminho "modo padrão" (single receipt → `Extract`), onde há um único `draft`. Se o cartão não estiver resolvido, cai no fluxo de clarify atual (a parcelização pode ser refeita depois pelo chat) — documente isso com um log.

- [ ] **Step 1: Injetar as dependências necessárias no handler**

Confirme que o `UploadHandler` tem acesso a `installmentRepo *repository.InstallmentPurchaseRepository` e `catRepo`. Se não tiver, adicione o campo na struct do handler e no construtor, e passe `installmentRepo` no wiring do `main.go` onde o `uploadHandler` é construído.

- [ ] **Step 2: Ramificar no ponto de criação do draft (modo padrão)**

No bloco do modo padrão (perto de `savedTxs = append(savedTxs, *draft)`, ~linha 400), antes de criar a transação avulsa, insira:

```go
			// Compra parcelada: o usuário disse "parcelado em Nx" na instrução e o
			// cartão de crédito foi resolvido. Cria a compra pai + N parcelas.
			if receipt.Parcelas != nil && *receipt.Parcelas > 1 && draft.PaymentMethodID != nil {
				if pm, perr := h.pmRepo.FindByID(*draft.PaymentMethodID, accountID); perr == nil &&
					pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
					purchase := &models.InstallmentPurchase{
						AccountID:       accountID,
						PaymentMethodID: pm.ID,
						CategoryID:      draft.CategoryID,
						Description:     draft.Description,
						Merchant:        draft.Merchant,
						TotalAmount:     math.Abs(draft.Amount),
						Installments:    *receipt.Parcelas,
						PurchaseDate:    draft.Date,
					}
					txs := installments.GenerateTransactions(purchase, *pm.ClosingDay, userID, draft.Category)
					if err := h.installmentRepo.Create(purchase, txs); err != nil {
						logger.Error().Err(err).Msg("upload: create installment purchase")
						respondError(w, http.StatusInternalServerError, "não consegui registrar a compra parcelada")
						return
					}
					logger.Info().Int("parcelas", *receipt.Parcelas).Msg("upload: compra parcelada criada")
					respondJSON(w, http.StatusOK, map[string]any{"installment_purchase": purchase})
					return
				}
				logger.Warn().Msg("upload: parcelas informadas mas cartão não resolvido — caindo no fluxo avulso")
			}
```

Adicione os imports necessários no topo de `upload.go`: `"math"`, `"github.com/marianamohr/financas/internal/services/installments"` (e `models` já deve estar).

Nota de revisão: confirme os nomes exatos das variáveis no escopo (`receipt`, `draft`, `userID`, `accountID`, `logger`, `h.pmRepo`, `h.installmentRepo`) — ajuste conforme o arquivo. No modo `food_detail` (múltiplos itens) este MVP não trata parcelamento; documente com um comentário.

- [ ] **Step 3: Compilar**

Run: `cd backend && go build ./...`
Expected: compila sem erros.

- [ ] **Step 4: Teste manual de fumaça**

Suba uma foto de nota com `instruction = "parcelado em 3x"` e um cartão de crédito mencionado/resolvível. Confirme que `GET /installment-purchases` lista a nova compra com 3 parcelas.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/upload.go backend/cmd/server/main.go
git commit -m "feat(upload): cria compra parcelada quando a instrução informa N parcelas"
```

---

## FASE 4 — Frontend: rótulo "k/N"

### Task 12: Mostrar a parcela no card da transação

**Files:**
- Modify: o componente de card de transação no frontend (provavelmente `frontend/src/components/TransactionCard.tsx` ou equivalente — localize com grep).

- [ ] **Step 1: Localizar o componente e o tipo de transação**

Run: `grep -rln "installment\|InstallmentNumber\|payment_method_id" frontend/src | head` e `grep -rln "TransactionCard\|interface Transaction" frontend/src | head`
Identifique onde a transação é renderizada e onde o tipo TS é declarado.

- [ ] **Step 2: Adicionar os campos ao tipo TS**

No tipo `Transaction` do frontend, adicione:
```ts
  installment_purchase_id?: string | null;
  installment_number?: number | null;
```

- [ ] **Step 3: Renderizar o rótulo**

No card, quando `installment_number` existir, exiba um chip discreto, ex.:
```tsx
{tx.installment_number != null && (
  <span className="installment-badge">parcela {tx.installment_number}</span>
)}
```
(A descrição já contém "(k/N)" vinda do backend; o badge é opcional/complementar. Siga o estilo dos chips existentes no card.)

- [ ] **Step 4: Build do frontend**

Run: `cd frontend && npm run build` (ou o comando de build/typecheck do projeto)
Expected: build/typecheck sem erros.

- [ ] **Step 5: Commit**

```bash
git add frontend/
git commit -m "feat(ui): rótulo de parcela no card da transação"
```

---

## Self-Review (preenchido pelo autor do plano)

**Cobertura do spec:**
- Modelo de dados (tabela pai + colunas) → Tasks 1, 5.
- Lógica de geração (split resto-na-última, billing months, date=1º dia) → Tasks 2, 3, 4.
- Backend repo/handlers/rotas → Tasks 6, 7.
- Entrada chat → Task 8; aposentar prompt → Task 9.
- Entrada foto (N da instrução) → Tasks 10, 11.
- Editar/excluir tudo-ou-nada → Task 6 (Delete/Regenerate) + Task 7 (rotas).
- Visões sem tela nova → coberto pela convenção `date`=1º dia + `billing_month` (Task 4); UI label → Task 12.
- Testes → unidade Tasks 2-4; integração Task 6; fumaça Tasks 7, 11.

**Desvio do spec registrado:** o spec dizia "resolve o cartão por nome" no chat; a realidade do código é que o agente não escolhe cartão por ID. A Task 8 implementa resolução por nº de cartões ativos + `card_name` opcional com pergunta de volta — fiel à arquitetura existente (validator/clarify). Sem entidade nova de pending action (mantém escopo).

**Placeholders:** nenhum "TODO"/"TBD"; todo passo de código tem código real.

**Consistência de tipos:** `GenerateTransactions(p *models.InstallmentPurchase, closingDay int, userID uuid.UUID, categorySlug string)` usado igual em Tasks 4, 7, 8, 11. `SplitCents(int64,int)`, `BillingMonths(int,time.Month,int)`, `BillingMonth{Year,Month}` consistentes. Repo: `Create(p, txs)`, `Delete(id, accountID)`, `Regenerate(p, txs, accountID)`, `FindByID`, `ListByAccount`, `ListTransactions` consistentes entre Tasks 6 e 7.

## Pontos a verificar na execução (não bloqueiam o plano)
- Nome real do helper de DB nos testes de integração do pacote `repository` (Task 6 Step 1).
- Numeração exata das instruções no prompt do OCR (Task 10).
- Nomes reais de variáveis em `upload.go` e se `UploadHandler` já tem `pmRepo`/precisa de `installmentRepo` (Task 11).
- Caminho/Nome do componente de card no frontend (Task 12).
