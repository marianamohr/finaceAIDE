# Payment Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adicionar forma de pagamento (crédito, débito, pix, VR, boleto) às transações, com FK para nova tabela `payment_modes`, endpoint de listagem, campo no modal de edição do Dashboard, e reconhecimento automático no agente de chat.

**Architecture:** Nova tabela `payment_modes` com seed idempotente, seguindo o padrão de `Category`. `transactions` ganha `payment_mode_id` nullable via `addNullableColumns`. O agente de chat recebe instrução no system prompt para reconhecer termos e perguntar quando não informado.

**Tech Stack:** Go, GORM, PostgreSQL, React, TypeScript, TanStack Query, chi router

---

## File Map

| Arquivo | Ação |
|---|---|
| `backend/internal/models/payment_mode.go` | Criar — modelo `PaymentMode` |
| `backend/internal/models/transaction.go` | Modificar — adicionar `PaymentModeID` e `PaymentModeRef` |
| `backend/internal/repository/payment_mode.go` | Criar — `PaymentModeRepository` com `ListAll` |
| `backend/migrations/migrate.go` | Modificar — AutoMigrate, seed, addNullableColumns |
| `backend/internal/handlers/payment_mode.go` | Criar — `GET /payment-modes` |
| `backend/cmd/server/main.go` | Modificar — wiring do novo handler e rota |
| `backend/internal/handlers/transaction.go` | Modificar — aceitar `payment_mode_id` em Create/Update |
| `backend/internal/services/rag/context.go` | Modificar — instruções de payment_mode no system prompt |
| `backend/internal/agent/tools_write.go` | Modificar — `payment_mode_slug` no schema de `create_transaction` |
| `frontend/src/hooks/useDashboard.ts` | Modificar — adicionar `payment_mode_id` ao tipo `Transaction` |
| `frontend/src/pages/Dashboard.tsx` | Modificar — buscar payment modes, campo no form de edição, exibição no modo visualização |

---

## Task 1: Modelo Go e migração

**Files:**
- Create: `backend/internal/models/payment_mode.go`
- Modify: `backend/internal/models/transaction.go`
- Modify: `backend/migrations/migrate.go`

- [ ] **Step 1: Criar `models/payment_mode.go`**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentMode struct {
	ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Slug      string    `gorm:"not null;uniqueIndex"                           json:"slug"`
	Label     string    `gorm:"not null"                                       json:"label"`
	CreatedAt time.Time `                                                      json:"created_at"`
}
```

- [ ] **Step 2: Adicionar campos em `transaction.go`**

Adicionar após o campo `RecurringExpenseID` (linha 26), antes do bloco `// Multi-currency fields`:

```go
PaymentModeID  *uuid.UUID   `gorm:"type:uuid;index"                       json:"payment_mode_id,omitempty"`
PaymentModeRef *PaymentMode `gorm:"foreignKey:PaymentModeID;references:ID" json:"-"`
```

- [ ] **Step 3: Atualizar `migrations/migrate.go` — AutoMigrate inicial**

Adicionar `&models.PaymentMode{}` **antes** de `&models.Transaction{}` no primeiro bloco `AutoMigrate` (linha 18):

```go
if err := db.AutoMigrate(
    &models.Category{},
    &models.Account{},
    &models.Invite{},
    &models.User{},
    &models.PaymentMode{},   // ← novo, antes de Transaction
    &models.Transaction{},
    // ... resto igual
```

- [ ] **Step 4: Adicionar `seedPaymentModes` e chamada no `Run`**

Adicionar a função no final do arquivo `migrate.go`:

```go
func seedPaymentModes(db *gorm.DB) error {
	rows := []struct{ slug, label string }{
		{"credito", "crédito"},
		{"debito", "débito"},
		{"pix", "pix"},
		{"vr", "VR"},
		{"boleto", "boleto"},
	}
	for _, r := range rows {
		if err := db.Exec(`
			INSERT INTO payment_modes (id, slug, label, created_at)
			VALUES (gen_random_uuid(), ?, ?, NOW())
			ON CONFLICT (slug) DO NOTHING
		`, r.slug, r.label).Error; err != nil {
			return fmt.Errorf("seed payment mode %q: %w", r.slug, err)
		}
	}
	return nil
}
```

