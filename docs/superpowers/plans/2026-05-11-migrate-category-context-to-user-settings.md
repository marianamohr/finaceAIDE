# Migrate category_context to user_settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move `category_context` from `users` to `user_settings`, add three new columns (`show_goals bool`, `user_api_key text`, `show_limits_categories bool`), and update every call site so there is no contract break.

**Architecture:** The `UserSettings` model gains the four fields. `UserRepository.FindByID` (and the equivalent used by upload/chat handlers) gains a JOIN so that `user.CategoryContext` still resolves seamlessly via a composite struct. The `UpdateSettings` handler is expanded to also accept the new fields. The frontend `getMe` response type and `useSettings` hook are updated to carry the new fields; the `api.updateSettings` call is widened accordingly.

**Tech Stack:** Go / GORM (PostgreSQL), React / TypeScript, Zerolog

---

## File Map

| File | Change |
|---|---|
| `backend/internal/models/user_settings.go` | Add `CategoryContext`, `ShowGoals`, `UserAPIKey`, `ShowLimitsCategories` |
| `backend/internal/models/user.go` | Remove `CategoryContext` field |
| `backend/internal/repository/user.go` | Update `FindByID` to JOIN `user_settings`; update `UpdateSettings` signature |
| `backend/internal/handlers/user.go` | Expand `Me` response; expand `UpdateSettings` request/response |
| `backend/internal/handlers/upload.go` | No change needed — reads `user.CategoryContext`; resolved by JOIN in repo |
| `backend/internal/handlers/chat.go` | No change needed — reads `user.CategoryContext`; resolved by JOIN in repo |
| `backend/migrations/migrate.go` | Add `addNullableColumns` entries; add backfill; run `AutoMigrate` for new shape |
| `frontend/src/services/api.ts` | Widen `getMe` return type; widen `updateSettings` argument |
| `frontend/src/hooks/useSettings.ts` | Add `showGoals`, `userApiKey`, `showLimitsCategories` to state and return |

---

## Task 1: Update the UserSettings model

**Files:**
- Modify: `backend/internal/models/user_settings.go`

- [ ] **Step 1: Add the four new fields**

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// UserSettings stores per-user preferences.
// DistributionMethod values: "70-30" | "60-20-20"
// CategoryContext values: "standard" | "food_detail"
type UserSettings struct {
	UserID                 uuid.UUID `gorm:"primarykey;type:uuid"           json:"user_id"`
	BaseCurrency           string    `gorm:"not null;default:'BRL'"         json:"base_currency"`
	DistributionMethod     string    `gorm:"not null;default:'70-30'"       json:"distribution_method"`
	FuturePct              float64   `gorm:"type:numeric(6,4);default:0.30" json:"future_pct"`
	CategoryContext        string    `gorm:"not null;default:'standard'"    json:"category_context"`
	ShowGoals              bool      `gorm:"not null;default:true"          json:"show_goals"`
	UserAPIKey             string    `gorm:"not null;default:''"            json:"user_api_key"`
	ShowLimitsCategories   bool      `gorm:"not null;default:false"         json:"show_limits_categories"`
	UpdatedAt              time.Time `gorm:"not null;autoUpdateTime"        json:"updated_at"`
}
```

- [ ] **Step 2: Remove `CategoryContext` from the User model**

In `backend/internal/models/user.go`, delete the `CategoryContext` line:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null"                           json:"email"`
	PasswordHash string    `gorm:"not null"                                       json:"-"`
	Role         string    `gorm:"not null;default:'user'"                        json:"role"`
	Status       string    `gorm:"not null;default:'pending'"                     json:"status"`     // "pending"|"active"|"suspended"
	AccountID    uuid.UUID `gorm:"type:uuid"                                      json:"account_id"` // FK to accounts
	CreatedAt    time.Time `                                                       json:"created_at"`
	UpdatedAt    time.Time `                                                       json:"updated_at"`
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/user_settings.go backend/internal/models/user.go
git commit -m "feat(models): move category_context to user_settings, add show_goals/user_api_key/show_limits_categories"
```

---

## Task 2: Add a composite UserWithSettings struct for JOIN queries

**Files:**
- Modify: `backend/internal/repository/user.go`

The handlers that use `user.CategoryContext` (upload, chat) call `FindByID`. We need `FindByID` to return a type that still exposes `.CategoryContext` so the call sites need zero changes.

- [ ] **Step 1: Add UserWithSettings struct and update FindByID**

Replace the existing `FindByID` and `UpdateSettings` in `backend/internal/repository/user.go`:

```go
package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// UserWithSettings is a flat projection of users JOIN user_settings.
// Call sites that need category_context (upload, chat) use this type.
type UserWithSettings struct {
	models.User
	CategoryContext      string  `gorm:"column:category_context"`
	ShowGoals            bool    `gorm:"column:show_goals"`
	UserAPIKey           string  `gorm:"column:user_api_key"`
	ShowLimitsCategories bool    `gorm:"column:show_limits_categories"`
	BaseCurrency         string  `gorm:"column:base_currency"`
	DistributionMethod   string  `gorm:"column:distribution_method"`
	FuturePct            float64 `gorm:"column:future_pct"`
}

