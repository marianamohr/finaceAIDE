# Limites de Gasto por Categoria — Plano de Implementação

> **Para agentes:** SUB-SKILL OBRIGATÓRIO: Use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam sintaxe de checkbox (`- [ ]`) para rastreamento.

**Objetivo:** Adicionar limites mensais de gasto por categoria com escopo de conta, aba dedicada no bottom nav e modal de confirmação ao ultrapassar o limite.

**Arquitetura:** Nova tabela `account_category_limits` (account_id + category_id únicos). Backend expõe 4 endpoints REST; o cálculo de gasto reutiliza `TransactionRepository.SummaryByMonth` já existente. Frontend adiciona hook `useLimits`, página `Limites.tsx`, componente `LimitCard` com progresso circular, e sheet de criação/edição.

**Tech Stack:** Go + Chi + GORM (backend), React + TypeScript + Tailwind + shadcn/ui + lucide-react (frontend)

---

## Mapa de Arquivos

**Criar:**
- `backend/internal/models/category_limit.go` — struct `CategoryLimit` e DTO `LimitWithProgress`
- `backend/internal/repository/category_limit.go` — `CategoryLimitRepository` com `ListByAccount`, `Upsert`, `Delete`
- `backend/internal/handlers/category_limit.go` — handlers para os 4 endpoints
- `frontend/src/hooks/useLimits.ts` — hook de API para limites
- `frontend/src/pages/Limites.tsx` — página com `LimitCard` e `LimitSheet`

**Modificar:**
- `backend/migrations/migrate.go` — adicionar `CategoryLimit` ao `AutoMigrate`
- `backend/cmd/server/main.go` — instanciar repo/handler e registrar rotas
- `frontend/src/lib/categories.ts` — adicionar `CATEGORY_EMOJIS` e `EXPENSE_CATEGORIES`
- `frontend/src/store/settings.ts` — adicionar `showLimitsCategories` ao store
- `frontend/src/hooks/useSettings.ts` — sincronizar `showLimitsCategories` com o store
- `frontend/src/components/BottomNav/BottomNav.tsx` — adicionar item "Limites"
- `frontend/src/App.tsx` — adicionar rota `/limites`

---

## Tarefa 1: Model e Migration (Backend)

**Arquivos:**
- Criar: `backend/internal/models/category_limit.go`
- Modificar: `backend/migrations/migrate.go`

- [ ] **Passo 1: Criar o model**

Criar `backend/internal/models/category_limit.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

type CategoryLimit struct {
	ID           uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID    uuid.UUID `gorm:"not null;index;type:uuid"                       json:"account_id"`
	CategoryID   uuid.UUID `gorm:"not null;type:uuid"                             json:"category_id"`
	MonthlyLimit float64   `gorm:"type:numeric(12,2);not null"                    json:"monthly_limit"`
	CreatedAt    time.Time `                                                      json:"created_at"`
	UpdatedAt    time.Time `                                                      json:"updated_at"`
}

// LimitWithProgress é o DTO retornado pelo GET /account/limits.
// Combina o limite com o gasto atual do mês via SummaryByMonth.
type LimitWithProgress struct {
	ID           uuid.UUID `json:"id"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategorySlug string    `json:"category_slug"`
	MonthlyLimit float64   `json:"monthly_limit"`
	Spent        float64   `json:"spent"` // valor absoluto gasto no mês atual
	Pct          float64   `json:"pct"`   // spent / monthly_limit * 100
}
```

- [ ] **Passo 2: Registrar no AutoMigrate**

Em `backend/migrations/migrate.go`, adicionar `&models.CategoryLimit{}` na lista do primeiro `AutoMigrate` (linha ~18), após `&models.PendingAction{}`:

```go
if err := db.AutoMigrate(
    &models.Category{},
    &models.Account{},
    &models.Invite{},
    &models.User{},
    &models.Transaction{},
    &models.ChatMessage{},
    &models.Passkey{},
    &models.WebAuthnSession{},
    &models.UserSettings{},
    &models.ExchangeRateCache{},
    &models.MerchantAlias{},
    &models.Goal{},
    &models.GoalTransaction{},
    &models.AgentInsight{},
    &models.PendingAction{},
    &models.CategoryLimit{}, // novo
); err != nil {
    return fmt.Errorf("migrations: initial auto migrate: %w", err)
}
```

- [ ] **Passo 3: Rodar a migration e verificar**

```bash
cd backend && go run ./cmd/migrate
```

Saída esperada: sem erros. Verificar no banco:

```sql
\d account_category_limits
```

A tabela deve existir com as colunas: `id`, `account_id`, `category_id`, `monthly_limit`, `created_at`, `updated_at`. Confirmar que o GORM criou o índice em `account_id` e a constraint `UNIQUE(account_id, category_id)` — se não criou a unique automaticamente, adicionar migration SQL manual (ver Passo 4).

- [ ] **Passo 4: Adicionar unique constraint manualmente se necessário**

Se a constraint unique não foi criada pelo GORM, adicionar em `addNullableColumns` em `migrate.go`:

```go
db.Exec(`ALTER TABLE account_category_limits
    ADD CONSTRAINT IF NOT EXISTS uq_account_category_limits UNIQUE (account_id, category_id)`)