Adicionar chamada no `Run()` após o `AutoMigrate` inicial e antes de `addNullableColumns` (após linha 38, antes da chamada `addNullableColumns`):

```go
if err := seedPaymentModes(db); err != nil {
    return fmt.Errorf("migrations: seed payment modes: %w", err)
}
```

- [ ] **Step 5: Adicionar coluna em `addNullableColumns`**

No slice `stmts` de `addNullableColumns`, adicionar ao final (antes do `}`):

```go
`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS payment_mode_id uuid REFERENCES payment_modes(id)`,
```

- [ ] **Step 6: Adicionar `&models.PaymentMode{}` no AutoMigrate final**

No segundo bloco `AutoMigrate` (linha 85), adicionar `&models.PaymentMode{}`:

```go
if err := db.AutoMigrate(
    &models.PaymentMode{},   // ← novo
    &models.User{},
    &models.Transaction{},
    // ... resto igual
```

- [ ] **Step 7: Compilar o backend**

```bash
cd backend && go build ./...
```

Esperado: sem erros de compilação.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/models/payment_mode.go backend/internal/models/transaction.go backend/migrations/migrate.go
git commit -m "feat(payment-mode): add PaymentMode model and migration"
```

---

## Task 2: Repository e handler GET /payment-modes

**Files:**
- Create: `backend/internal/repository/payment_mode.go`
- Create: `backend/internal/handlers/payment_mode.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Criar `repository/payment_mode.go`**

```go
package repository

import (
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type PaymentModeRepository struct {
	db *gorm.DB
}

func NewPaymentModeRepository(db *gorm.DB) *PaymentModeRepository {
	return &PaymentModeRepository{db: db}
}

func (r *PaymentModeRepository) ListAll() ([]models.PaymentMode, error) {
	var modes []models.PaymentMode
	if err := r.db.Order("slug").Find(&modes).Error; err != nil {
		return nil, err
	}
	return modes, nil
}
```

- [ ] **Step 2: Criar `handlers/payment_mode.go`**

```go
package handlers

import (
	"net/http"

	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type PaymentModeHandler struct {
	repo *repository.PaymentModeRepository
}

func NewPaymentModeHandler(repo *repository.PaymentModeRepository) *PaymentModeHandler {
	return &PaymentModeHandler{repo: repo}
}

func (h *PaymentModeHandler) List(w http.ResponseWriter, r *http.Request) {
	modes, err := h.repo.ListAll()
	if err != nil {
		log.Error().Err(err).Msg("payment-modes: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, modes)
}
```

- [ ] **Step 3: Registrar no `main.go`**

Em `main.go`, após `catRepo` (linha 95), adicionar:

```go
paymentModeRepo := repository.NewPaymentModeRepository(db)
```

Após `txHandler := handlers.NewTransactionHandler(txRepo, catRepo)` (linha 189), adicionar:

```go
paymentModeHandler := handlers.NewPaymentModeHandler(paymentModeRepo)
```

Na seção de rotas protegidas, após `r.Get("/categories", txHandler.ListCategories)` (linha 261), adicionar:

```go
r.Get("/payment-modes", paymentModeHandler.List)
```