// Create inserts a new user. Returns an error if the email is already taken.
func (r *UserRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("user repository: create: %w", err)
	}
	return nil
}

// FindByEmail looks up a user by email.
// Returns ErrNotFound if no user exists with that email.
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user repository: find by email: %w", err)
	}
	return &user, nil
}

// FindByID looks up a user by primary key and LEFT JOINs user_settings so that
// settings fields (including category_context) are populated on the returned struct.
// Returns ErrNotFound if no user exists with that ID.
func (r *UserRepository) FindByID(id uuid.UUID) (*UserWithSettings, error) {
	var u UserWithSettings
	err := r.db.Table("users").
		Select(`users.*,
			COALESCE(us.category_context, 'standard')        AS category_context,
			COALESCE(us.show_goals, true)                     AS show_goals,
			COALESCE(us.user_api_key, '')                     AS user_api_key,
			COALESCE(us.show_limits_categories, false)        AS show_limits_categories,
			COALESCE(us.base_currency, 'BRL')                 AS base_currency,
			COALESCE(us.distribution_method, '70-30')         AS distribution_method,
			COALESCE(us.future_pct, 0.30)                     AS future_pct`).
		Joins("LEFT JOIN user_settings us ON us.user_id = users.id").
		Where("users.id = ?", id).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user repository: find by id: %w", err)
	}
	return &u, nil
}

// UpdateSettings updates mutable user preferences in user_settings.
// It upserts — creates a settings row if none exists yet.
func (r *UserRepository) UpdateSettings(id uuid.UUID, s models.UserSettings) error {
	s.UserID = id
	result := r.db.
		Where(models.UserSettings{UserID: id}).
		Assign(s).
		FirstOrCreate(&s)
	if result.Error != nil {
		return fmt.Errorf("user repository: update settings: %w", result.Error)
	}
	// If record already existed, do a full update of all assignable fields.
	if result.RowsAffected == 0 {
		result = r.db.Model(&models.UserSettings{}).
			Where("user_id = ?", id).
			Updates(map[string]any{
				"category_context":        s.CategoryContext,
				"show_goals":              s.ShowGoals,
				"user_api_key":            s.UserAPIKey,
				"show_limits_categories":  s.ShowLimitsCategories,
				"base_currency":           s.BaseCurrency,
				"distribution_method":     s.DistributionMethod,
				"future_pct":              s.FuturePct,
			})
		if result.Error != nil {
			return fmt.Errorf("user repository: update settings (update): %w", result.Error)
		}
	}
	return nil
}

// ListAll returns all users ordered by created_at asc (admin use).
func (r *UserRepository) ListAll() ([]models.User, error) {
	var users []models.User
	if err := r.db.Order("created_at asc").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("user repository: list all: %w", err)
	}
	return users, nil
}

// CountAll returns the total number of users in the database.
func (r *UserRepository) CountAll() (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("user repository: count: %w", err)
	}
	return count, nil
}