```

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/models/category_limit.go backend/migrations/migrate.go
git commit -m "feat(backend): add CategoryLimit model and migration"
```

---

## Tarefa 2: Repository (Backend)

**Arquivos:**
- Criar: `backend/internal/repository/category_limit.go`

- [ ] **Passo 1: Criar o repository**

Criar `backend/internal/repository/category_limit.go`:

```go
package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CategoryLimitRepository struct {
	db *gorm.DB
}

func NewCategoryLimitRepository(db *gorm.DB) *CategoryLimitRepository {
	return &CategoryLimitRepository{db: db}
}

// ListByAccount retorna todos os limites da conta.
func (r *CategoryLimitRepository) ListByAccount(accountID uuid.UUID) ([]models.CategoryLimit, error) {
	var limits []models.CategoryLimit
	if err := r.db.Where("account_id = ?", accountID).Find(&limits).Error; err != nil {
		return nil, fmt.Errorf("category limit repository: list: %w", err)
	}
	return limits, nil
}

// Upsert cria ou atualiza um limite. Usa ON CONFLICT para garantir idempotência.
func (r *CategoryLimitRepository) Upsert(limit *models.CategoryLimit) error {
	result := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}, {Name: "category_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"monthly_limit", "updated_at"}),
	}).Create(limit)
	if result.Error != nil {
		return fmt.Errorf("category limit repository: upsert: %w", result.Error)
	}
	return nil
}

// Delete remove um limite pelo category_id, garantindo que pertence à conta.
func (r *CategoryLimitRepository) Delete(accountID, categoryID uuid.UUID) error {
	result := r.db.
		Where("account_id = ? AND category_id = ?", accountID, categoryID).
		Delete(&models.CategoryLimit{})
	if result.Error != nil {
		return fmt.Errorf("category limit repository: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Passo 2: Compilar para verificar**

```bash
cd backend && go build ./...
```

Saída esperada: sem erros.

- [ ] **Passo 3: Commit**

```bash
git add backend/internal/repository/category_limit.go
git commit -m "feat(backend): add CategoryLimitRepository"
```

---

## Tarefa 3: Handler (Backend)

**Arquivos:**
- Criar: `backend/internal/handlers/category_limit.go`

- [ ] **Passo 1: Criar o handler**

Criar `backend/internal/handlers/category_limit.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type CategoryLimitHandler struct {
	limitRepo *repository.CategoryLimitRepository
	txRepo    *repository.TransactionRepository
	catRepo   *repository.CategoryRepository
}

func NewCategoryLimitHandler(
	limitRepo *repository.CategoryLimitRepository,
	txRepo *repository.TransactionRepository,
	catRepo *repository.CategoryRepository,
) *CategoryLimitHandler {
	return &CategoryLimitHandler{limitRepo: limitRepo, txRepo: txRepo, catRepo: catRepo}
}