- [ ] **Step 4: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repository/payment_mode.go backend/internal/handlers/payment_mode.go backend/cmd/server/main.go
git commit -m "feat(payment-mode): add repository, handler and GET /payment-modes route"
```

---

## Task 3: Aceitar `payment_mode_id` em Create/Update de transações

**Files:**
- Modify: `backend/internal/handlers/transaction.go`

- [ ] **Step 1: Adicionar `PaymentModeID` ao `transactionRequest`**

Localizar a struct `transactionRequest` (linha 30) e adicionar o campo:

```go
type transactionRequest struct {
	Amount        float64    `json:"amount"`
	Category      string     `json:"category"`
	Merchant      string     `json:"merchant"`
	Description   string     `json:"description"`
	Date          string     `json:"date"`
	Source        string     `json:"source"`
	PaymentModeID *uuid.UUID `json:"payment_mode_id"`
}
```

- [ ] **Step 2: Propagar `PaymentModeID` no método `Create`**

No bloco de construção do `tx` em `Create` (após linha 127), adicionar o campo:

```go
tx := &models.Transaction{
    UserID:        userID,
    AccountID:     accountID,
    Amount:        req.Amount,
    Category:      cat.Slug,
    CategoryID:    &cat.ID,
    Merchant:      req.Merchant,
    Description:   req.Description,
    Date:          date,
    Source:        source,
    PaymentModeID: req.PaymentModeID,
}
```

- [ ] **Step 3: Propagar `PaymentModeID` no método `Update`**

No bloco de construção do `tx` em `Update` (após linha 213), adicionar o campo:

```go
tx := &models.Transaction{
    ID:            id,
    UserID:        userID,
    AccountID:     accountID,
    Amount:        req.Amount,
    Category:      cat.Slug,
    CategoryID:    &cat.ID,
    Merchant:      req.Merchant,
    Description:   req.Description,
    Date:          date,
    Source:        req.Source,
    PaymentModeID: req.PaymentModeID,
}
```

- [ ] **Step 4: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/transaction.go
git commit -m "feat(payment-mode): accept payment_mode_id in transaction create/update"
```

---

## Task 4: Agente de chat — reconhecer forma de pagamento

**Files:**
- Modify: `backend/internal/agent/tools_write.go`
- Modify: `backend/internal/services/rag/context.go`

- [ ] **Step 1: Adicionar `payment_mode_slug` ao schema de `create_transaction`**

Em `tools_write.go`, no método `Schema()` de `createTransactionTool` (linha 76), adicionar `payment_mode_slug` no `InputSchema`:

```go
InputSchema: mustMarshal(map[string]any{
    "type": "object",
    "properties": map[string]any{
        "amount":             map[string]any{"type": "number", "description": "Valor negativo para despesa, positivo para receita"},
        "category":           map[string]any{"type": "string"},
        "description":        map[string]any{"type": "string"},
        "date":               map[string]any{"type": "string", "description": "ISO 8601 (YYYY-MM-DD)"},
        "merchant":           map[string]any{"type": "string"},
        "payment_mode_slug":  map[string]any{"type": "string", "description": "Forma de pagamento: credito, debito, pix, vr, boleto. Omitir se não informado pelo usuário."},
    },
    "required": []string{"amount", "category", "date"},
}),
```

- [ ] **Step 2: Receber `payment_mode_slug` no `Execute` de `create_transaction`**

O `Execute` precisa de acesso ao `PaymentModeRepository` para resolver o slug para um UUID. Primeiro, adicionar o repo à struct:

```go
type createTransactionTool struct {
	repo            *repository.TransactionRepository
	catRepo         *repository.CategoryRepository
	currencySvc     *currency.Service
	paymentModeRepo *repository.PaymentModeRepository
}
```

Atualizar o construtor:

```go
func NewCreateTransactionTool(repo *repository.TransactionRepository, catRepo *repository.CategoryRepository, currencySvc *currency.Service, paymentModeRepo *repository.PaymentModeRepository) WriteTool {
	return &createTransactionTool{repo: repo, catRepo: catRepo, currencySvc: currencySvc, paymentModeRepo: paymentModeRepo}
}
```

- [ ] **Step 3: Usar `payment_mode_slug` no `Execute`**

No método `Execute` de `createTransactionTool`, estender o parsing do input e a criação da transaction:

```go
func (t *createTransactionTool) Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		Amount          float64 `json:"amount"`
		Category        string  `json:"category"`
		Description     string  `json:"description"`
		Date            string  `json:"date"`
		Merchant        string  `json:"merchant"`
		PaymentModeSlug string  `json:"payment_mode_slug"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_transaction: parse input: %w", err)
	}
	cat, err := resolveCategoryForAmount(t.catRepo, p.Category, p.Amount)
	if err != nil {
		return "", fmt.Errorf("create_transaction: %w", err)
	}
	date := parseDateNoon(p.Date)
	cur := userCurrency(ctx, t.currencySvc)

	var paymentModeID *uuid.UUID
	if p.PaymentModeSlug != "" {
		modes, err := t.paymentModeRepo.ListAll()
		if err == nil {
			for _, m := range modes {
				if m.Slug == p.PaymentModeSlug {
					id := m.ID
					paymentModeID = &id
					break
				}
			}
		}
	}

	tx := &models.Transaction{
		AccountID:      accountID,
		UserID:         auth.UserIDFromContext(ctx),
		Amount:         p.Amount,
		Category:       cat.Slug,
		CategoryID:     &cat.ID,
		Description:    p.Description,
		Merchant:       p.Merchant,
		Date:           date,
		Source:         "chat",
		Currency:       cur,
		Locale:         currencyToLocale(cur),
		OriginalAmount: math.Abs(p.Amount),
		PaymentModeID:  paymentModeID,
	}
	if err := t.repo.Create(tx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Transação criada: R$ %.2f — %s", math.Abs(p.Amount), cat.Slug), nil
}
```

- [ ] **Step 4: Atualizar wiring em `main.go`**

Em `main.go`, o `NewCreateTransactionTool` agora requer `paymentModeRepo`. Atualizar a linha de criação (dentro do slice `writeTools`):

```go
agent.NewCreateTransactionTool(txRepo, catRepo, currencyService, paymentModeRepo),
```

- [ ] **Step 5: Adicionar instruções no system prompt (`context.go`)**

No método `buildPrompt` em `context.go`, antes da linha `return sb.String()` (linha 217), adicionar a seção de formas de pagamento:

```go
sb.WriteString("\n== FORMAS DE PAGAMENTO ==\n")
sb.WriteString("Ao registrar uma DESPESA, você DEVE informar o campo payment_mode_slug em create_transaction.\n")
sb.WriteString("Reconheça os seguintes termos do usuário e mapeie para o slug correto:\n")
sb.WriteString("  - \"crédito\", \"cartão de crédito\" → credito\n")
sb.WriteString("  - \"débito\", \"cartão de débito\", \"débito automático\" → debito\n")
sb.WriteString("  - \"pix\" → pix\n")
sb.WriteString("  - \"vr\", \"vale refeição\", \"vale alimentação\" → vr\n")
sb.WriteString("  - \"boleto\" → boleto\n")
sb.WriteString("- Se o usuário mencionar apenas \"cartão\" sem especificar: PERGUNTE se é crédito ou débito antes de criar a transação.\n")
sb.WriteString("- Se nenhuma forma de pagamento for mencionada para uma DESPESA: PERGUNTE antes de criar a transação.\n")
sb.WriteString("- Para receitas e transferências internas (type=transfer): NÃO pergunte — omita payment_mode_slug.\n")
```

- [ ] **Step 6: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/agent/tools_write.go backend/internal/services/rag/context.go backend/cmd/server/main.go
git commit -m "feat(payment-mode): add payment_mode_slug to create_transaction tool and chat instructions"
```

---

## Task 5: Frontend — tipo Transaction e hook de payment modes

**Files:**
- Modify: `frontend/src/hooks/useDashboard.ts`

- [ ] **Step 1: Adicionar `payment_mode_id` ao tipo `Transaction`**

Em `useDashboard.ts`, na interface `Transaction` (linha 5), adicionar o campo:

```typescript
export interface Transaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
  image_url: string;
  payment_mode_id?: string;
}
```

- [ ] **Step 2: Criar tipo `PaymentMode`**

No mesmo arquivo, após a interface `Transaction`, adicionar:

```typescript
export interface PaymentMode {
  id: string;
  slug: string;
  label: string;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useDashboard.ts
git commit -m "feat(payment-mode): add payment_mode_id to Transaction type"
```

---

## Task 6: Frontend — modal de edição no Dashboard

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

- [ ] **Step 1: Importar `PaymentMode` do hook**

Na linha 8 de `Dashboard.tsx`, onde está o import de `useDashboard`, adicionar `PaymentMode`:

```typescript
import { useDashboardQuery, useInvalidateDashboard, usePrefetchNextMonth, Transaction, PaymentMode } from "@/hooks/useDashboard";
```

- [ ] **Step 2: Adicionar state de payment modes**

Após o state `allCategories` (linha 77), adicionar:

```typescript
const [paymentModes, setPaymentModes] = useState<PaymentMode[]>([]);

useEffect(() => {
  api.get<PaymentMode[]>("/payment-modes")
    .then(setPaymentModes).catch(() => {});
}, []);
```