// UpdateStatus sets the user's status field.
func (r *UserRepository) UpdateStatus(id uuid.UUID, status string) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("user repository: update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateAccount moves a user to a different account.
func (r *UserRepository) UpdateAccount(id uuid.UUID, accountID uuid.UUID) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", id).
		Update("account_id", accountID)
	if result.Error != nil {
		return fmt.Errorf("user repository: update account: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("repository: record not found")

// IsUniqueViolation reports whether err is a PostgreSQL unique constraint violation (code 23505).
func IsUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/repository/user.go
git commit -m "feat(repository): FindByID joins user_settings; UpdateSettings takes full UserSettings"
```

---

## Task 3: Fix compilation errors in handlers caused by the changed types

**Files:**
- Modify: `backend/internal/handlers/user.go`
- Verify: `backend/internal/handlers/upload.go` (no change expected)
- Verify: `backend/internal/handlers/chat.go` (no change expected)

- [ ] **Step 1: Rewrite handlers/user.go**

`Me` now returns the joined `UserWithSettings`. `UpdateSettings` accepts all settings fields and passes a `models.UserSettings` struct to the repo.

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

// UserHandler handles user profile and settings endpoints.
type UserHandler struct {
	repo *repository.UserRepository
}

func NewUserHandler(repo *repository.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// Me godoc
// GET /user/me
// Returns the authenticated user's profile including settings (category_context, show_goals, etc.).
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	user, err := h.repo.FindByID(userID)
	if err != nil {
		log.Error().Err(err).Msg("user: me")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type meResponse struct {
		ID                   string  `json:"id"`
		Email                string  `json:"email"`
		Role                 string  `json:"role"`
		Status               string  `json:"status"`
		AccountID            string  `json:"account_id"`
		CategoryContext      string  `json:"category_context"`
		ShowGoals            bool    `json:"show_goals"`
		UserAPIKey           string  `json:"user_api_key"`
		ShowLimitsCategories bool    `json:"show_limits_categories"`
		BaseCurrency         string  `json:"base_currency"`
		DistributionMethod   string  `json:"distribution_method"`
		FuturePct            float64 `json:"future_pct"`
	}

	respondJSON(w, http.StatusOK, meResponse{
		ID:                   user.ID.String(),
		Email:                user.Email,
		Role:                 user.Role,
		Status:               user.Status,
		AccountID:            user.AccountID.String(),
		CategoryContext:      user.CategoryContext,
		ShowGoals:            user.ShowGoals,
		UserAPIKey:           user.UserAPIKey,
		ShowLimitsCategories: user.ShowLimitsCategories,
		BaseCurrency:         user.BaseCurrency,
		DistributionMethod:   user.DistributionMethod,
		FuturePct:            user.FuturePct,
	})
}

// UpdateSettings godoc
// PATCH /user/settings
// Body: any subset of { category_context, show_goals, user_api_key, show_limits_categories,
//                        base_currency, distribution_method, future_pct }
func (h *UserHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	// Load current settings first so we can do a partial update (PATCH semantics).
	current, err := h.repo.FindByID(userID)
	if err != nil {
		log.Error().Err(err).Msg("user: update settings fetch current")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req struct {
		CategoryContext      *string  `json:"category_context"`
		ShowGoals            *bool    `json:"show_goals"`
		UserAPIKey           *string  `json:"user_api_key"`
		ShowLimitsCategories *bool    `json:"show_limits_categories"`
		BaseCurrency         *string  `json:"base_currency"`
		DistributionMethod   *string  `json:"distribution_method"`
		FuturePct            *float64 `json:"future_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Merge patch onto current values.
	updated := models.UserSettings{
		CategoryContext:      current.CategoryContext,
		ShowGoals:            current.ShowGoals,
		UserAPIKey:           current.UserAPIKey,
		ShowLimitsCategories: current.ShowLimitsCategories,
		BaseCurrency:         current.BaseCurrency,
		DistributionMethod:   current.DistributionMethod,
		FuturePct:            current.FuturePct,
	}
	if req.CategoryContext != nil {
		if *req.CategoryContext != "standard" && *req.CategoryContext != "food_detail" {
			respondError(w, http.StatusBadRequest, "category_context must be 'standard' or 'food_detail'")
			return
		}
		updated.CategoryContext = *req.CategoryContext
	}
	if req.ShowGoals != nil {
		updated.ShowGoals = *req.ShowGoals
	}
	if req.UserAPIKey != nil {
		updated.UserAPIKey = *req.UserAPIKey
	}
	if req.ShowLimitsCategories != nil {
		updated.ShowLimitsCategories = *req.ShowLimitsCategories
	}
	if req.BaseCurrency != nil {
		updated.BaseCurrency = *req.BaseCurrency
	}
	if req.DistributionMethod != nil {
		if *req.DistributionMethod != "70-30" && *req.DistributionMethod != "60-20-20" {
			respondError(w, http.StatusBadRequest, "distribution_method must be '70-30' or '60-20-20'")
			return
		}
		updated.DistributionMethod = *req.DistributionMethod
	}
	if req.FuturePct != nil {
		updated.FuturePct = *req.FuturePct
	}

	if err := h.repo.UpdateSettings(userID, updated); err != nil {
		log.Error().Err(err).Msg("user: update settings")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, updated)
}
```

- [ ] **Step 2: Verify upload.go and chat.go compile without changes**

The handlers access `user.CategoryContext`. Since `FindByID` now returns `*UserWithSettings` which embeds `models.User` and has its own `CategoryContext` field, both files will resolve `.CategoryContext` from the flat struct — no changes needed. Confirm this by running:

```bash
cd backend && go build ./...
```

Expected: no errors. If there are type-mismatch errors (e.g., a handler still expects `*models.User`), fix the variable type annotation in the affected handler.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/user.go
git commit -m "feat(handlers): Me returns full settings; UpdateSettings accepts all settings fields"
```

---

## Task 4: Add the migration

**Files:**
- Modify: `backend/migrations/migrate.go`

- [ ] **Step 1: Add new columns to addNullableColumns and a backfill**

Add the following entries to the `stmts` slice inside `addNullableColumns`:

```go
`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS category_context text NOT NULL DEFAULT 'standard'`,
`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS show_goals bool NOT NULL DEFAULT true`,
`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS user_api_key text NOT NULL DEFAULT ''`,
`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS show_limits_categories bool NOT NULL DEFAULT false`,
```

- [ ] **Step 2: Add a backfill function to migrate existing category_context values**

Add a new function after `backfillCurrency`:

```go
// backfillCategoryContext copies category_context from users into user_settings
// for users whose settings row already exists, then drops the column from users.
// Safe to run multiple times (idempotent).
func backfillCategoryContext(db *gorm.DB) error {
	if err := db.Exec(`
		UPDATE user_settings us
		SET category_context = u.category_context
		FROM users u
		WHERE us.user_id = u.id
		  AND u.category_context IS NOT NULL
		  AND u.category_context != ''
	`).Error; err != nil {
		return fmt.Errorf("backfill category_context: %w", err)
	}
	// Drop column from users after successful backfill.
	if err := db.Exec(`ALTER TABLE users DROP COLUMN IF EXISTS category_context`).Error; err != nil {
		return fmt.Errorf("drop category_context from users: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Call backfillCategoryContext in Run after backfillCurrency**

```go
if err := backfillCategoryContext(db); err != nil {
    return fmt.Errorf("migrations: backfill category context: %w", err)
}
```

- [ ] **Step 4: Verify go build still passes**

```bash
cd backend && go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/migrate.go
git commit -m "feat(migrations): add category_context/show_goals/user_api_key/show_limits_categories to user_settings, backfill from users"
```

---

## Task 5: Update the frontend API contract

**Files:**
- Modify: `frontend/src/services/api.ts`

- [ ] **Step 1: Widen the getMe response type and updateSettings argument**

Find the `getMe` and `updateSettings` lines (around line 113–118 in `api.ts`) and replace them with:

```typescript
getMe: () =>
  request<{
    id: string;
    email: string;
    role: string;
    status: string;
    account_id: string;
    category_context: string;
    show_goals: boolean;
    user_api_key: string;
    show_limits_categories: boolean;
    base_currency: string;
    distribution_method: string;
    future_pct: number;
  }>("/user/me"),

updateSettings: (patch: {
  category_context?: string;
  show_goals?: boolean;
  user_api_key?: string;
  show_limits_categories?: boolean;
  base_currency?: string;
  distribution_method?: string;
  future_pct?: number;
}) =>
  request<{
    category_context: string;
    show_goals: boolean;
    user_api_key: string;
    show_limits_categories: boolean;
    base_currency: string;
    distribution_method: string;
    future_pct: number;
  }>("/user/settings", {
    method: "PATCH",
    body: JSON.stringify(patch),
  }),
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/services/api.ts
git commit -m "feat(api): widen getMe and updateSettings types to include new settings fields"
```

---

## Task 6: Update useSettings hook and fix all call sites

**Files:**
- Modify: `frontend/src/hooks/useSettings.ts`

- [ ] **Step 1: Expand the hook to expose all settings fields**

Replace the entire content of `frontend/src/hooks/useSettings.ts`:

```typescript
import { useState, useEffect } from "react";
import { api } from "@/services/api";
import { useAuthStore } from "@/store/auth";

export type CategoryContext = "standard" | "food_detail";

interface Settings {
  categoryContext: CategoryContext;
  showGoals: boolean;
  userApiKey: string;
  showLimitsCategories: boolean;
}

const defaults: Settings = {
  categoryContext: "standard",
  showGoals: true,
  userApiKey: "",
  showLimitsCategories: false,
};

export function useSettings() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const [settings, setSettings] = useState<Settings>(defaults);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isAuthenticated) return;
    api
      .getMe()
      .then((me) =>
        setSettings({
          categoryContext: me.category_context as CategoryContext,
          showGoals: me.show_goals,
          userApiKey: me.user_api_key,
          showLimitsCategories: me.show_limits_categories,
        })
      )
      .catch(() => {/* keep defaults */})
      .finally(() => setLoading(false));
  }, [isAuthenticated]);

  const updateContext = async (ctx: CategoryContext) => {
    const prev = settings.categoryContext;
    setSettings((s) => ({ ...s, categoryContext: ctx })); // optimistic
    try {
      await api.updateSettings({ category_context: ctx });
    } catch {
      setSettings((s) => ({ ...s, categoryContext: prev }));
    }
  };

  const updateSetting = async (patch: Partial<Omit<Settings, "categoryContext">>) => {
    const prev = settings;
    setSettings((s) => ({ ...s, ...patch })); // optimistic
    try {
      await api.updateSettings({
        show_goals: patch.showGoals,
        user_api_key: patch.userApiKey,
        show_limits_categories: patch.showLimitsCategories,
      });
    } catch {
      setSettings(prev);
    }
  };

  return {
    // Destructured for backward-compat with existing call sites
    categoryContext: settings.categoryContext,
    showGoals: settings.showGoals,
    userApiKey: settings.userApiKey,
    showLimitsCategories: settings.showLimitsCategories,
    loading,
    updateContext,
    updateSetting,
  };
}
```

- [ ] **Step 2: Verify existing call sites still compile**

`ProfileCategories.tsx` and `Profile.tsx` use `{ categoryContext, loading, updateContext }` — all still returned from the hook. Run the TypeScript compiler to confirm:

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useSettings.ts
git commit -m "feat(hooks): useSettings exposes show_goals, user_api_key, show_limits_categories"
```