// GET /account/limits
// Retorna todos os limites da conta com gasto atual do mês.
func (h *CategoryLimitHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	limits, err := h.limitRepo.ListByAccount(accountID)
	if err != nil {
		log.Error().Err(err).Msg("category_limit: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	now := time.Now()
	summary, err := h.txRepo.SummaryByMonth(accountID, now.Year(), int(now.Month()))
	if err != nil {
		log.Error().Err(err).Msg("category_limit: summary")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Busca slugs das categorias para montar o DTO
	cats, err := h.catRepo.FindByIDs(categoryIDs(limits))
	if err != nil {
		log.Error().Err(err).Msg("category_limit: find categories")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	slugByID := make(map[uuid.UUID]string, len(cats))
	for _, c := range cats {
		slugByID[c.ID] = c.Slug
	}

	result := make([]models.LimitWithProgress, 0, len(limits))
	for _, l := range limits {
		slug := slugByID[l.CategoryID]
		spent := math.Abs(summary.Categories[slug])
		pct := 0.0
		if l.MonthlyLimit > 0 {
			pct = spent / l.MonthlyLimit * 100
		}
		result = append(result, models.LimitWithProgress{
			ID:           l.ID,
			CategoryID:   l.CategoryID,
			CategorySlug: slug,
			MonthlyLimit: l.MonthlyLimit,
			Spent:        spent,
			Pct:          pct,
		})
	}

	respondJSON(w, http.StatusOK, result)
}

// PUT /account/limits/:category_id
// Cria ou atualiza um limite (upsert).
func (h *CategoryLimitHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	categoryID, err := uuid.Parse(chi.URLParam(r, "category_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "category_id inválido")
		return
	}

	var body struct {
		MonthlyLimit float64 `json:"monthly_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.MonthlyLimit <= 0 {
		respondError(w, http.StatusBadRequest, "monthly_limit deve ser maior que zero")
		return
	}

	limit := &models.CategoryLimit{
		AccountID:    accountID,
		CategoryID:   categoryID,
		MonthlyLimit: body.MonthlyLimit,
	}
	if err := h.limitRepo.Upsert(limit); err != nil {
		log.Error().Err(err).Msg("category_limit: upsert")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, limit)
}

// DELETE /account/limits/:category_id
// Remove um limite.
func (h *CategoryLimitHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	categoryID, err := uuid.Parse(chi.URLParam(r, "category_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "category_id inválido")
		return
	}

	if err := h.limitRepo.Delete(accountID, categoryID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "limite não encontrado")
			return
		}
		log.Error().Err(err).Msg("category_limit: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /account/limits/check?category_id=X&amount=Y
// Verifica se um valor ultrapassaria o limite mensal de uma categoria.
func (h *CategoryLimitHandler) Check(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	categoryID, err := uuid.Parse(r.URL.Query().Get("category_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "category_id inválido")
		return
	}

	var amount float64
	if _, err := fmt.Sscanf(r.URL.Query().Get("amount"), "%f", &amount); err != nil || amount <= 0 {
		respondError(w, http.StatusBadRequest, "amount inválido")
		return
	}

	limits, err := h.limitRepo.ListByAccount(accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var matchedLimit *models.CategoryLimit
	for _, l := range limits {
		if l.CategoryID == categoryID {
			cl := l
			matchedLimit = &cl
			break
		}
	}

	if matchedLimit == nil {
		// Sem limite configurado — nunca excede
		respondJSON(w, http.StatusOK, map[string]any{"exceeds": false, "current": 0, "limit": 0})
		return
	}

	// Busca slug da categoria para cruzar com o summary
	cats, err := h.catRepo.FindByIDs([]uuid.UUID{categoryID})
	if err != nil || len(cats) == 0 {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	slug := cats[0].Slug

	now := time.Now()
	summary, err := h.txRepo.SummaryByMonth(accountID, now.Year(), int(now.Month()))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	current := math.Abs(summary.Categories[slug])
	exceeds := (current + amount) > matchedLimit.MonthlyLimit

	respondJSON(w, http.StatusOK, map[string]any{
		"exceeds": exceeds,
		"current": current,
		"limit":   matchedLimit.MonthlyLimit,
	})
}

// categoryIDs extrai os UUIDs de categoria de uma lista de limites.
func categoryIDs(limits []models.CategoryLimit) []uuid.UUID {
	ids := make([]uuid.UUID, len(limits))
	for i, l := range limits {
		ids[i] = l.CategoryID
	}
	return ids
}
```

- [ ] **Passo 2: Verificar se `CategoryRepository` tem `FindByIDs`**

```bash
grep -n "FindByIDs\|FindByID" backend/internal/repository/category.go
```

Se `FindByIDs` não existir, adicionar em `category.go`:

```go
func (r *CategoryRepository) FindByIDs(ids []uuid.UUID) ([]models.Category, error) {
	var cats []models.Category
	if len(ids) == 0 {
		return cats, nil
	}
	if err := r.db.Where("id IN ?", ids).Find(&cats).Error; err != nil {
		return nil, fmt.Errorf("category repository: find by ids: %w", err)
	}
	return cats, nil
}
```

- [ ] **Passo 3: Adicionar `fmt` ao import do handler**

O `Check` usa `fmt.Sscanf`. Adicionar `"fmt"` nos imports de `category_limit.go`.

- [ ] **Passo 4: Compilar**

```bash
cd backend && go build ./...
```

Saída esperada: sem erros.

- [ ] **Passo 5: Commit**

```bash
git add backend/internal/handlers/category_limit.go backend/internal/repository/category.go
git commit -m "feat(backend): add CategoryLimitHandler with list/upsert/delete/check"
```

---

## Tarefa 4: Rotas (Backend)

**Arquivos:**
- Modificar: `backend/cmd/server/main.go`

- [ ] **Passo 1: Instanciar repo e handler**

Em `main.go`, após a linha que instancia `goalRepo` (~linha 91), adicionar:

```go
limitRepo := repository.NewCategoryLimitRepository(db)
```

Após a linha que instancia `goalHandler` (~linha 196), adicionar:

```go
limitHandler := handlers.NewCategoryLimitHandler(limitRepo, txRepo, catRepo)
```

- [ ] **Passo 2: Registrar as rotas**

No bloco `r.Group` protegido por JWT (~linha 250), após o bloco `/goals`, adicionar:

```go
r.Route("/account/limits", func(r chi.Router) {
    r.Get("/", limitHandler.List)
    r.Get("/check", limitHandler.Check)
    r.Put("/{category_id}", limitHandler.Upsert)
    r.Delete("/{category_id}", limitHandler.Delete)
})
```

- [ ] **Passo 3: Compilar e testar manualmente**

```bash
cd backend && go build ./... && go run ./cmd/server
```

Testar os endpoints com curl (substituir `TOKEN` por um JWT válido):

```bash
# Listar limites (deve retornar [])
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/account/limits

# Criar limite (substituir UUID_DA_CATEGORIA por um UUID real da tabela categories)
curl -X PUT -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"monthly_limit": 300}' \
  http://localhost:8080/account/limits/UUID_DA_CATEGORIA

# Listar novamente (deve retornar o limite criado com spent e pct)
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/account/limits

# Verificar excedência
curl -H "Authorization: Bearer TOKEN" \
  "http://localhost:8080/account/limits/check?category_id=UUID_DA_CATEGORIA&amount=50"

# Deletar
curl -X DELETE -H "Authorization: Bearer TOKEN" \
  http://localhost:8080/account/limits/UUID_DA_CATEGORIA
```

- [ ] **Passo 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(backend): register /account/limits routes"
```

---

## Tarefa 5: Categorias com emojis e lista de despesas (Frontend)

**Arquivos:**
- Modificar: `frontend/src/lib/categories.ts`

- [ ] **Passo 1: Adicionar `CATEGORY_EMOJIS` e `EXPENSE_CATEGORIES`**

Em `frontend/src/lib/categories.ts`, adicionar após o `categoryLabel` existente:

```ts
export const CATEGORY_EMOJIS: Record<string, string> = {
  alimentacao: "🛒",
  transporte: "🚗",
  saude: "🏥",
  casa: "🏠",
  lazer: "🎮",
  educacao: "📚",
  vestuario: "👗",
  presentes: "🎁",
  servicos: "🔧",
  pets: "🐾",
  emprestimos: "💳",
  investimentos_despesa: "📈",
  outros: "📦",
  comer_fora: "🍽️",
  comida: "🛒",
  frutaria: "🍎",
  bebidas_nao_alcoolicas: "🥤",
  bebidas_alcoolicas: "🍺",
  higiene_pessoal: "🧴",
  limpeza_da_casa: "🧹",
}

export function categoryEmoji(slug: string): string {
  return CATEGORY_EMOJIS[slug] ?? "📦"
}

// Slugs de despesa para o modo standard
export const EXPENSE_SLUGS_STANDARD = [
  "alimentacao", "transporte", "saude", "casa", "lazer", "educacao",
  "vestuario", "presentes", "servicos", "pets", "emprestimos",
  "investimentos_despesa", "outros",
]

// Slugs de despesa para o modo food_detail
export const EXPENSE_SLUGS_FOOD_DETAIL = [
  "comer_fora", "comida", "frutaria", "bebidas_nao_alcoolicas",
  "bebidas_alcoolicas", "higiene_pessoal", "limpeza_da_casa",
  "transporte", "saude", "casa", "lazer", "educacao",
  "vestuario", "presentes", "servicos", "pets", "emprestimos",
  "investimentos_despesa", "outros",
]
```

- [ ] **Passo 2: Verificar que compila**

```bash
cd frontend && npx tsc --noEmit
```

Saída esperada: sem erros relacionados a `categories.ts`.

- [ ] **Passo 3: Commit**

```bash
git add frontend/src/lib/categories.ts
git commit -m "feat(frontend): add CATEGORY_EMOJIS and EXPENSE_SLUGS to categories lib"
```

---

## Tarefa 6: Sincronizar `showLimitsCategories` no Zustand store (Frontend)

**Arquivos:**
- Modificar: `frontend/src/store/settings.ts`
- Modificar: `frontend/src/hooks/useSettings.ts`

- [ ] **Passo 1: Adicionar `showLimitsCategories` ao store**

Em `frontend/src/store/settings.ts`, substituir o conteúdo por:

```ts
import { create } from "zustand";

interface SettingsState {
  showGoals: boolean;
  setShowGoals: (v: boolean) => void;
  showTools: boolean;
  setShowTools: (v: boolean) => void;
  showLimitsCategories: boolean;
  setShowLimitsCategories: (v: boolean) => void;
}

export const useSettingsStore = create<SettingsState>()((set) => ({
  showGoals: true,
  setShowGoals: (v) => set({ showGoals: v }),
  showTools: false,
  setShowTools: (v) => set({ showTools: v }),
  showLimitsCategories: false,
  setShowLimitsCategories: (v) => set({ showLimitsCategories: v }),
}));
```

- [ ] **Passo 2: Sincronizar o store no `useSettings`**

Em `frontend/src/hooks/useSettings.ts`, adicionar a leitura e set do store.

Adicionar após a linha `const setShowToolsStore = useSettingsStore((s) => s.setShowTools);`:

```ts
const setShowLimitsCategoriesStore = useSettingsStore((s) => s.setShowLimitsCategories);
```

Nos dois lugares onde `setShowGoalsStore` e `setShowToolsStore` são chamados (linhas ~44-45 e ~89-90), adicionar:

```ts
setShowLimitsCategoriesStore(next.showLimitsCategories);
```

Atualizar o array de dependências do `useEffect` para incluir `setShowLimitsCategoriesStore`.

- [ ] **Passo 3: Compilar**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Passo 4: Commit**

```bash
git add frontend/src/store/settings.ts frontend/src/hooks/useSettings.ts
git commit -m "feat(frontend): sync showLimitsCategories to Zustand store"
```

---

## Tarefa 7: Hook `useLimits` (Frontend)

**Arquivos:**
- Criar: `frontend/src/hooks/useLimits.ts`

- [ ] **Passo 1: Criar o hook**

Criar `frontend/src/hooks/useLimits.ts`:

```ts
import { useCallback, useEffect, useState } from "react"
import { api } from "@/services/api"

export interface LimitWithProgress {
  id: string
  category_id: string
  category_slug: string
  monthly_limit: number
  spent: number
  pct: number
}

export function useLimits() {
  const [limits, setLimits] = useState<LimitWithProgress[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const fetchLimits = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await api.get<LimitWithProgress[]>("/account/limits")
      setLimits(data ?? [])
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchLimits()
  }, [fetchLimits])

  // Recarrega ao recuperar foco da janela (mesmo padrão do useReservas)
  useEffect(() => {
    window.addEventListener("focus", fetchLimits)
    return () => window.removeEventListener("focus", fetchLimits)
  }, [fetchLimits])

  const upsert = useCallback(
    async (categoryId: string, monthlyLimit: number) => {
      await api.put(`/account/limits/${categoryId}`, { monthly_limit: monthlyLimit })
      await fetchLimits()
    },
    [fetchLimits]
  )

  const remove = useCallback(
    async (categoryId: string) => {
      await api.delete(`/account/limits/${categoryId}`)
      await fetchLimits()
    },
    [fetchLimits]
  )

  const checkExceeds = useCallback(
    async (categoryId: string, amount: number): Promise<{ exceeds: boolean; current: number; limit: number }> => {
      return api.get(`/account/limits/check?category_id=${categoryId}&amount=${amount}`)
    },
    []
  )

  return { limits, isLoading, upsert, remove, checkExceeds, refetch: fetchLimits }
}
```

- [ ] **Passo 2: Compilar**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Passo 3: Commit**

```bash
git add frontend/src/hooks/useLimits.ts
git commit -m "feat(frontend): add useLimits hook"
```

---

## Tarefa 8: Página `Limites.tsx` (Frontend)

**Arquivos:**
- Criar: `frontend/src/pages/Limites.tsx`

- [ ] **Passo 1: Criar o hook de formatação de moeda**

Verificar que `formatCurrency` existe em `frontend/src/store/currency.ts`:

```bash
grep -n "formatCurrency\|useCurrencyStore" frontend/src/store/currency.ts | head -5
```

- [ ] **Passo 2: Buscar os category_ids do backend**

A página precisa mapear slugs para UUIDs ao criar um limite. Verificar se o endpoint `GET /categories` retorna `id` e `slug`:

```bash
grep -n "ListCategories\|Category\|slug\|ID" backend/internal/handlers/transaction.go | head -20
```

Se retornar, o frontend pode usar `api.get<Category[]>("/categories")` para obter o mapeamento slug→id.

- [ ] **Passo 3: Criar a página**

Criar `frontend/src/pages/Limites.tsx`:

```tsx
import { useState } from "react"
import { useLimits, LimitWithProgress } from "@/hooks/useLimits"
import { useCurrencyStore, formatCurrency } from "@/store/currency"
import { useSettings } from "@/hooks/useSettings"
import { api } from "@/services/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { useMediaQuery } from "@/hooks/useMediaQuery"
import { cn } from "@/lib/utils"
import {
  categoryLabel,
  categoryEmoji,
  EXPENSE_SLUGS_STANDARD,
  EXPENSE_SLUGS_FOOD_DETAIL,
} from "@/lib/categories"

// ── Tipos ──────────────────────────────────────────────────────────────────

interface Category {
  id: string
  slug: string
  label: string
  type: string
  mode: string
}

// ── CircularProgress ───────────────────────────────────────────────────────

function LimitCircle({ pct, size }: { pct: number; size: number }) {
  const clamped = Math.min(pct, 100)
  const strokeWidth = size * 0.085
  const r = size / 2 - strokeWidth / 2
  const circumference = 2 * Math.PI * r
  const offset = circumference * (1 - clamped / 100)

  const stroke =
    pct > 100
      ? "hsl(var(--destructive))"
      : pct >= 70
      ? "hsl(var(--warning))"
      : "hsl(var(--primary))"

  return (
    <div className="relative inline-flex items-center justify-center shrink-0" style={{ width: size, height: size }}>
      <svg width={size} height={size} style={{ transform: "rotate(-90deg)" }}>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="hsl(var(--border))" strokeWidth={strokeWidth} />
        <circle
          cx={size / 2} cy={size / 2} r={r} fill="none"
          stroke={stroke} strokeWidth={strokeWidth} strokeLinecap="round"
          strokeDasharray={circumference} strokeDashoffset={offset}
          style={{ transition: "stroke-dashoffset 0.6s ease" }}
        />
      </svg>
      <span className="absolute text-[10px] font-bold" style={{ color: stroke }}>
        {Math.round(pct)}%
      </span>
    </div>
  )
}

// ── LimitCard ──────────────────────────────────────────────────────────────

function LimitCard({
  item,
  fmt,
  onEdit,
  onDelete,
}: {
  item: LimitWithProgress
  fmt: (v: number) => string
  onEdit: (item: LimitWithProgress) => void
  onDelete: (item: LimitWithProgress) => void
}) {
  const isOver = item.pct > 100
  const isWarn = item.pct >= 70 && item.pct <= 100

  return (
    <div
      className={cn(
        "flex items-center gap-3 p-3.5 rounded-xl border",
        isOver
          ? "border-destructive/40 bg-destructive/5"
          : "border-border bg-card"
      )}
      onClick={() => onEdit(item)}
    >
      <LimitCircle pct={item.pct} size={52} />

      <div className="flex-1 min-w-0">
        <p className="font-semibold text-sm truncate">
          {categoryEmoji(item.category_slug)} {categoryLabel(item.category_slug)}
        </p>
        <p className={cn(
          "text-xs mt-0.5",
          isOver ? "text-destructive" : isWarn ? "text-warning" : "text-muted-foreground"
        )}>
          {fmt(item.spent)} de {fmt(item.monthly_limit)}
          {isOver && " ⚠️"}
        </p>
      </div>

      <button
        className="text-muted-foreground hover:text-destructive text-xs px-2 py-1 shrink-0"
        onClick={(e) => { e.stopPropagation(); onDelete(item) }}
      >
        ✕
      </button>
    </div>
  )
}

// ── LimitForm (conteúdo compartilhado entre Sheet e Dialog) ───────────────

function LimitForm({
  categories,
  existingSlugs,
  editing,
  onSave,
  onCancel,
}: {
  categories: Category[]
  existingSlugs: string[]
  editing: LimitWithProgress | null
  onSave: (categoryId: string, monthlyLimit: number) => void
  onCancel: () => void
}) {
  const [selectedId, setSelectedId] = useState(editing?.category_id ?? "")
  const [amount, setAmount] = useState(editing ? String(editing.monthly_limit) : "")

  const available = categories.filter(
    (c) => !existingSlugs.includes(c.slug) || c.id === editing?.category_id
  )

  return (
    <div className="flex flex-col gap-4 pt-2">
      {!editing && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">Categoria</p>
          <div className="flex flex-col gap-1.5 max-h-48 overflow-y-auto">
            {available.map((c) => (
              <button
                key={c.id}
                onClick={() => setSelectedId(c.id)}
                className={cn(
                  "flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium text-left border transition-colors",
                  selectedId === c.id
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-transparent bg-muted/40 hover:bg-muted/60"
                )}
              >
                <span>{categoryEmoji(c.slug)}</span>
                <span>{categoryLabel(c.slug)}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      <div>
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">Limite mensal (R$)</p>
        <Input
          type="number"
          inputMode="decimal"
          min="0"
          step="0.01"
          placeholder="0,00"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
        />
      </div>

      <div className="flex gap-2 pt-1">
        <Button variant="outline" className="flex-1" onClick={onCancel}>Cancelar</Button>
        <Button
          className="flex-1"
          disabled={(!editing && !selectedId) || !amount || Number(amount) <= 0}
          onClick={() => onSave(editing ? editing.category_id : selectedId, Number(amount))}
        >
          Salvar limite
        </Button>
      </div>
    </div>
  )
}

// ── Página principal ───────────────────────────────────────────────────────

export default function Limites() {
  const { limits, isLoading, upsert, remove } = useLimits()
  const { currency } = useCurrencyStore()
  const fmt = (v: number) => formatCurrency(v, currency)
  const { categoryContext } = useSettings()
  const isDesktop = useMediaQuery("(min-width: 1024px)")

  const [allCategories, setAllCategories] = useState<Category[]>([])
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<LimitWithProgress | null>(null)

  // Carrega categorias do backend ao abrir o form
  const openAdd = async () => {
    if (allCategories.length === 0) {
      const cats = await api.get<Category[]>("/categories")
      setAllCategories(cats ?? [])
    }
    setEditing(null)
    setSheetOpen(true)
  }

  const openEdit = async (item: LimitWithProgress) => {
    if (allCategories.length === 0) {
      const cats = await api.get<Category[]>("/categories")
      setAllCategories(cats ?? [])
    }
    setEditing(item)
    setSheetOpen(true)
  }

  const handleSave = async (categoryId: string, monthlyLimit: number) => {
    await upsert(categoryId, monthlyLimit)
    setSheetOpen(false)
    setEditing(null)
  }

  const handleDelete = async (item: LimitWithProgress) => {
    await remove(item.category_id)
  }

  // Filtra categorias de despesa do contexto atual
  const expenseSlugs = categoryContext === "food_detail" ? EXPENSE_SLUGS_FOOD_DETAIL : EXPENSE_SLUGS_STANDARD
  const expenseCategories = allCategories.filter(
    (c) => expenseSlugs.includes(c.slug)
  )
  const existingSlugs = limits.map((l) => l.category_slug)

  // Ordena: acima do limite primeiro, depois por pct decrescente
  const sorted = [...limits].sort((a, b) => {
    if (a.pct > 100 && b.pct <= 100) return -1
    if (b.pct > 100 && a.pct <= 100) return 1
    return b.pct - a.pct
  })

  const now = new Date()
  const monthLabel = now.toLocaleDateString("pt-BR", { month: "long", year: "numeric" })

  const formContent = (
    <LimitForm
      categories={expenseCategories}
      existingSlugs={existingSlugs}
      editing={editing}
      onSave={handleSave}
      onCancel={() => { setSheetOpen(false); setEditing(null) }}
    />
  )

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto px-4 pt-4 pb-4 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-lg font-bold font-[Sora]">Limites</h1>
            <p className="text-xs text-muted-foreground capitalize">{monthLabel}</p>
          </div>
          <Button size="sm" onClick={openAdd}>+ Adicionar</Button>
        </div>

        {isLoading ? (
          <p className="text-sm text-muted-foreground text-center pt-8">Carregando...</p>
        ) : limits.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center pt-8">
            Nenhum limite configurado. Adicione um limite para acompanhar seus gastos.
          </p>
        ) : (
          sorted.map((item) => (
            <LimitCard key={item.id} item={item} fmt={fmt} onEdit={openEdit} onDelete={handleDelete} />
          ))
        )}
      </div>

      {isDesktop ? (
        <Dialog open={sheetOpen} onOpenChange={setSheetOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{editing ? "Editar limite" : "Novo limite"}</DialogTitle>
            </DialogHeader>
            {formContent}
          </DialogContent>
        </Dialog>
      ) : (
        <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
          <SheetContent side="bottom" className="rounded-t-2xl pb-8">
            <SheetHeader>
              <SheetTitle>{editing ? "Editar limite" : "Novo limite"}</SheetTitle>
            </SheetHeader>
            {formContent}
          </SheetContent>
        </Sheet>
      )}
    </div>
  )
}
```

- [ ] **Passo 4: Compilar**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Passo 5: Commit**

```bash
git add frontend/src/pages/Limites.tsx
git commit -m "feat(frontend): add Limites page with LimitCard and LimitSheet"
```

---

## Tarefa 9: Rota e BottomNav (Frontend)

**Arquivos:**
- Modificar: `frontend/src/App.tsx`
- Modificar: `frontend/src/components/BottomNav/BottomNav.tsx`

- [ ] **Passo 1: Adicionar import da página em `App.tsx`**

Em `frontend/src/App.tsx`, adicionar o import lazy ou direto junto aos demais imports de páginas:

```tsx
import Limites from "@/pages/Limites"
```

- [ ] **Passo 2: Adicionar a rota em `App.tsx`**

Após a linha da rota `/reservas`:

```tsx
<Route path="/reservas" element={<ProtectedRoute><AppShell><Reservas /></AppShell></ProtectedRoute>} />
```

Adicionar:

```tsx
<Route path="/limites" element={<ProtectedRoute><AppShell><Limites /></AppShell></ProtectedRoute>} />
```

- [ ] **Passo 3: Adicionar item no `BottomNav`**

Em `frontend/src/components/BottomNav/BottomNav.tsx`:

Adicionar `Target` ao import do lucide-react:

```tsx
import { MessageSquare, BarChart3, BarChart2, Wrench, PiggyBank, Target } from "lucide-react";
```

Adicionar leitura do store:

```tsx
const showLimitsCategories = useSettingsStore((s) => s.showLimitsCategories);
```

Adicionar item no array `navItems`, entre Dashboard e Reservas:

```tsx
const navItems = [
  { path: "/chat", icon: MessageSquare, label: "Chat" },
  { path: "/dashboard", icon: BarChart3, label: "Dashboard" },
  ...(showLimitsCategories ? [{ path: "/limites", icon: Target, label: "Limites" } as const] : []),
  ...(showGoals ? [{ path: "/reservas", icon: PiggyBank, label: "Reservas" } as const] : []),
  ...(isDesktop ? [{ path: "/full-dashboard", icon: BarChart2, label: "Analytics" } as const] : []),
  ...(showTools ? [{ path: "/tools", icon: Wrench, label: "Tools" } as const] : []),
];
```

- [ ] **Passo 4: Compilar**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Passo 5: Testar no browser**

```bash
cd frontend && npm run dev
```

Abrir http://localhost:5173, ativar `show_limits_categories` no perfil (ou direto no banco: `UPDATE user_settings SET show_limits_categories = true WHERE user_id = 'SEU_UUID'`), e verificar:
- Aba "Limites" aparece no bottom nav
- Página abre sem erros
- Botão "Adicionar" abre o sheet com categorias
- Criar um limite, confirmar que aparece na lista com progresso circular

- [ ] **Passo 6: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/BottomNav/BottomNav.tsx
git commit -m "feat(frontend): add /limites route and BottomNav entry"
```

---

## Tarefa 10: Confirmação ao ultrapassar limite no fluxo de transação

**Arquivos:**
- Localizar onde transações são criadas no frontend e adicionar a verificação.

- [ ] **Passo 1: Localizar o componente de criação de transação**

```bash
grep -rn "api.post.*transactions\|createTransaction\|/transactions" frontend/src --include="*.tsx" --include="*.ts" | grep -v node_modules
```

- [ ] **Passo 2: Identificar onde o `category_id` é conhecido antes do submit**

O fluxo de criação de transação precisa saber o `category_id` para chamar o check. Localizar onde o campo de categoria é preenchido no formulário.

- [ ] **Passo 3: Adicionar verificação antes do submit**

No handler de submit do formulário de transação, antes de chamar `api.post("/transactions", ...)`, adicionar:

```tsx
// Se a transação é uma despesa (amount < 0) e há category_id
if (amount < 0 && categoryId) {
  const check = await api.get<{ exceeds: boolean; current: number; limit: number }>(
    `/account/limits/check?category_id=${categoryId}&amount=${Math.abs(amount)}`
  )
  if (check.exceeds) {
    const confirmed = window.confirm(
      `Você vai ultrapassar o limite de ${formatCurrency(check.limit, currency)} nessa categoria. Confirmar mesmo assim?`
    )
    if (!confirmed) return
  }
}
```

> **Nota:** Se o projeto usa dialogs customizados ao invés de `window.confirm`, adaptar para usar o padrão existente (Dialog/AlertDialog do shadcn/ui). Verificar o padrão no codebase antes de implementar.

- [ ] **Passo 4: Testar o fluxo completo**

1. Criar um limite de R$300 em "Bebidas alcoólicas"
2. Adicionar transações até chegar perto do limite
3. Tentar lançar uma transação que ultrapasse — confirmar que o modal aparece
4. Cancelar — confirmar que a transação não é criada
5. Confirmar — confirmar que a transação é criada normalmente

- [ ] **Passo 5: Compilar e commit**

```bash
cd frontend && npx tsc --noEmit
git add -p  # adicionar apenas os arquivos modificados
git commit -m "feat(frontend): warn before exceeding category limit on transaction submit"
```

---

## Tarefa 11: Verificação multi-usuário e limpeza

- [ ] **Passo 1: Testar com dois usuários da mesma conta**

1. Fazer login com usuário A, criar limite de R$300 em "Lazer"
2. Fazer logout, fazer login com usuário B (mesma conta)
3. Verificar que o limite aparece na aba Limites
4. Adicionar transação em "Lazer" com usuário B, verificar que o `spent` reflete na aba de usuário A também

- [ ] **Passo 2: Verificar que limite por conta isolado de outras contas**

Criar um limite com conta X, verificar que conta Y não vê esse limite.

- [ ] **Passo 3: Commit final se houver ajustes**

```bash
git add -p
git commit -m "fix: adjust category limits after multi-user testing"
```

---

## Checklist de Verificação Final

- [ ] Migration roda sem erros em banco limpo
- [ ] `GET /account/limits` retorna `[]` para conta sem limites
- [ ] Upsert cria e atualiza sem duplicar linha (unique constraint funcionando)
- [ ] `pct` e `spent` corretos após adicionar transações
- [ ] `GET /account/limits/check` retorna `exceeds: false` abaixo do limite e `true` acima
- [ ] Aba "Limites" aparece no bottom nav quando `show_limits_categories = true`
- [ ] Cards renderizam com cor verde / âmbar / vermelho nos estados corretos
- [ ] Sheet de criação abre, lista categorias sem limite já configurado
- [ ] Editar limite atualiza o valor (não cria duplicata)
- [ ] Deletar limite remove o card da lista
- [ ] Modal de confirmação aparece ao ultrapassar limite em nova transação
- [ ] Limites são compartilhados entre usuários da mesma conta
