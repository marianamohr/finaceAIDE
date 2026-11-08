//go:build integration

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/gorm"
)

func newInstallmentRouter(h *handlers.InstallmentPurchaseHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/installment-purchases", h.List)
	r.Post("/installment-purchases", h.Create)
	r.Get("/installment-purchases/{id}", h.Get)
	r.Put("/installment-purchases/{id}", h.Update)
	r.Delete("/installment-purchases/{id}", h.Delete)
	return r
}

func buildInstallmentHandlerWithDB(t *testing.T) (*handlers.InstallmentPurchaseHandler, *gorm.DB) {
	t.Helper()
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.InstallmentPurchase{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate installment: %v", err)
	}
	repo := repository.NewInstallmentPurchaseRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	h := handlers.NewInstallmentPurchaseHandler(repo, pmRepo, catRepo)
	return h, db
}

func seedCreditCard(t *testing.T, db *gorm.DB, accountID uuid.UUID, closingDay int) *models.PaymentMethod {
	t.Helper()
	m := &models.PaymentMethod{
		AccountID:  accountID,
		Name:       "Nubank Crédito",
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
		Color:      "#820AD1",
		Active:     true,
	}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("seed credit card: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(m) })
	return m
}

func seedCategory(t *testing.T, db *gorm.DB, slug string) *models.Category {
	t.Helper()
	cat := &models.Category{Slug: slug, Label: slug, Type: "expense", Mode: "standard"}
	if err := db.Where("slug = ?", slug).FirstOrCreate(cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return cat
}

// TestInstallmentPurchaseHandler_EndToEnd exercises the full lifecycle through
// the HTTP layer against the real database: create → get (3 parcels with correct
// billing months and amounts) → list → update (regenerate to 2 parcels) → delete → 404.
func TestInstallmentPurchaseHandler_EndToEnd(t *testing.T) {
	h, db := buildInstallmentHandlerWithDB(t)
	mux := newInstallmentRouter(h)
	accountID := uuid.New()
	card := seedCreditCard(t, db, accountID, 20)
	seedCategory(t, db, "eletronicos")

	// Cleanup any parcels created (linked to the purchase) at the end.
	t.Cleanup(func() {
		db.Unscoped().Where("account_id = ?", accountID).Delete(&models.Transaction{})
		db.Unscoped().Where("account_id = ?", accountID).Delete(&models.InstallmentPurchase{})
	})

	// 1. Create: R$1000 em 3x, compra 10/mai (closing 20 → 1ª fatura em maio).
	createBody := map[string]any{
		"payment_method_id": card.ID,
		"category":          "eletronicos",
		"description":       "TV Samsung",
		"total_amount":      1000.0,
		"installments":      3,
		"date":              "2026-05-10",
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodPost, "/installment-purchases", createBody, accountID))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d — body: %s", w.Code, w.Body.String())
	}
	var created models.InstallmentPurchase
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("create: decode response: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("create: expected a non-nil purchase id")
	}

	// 2. Get: deve trazer 3 parcelas com meses 5,6,7 e valores -333.33,-333.33,-333.34.
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodGet, "/installment-purchases/"+created.ID.String(), nil, accountID))
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d — body: %s", w.Code, w.Body.String())
	}
	var detail struct {
		Purchase     models.InstallmentPurchase `json:"purchase"`
		Installments []models.Transaction       `json:"installments"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("get: decode response: %v", err)
	}
	if len(detail.Installments) != 3 {
		t.Fatalf("get: expected 3 installments, got %d", len(detail.Installments))
	}
	wantMonths := []int{5, 6, 7}
	wantAmounts := []float64{-333.33, -333.33, -333.34}
	var sum float64
	for i, tx := range detail.Installments {
		if tx.BillingMonth == nil || *tx.BillingMonth != wantMonths[i] {
			t.Errorf("installment %d: expected billing_month %d, got %v", i+1, wantMonths[i], tx.BillingMonth)
		}
		if tx.BillingYear == nil || *tx.BillingYear != 2026 {
			t.Errorf("installment %d: expected billing_year 2026, got %v", i+1, tx.BillingYear)
		}
		if d := tx.Amount - wantAmounts[i]; d > 0.001 || d < -0.001 {
			t.Errorf("installment %d: expected amount %.2f, got %.2f", i+1, wantAmounts[i], tx.Amount)
		}
		if tx.Source != "installment" {
			t.Errorf("installment %d: expected source 'installment', got %q", i+1, tx.Source)
		}
		if tx.InstallmentNumber == nil || *tx.InstallmentNumber != i+1 {
			t.Errorf("installment %d: expected installment_number %d, got %v", i+1, i+1, tx.InstallmentNumber)
		}
		sum += tx.Amount
	}
	if d := sum + 1000.0; d > 0.001 || d < -0.001 {
		t.Errorf("sum of installments should equal -1000.00, got %.2f", sum)
	}

	// 3. List: deve conter a compra.
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodGet, "/installment-purchases", nil, accountID))
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
	var list []models.InstallmentPurchase
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("list: decode: %v", err)
	}
	found := false
	for _, p := range list {
		if p.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("list: created purchase not found in list")
	}

	// 4. Update: regenera para 2x — deve passar a ter 2 parcelas.
	updateBody := map[string]any{
		"payment_method_id": card.ID,
		"category":          "eletronicos",
		"description":       "TV Samsung",
		"total_amount":      1000.0,
		"installments":      2,
		"date":              "2026-05-10",
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodPut, "/installment-purchases/"+created.ID.String(), updateBody, accountID))
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d — body: %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodGet, "/installment-purchases/"+created.ID.String(), nil, accountID))
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("get after update: decode: %v", err)
	}
	if len(detail.Installments) != 2 {
		t.Fatalf("update: expected 2 installments after regenerate, got %d", len(detail.Installments))
	}

	// 5. Delete: 200, depois GET → 404.
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodDelete, "/installment-purchases/"+created.ID.String(), nil, accountID))
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodGet, "/installment-purchases/"+created.ID.String(), nil, accountID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", w.Code)
	}
}

// TestInstallmentPurchaseHandler_RejectsNonCreditCard verifies the credit-card-only guard.
func TestInstallmentPurchaseHandler_RejectsNonCreditCard(t *testing.T) {
	h, db := buildInstallmentHandlerWithDB(t)
	mux := newInstallmentRouter(h)
	accountID := uuid.New()
	seedCategory(t, db, "eletronicos")

	debit := &models.PaymentMethod{AccountID: accountID, Name: "Débito", Kind: models.KindDebit, Active: true}
	if err := db.Create(debit).Error; err != nil {
		t.Fatalf("seed debit: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(debit) })

	body := map[string]any{
		"payment_method_id": debit.ID,
		"category":          "eletronicos",
		"description":       "TV",
		"total_amount":      1000.0,
		"installments":      3,
		"date":              "2026-05-10",
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodPost, "/installment-purchases", body, accountID))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-credit-card, got %d — body: %s", w.Code, w.Body.String())
	}
}