- [ ] **Step 3: Inicializar `payment_mode_id` no `handleEditStart`**

No `handleEditStart` (linha 190), adicionar o campo ao `setEditForm`:

```typescript
const handleEditStart = () => {
  if (!selectedTx) return;
  setEditForm({
    amount: selectedTx.amount,
    category: selectedTx.category,
    merchant: selectedTx.merchant,
    description: selectedTx.description,
    date: selectedTx.date.slice(0, 10),
    payment_mode_id: selectedTx.payment_mode_id,
  });
  setIsEditing(true);
};
```

- [ ] **Step 4: Incluir `payment_mode_id` no payload de `updateTransaction`**

Na função `updateTransaction` (linha 156), adicionar o campo:

```typescript
const updateTransaction = async (id: string, data: Partial<Omit<Transaction, "id" | "source" | "image_url">>) => {
  await api.put(`/transactions/${id}`, {
    amount: data.amount,
    category: data.category,
    merchant: data.merchant,
    description: data.description,
    date: data.date ? data.date.slice(0, 10) : undefined,
    source: "manual",
    payment_mode_id: data.payment_mode_id ?? null,
  });
  invalidateDashboard();
};
```

- [ ] **Step 5: Adicionar campo no formulário de edição**

No bloco de edição (dentro do `isEditing` do Sheet, após o campo Data, antes dos botões Cancelar/Salvar — por volta da linha 548), adicionar:

```tsx
{selectedTx && selectedTx.amount < 0 && selectedTx.type !== "transfer" && (
  <div className="space-y-1">
    <label className="text-xs text-muted-foreground">Forma de pagamento</label>
    <select
      value={editForm.payment_mode_id ?? ""}
      onChange={(e) => setEditForm((f) => ({ ...f, payment_mode_id: e.target.value || undefined }))}
      className="w-full h-9 rounded-md border border-input bg-background px-3 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
    >
      <option value="">Selecionar</option>
      {paymentModes.map((m) => (
        <option key={m.id} value={m.id}>{m.label}</option>
      ))}
    </select>
  </div>
)}
```

- [ ] **Step 6: Exibir forma de pagamento no modo visualização**

No bloco de visualização (dentro do bloco `else` do `isEditing`, onde ficam os campos `Categoria`, `Valor`, etc. — por volta da linha 568), adicionar após o campo `Categoria`:

```tsx
{selectedTx.payment_mode_id && (
  <div className="flex justify-between text-sm">
    <span className="text-muted-foreground">Forma de pagamento</span>
    <span className="font-medium capitalize">
      {paymentModes.find((m) => m.id === selectedTx.payment_mode_id)?.label ?? "—"}
    </span>
  </div>
)}
```

- [ ] **Step 7: Verificar TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros de tipo.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/hooks/useDashboard.ts
git commit -m "feat(payment-mode): add payment mode selector to transaction edit modal"
```

---

## Self-Review

### Spec coverage

| Requisito | Task |
|---|---|
| Tabela `payment_modes` com seed | Task 1 |
| `payment_mode_id` nullable em `transactions` | Task 1 |
| Modelo Go `PaymentMode` | Task 1 |
| FK em `Transaction` | Task 1 |
| `GET /payment-modes` | Task 2 |
| Create/Update aceitam `payment_mode_id` | Task 3 |
| Agente reconhece termos | Task 4 |
| Agente pergunta quando não informado | Task 4 |
| "cartão" → pergunta crédito ou débito | Task 4 |
| Campo no modal de edição | Task 6 |
| Exibição no modo visualização | Task 6 |
| Somente para despesas no modal | Task 6 (Step 5: condicional `amount < 0`) |

### Consistência de tipos

- `PaymentModeRepository.ListAll()` → retorna `[]models.PaymentMode` — usado em Task 4 Step 3 ✓
- `NewCreateTransactionTool` assinatura atualizada em Task 4 Step 2, wiring atualizado em Task 4 Step 4 ✓
- `PaymentMode` interface criada em Task 5, importada em Task 6 ✓
- `payment_mode_id` como `*uuid.UUID` no modelo Go; como `string` (UUID serializado) no frontend ✓