---

## Task 7: End-to-end smoke test

- [ ] **Step 1: Start the backend and run a quick curl check**

```bash
# In one terminal:
cd backend && go run ./cmd/server

# In another terminal — replace TOKEN with a real JWT:
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/user/me | jq .
```

Expected response includes:
```json
{
  "category_context": "standard",
  "show_goals": true,
  "user_api_key": "",
  "show_limits_categories": false,
  ...
}
```

- [ ] **Step 2: Test PATCH /user/settings with new fields**

```bash
curl -s -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"show_goals": false, "show_limits_categories": true}' \
  http://localhost:8080/user/settings | jq .
```

Expected: response contains `show_goals: false` and `show_limits_categories: true`.

- [ ] **Step 3: Confirm upload and chat still work**

Upload a receipt image and confirm it processes without error. Send a chat message and confirm the RAG context builds correctly (logs should show `category_context` used from the JOIN).

- [ ] **Step 4: Confirm frontend compiles and hot-reloads without errors**

```bash
cd frontend && npm run dev
```

Open the ProfileCategories page — the toggle should still work.

---

## Self-Review Checklist

- [x] `category_context` moved from `users` to `user_settings` ✓
- [x] `show_goals bool`, `user_api_key text`, `show_limits_categories bool` added to `user_settings` ✓
- [x] `FindByID` LEFT JOINs `user_settings` so upload.go and chat.go read `.CategoryContext` without changes ✓
- [x] `UpdateSettings` handler is PATCH-safe (partial update, loads current values first) ✓
- [x] Migration adds columns to `user_settings`, backfills from `users`, drops from `users` ✓
- [x] Frontend `getMe` response type widened — no silent `any` ✓
- [x] `useSettings` hook backward-compatible (still exports `categoryContext`, `loading`, `updateContext`) ✓
- [x] `ProfileCategories.tsx` and `Profile.tsx` need no changes ✓
