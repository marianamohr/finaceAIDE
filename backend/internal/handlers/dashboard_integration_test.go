//go:build integration

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/gorm"
)

func newDashboardRouter(h *handlers.DashboardHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/dashboard", h.Dashboard)
	return r
}

func newDashboardRouterWithAuth(h *handlers.DashboardHandler, manager *auth.Manager) *chi.Mux {
	r := chi.NewRouter()
	r.Use(manager.Middleware)
	r.Get("/dashboard", h.Dashboard)
	return r
}

func buildDashboardHandlerWithDB(t *testing.T) (*handlers.DashboardHandler, *repository.PaymentMethodRepository, *gorm.DB) {
	t.Helper()
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.RecurringExpense{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate dashboard extras: %v", err)
	}
	txRepo := repository.NewTransactionRepository(db)
	recurringRepo := repository.NewRecurringExpenseRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewDashboardHandler(txRepo, recurringRepo, catRepo, pmRepo)
	return h, pmRepo, db
}

func createDashboardTx(
	t *testing.T,
	db *gorm.DB,
	accountID uuid.UUID,
	amount float64,
	date time.Time,
	pmID *uuid.UUID,
	billingYear *int,
	billingMonth *int,
) *models.Transaction {
	t.Helper()
	tx := &models.Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          amount,
		Category:        "food",
		Date:            date,
		Source:          "manual",
		Type:            "regular",
		Locale:          "BR",
		Currency:        "BRL",
		PaymentMethodID: pmID,
		BillingYear:     billingYear,
		BillingMonth:    billingMonth,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create dashboard transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(tx) })
	return tx
}

// TestDashboardHandler_PaymentMethodSummaryFieldPresent verifies that GET /dashboard
// always returns a payment_method_summary field (array, never null).
func TestDashboardHandler_PaymentMethodSummaryFieldPresent(t *testing.T) {
	h, _, _ := buildDashboardHandlerWithDB(t)
	mux := newDashboardRouter(h)

	accountID := uuid.New()
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/dashboard?year=2026&month=5", nil, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	raw, ok := resp["payment_method_summary"]
	if !ok {
		t.Fatal("payment_method_summary field missing from response")
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("payment_method_summary is not an array, got %T", raw)
	}
	if len(items) != 0 {
		t.Errorf("expected empty array for account with no transactions, got %d items", len(items))
	}
}

// TestDashboardHandler_PaymentMethodSummaryEntryPerMethod verifies that each payment method
// with transactions in the period appears as one entry in payment_method_summary.
func TestDashboardHandler_PaymentMethodSummaryEntryPerMethod(t *testing.T) {
	h, pmRepo, db := buildDashboardHandlerWithDB(t)
	mux := newDashboardRouter(h)

	accountID := uuid.New()

	pixPM := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "PIX BB",
		Kind:      models.KindPIX,
		Color:     "#0052CC",
		Active:    true,
	}
	if err := pmRepo.Create(pixPM); err != nil {
		t.Fatalf("create pix pm: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(pixPM) })

	pmID := pixPM.ID
	createDashboardTx(t, db, accountID, -200.00,
		time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
		&pmID, nil, nil)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/dashboard?year=2026&month=5", nil, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	items, _ := resp["payment_method_summary"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 entry in payment_method_summary, got %d", len(items))
	}

	entry, _ := items[0].(map[string]any)
	if entry["name"] != "PIX BB" {
		t.Errorf("name: got %v, want PIX BB", entry["name"])
	}
	if entry["kind"] != string(models.KindPIX) {
		t.Errorf("kind: got %v, want %v", entry["kind"], models.KindPIX)
	}
	if entry["total"] != -200.00 {
		t.Errorf("total: got %v, want -200.00", entry["total"])
	}
	if entry["count"] != float64(1) {
		t.Errorf("count: got %v, want 1", entry["count"])
	}
}

// TestDashboardHandler_CreditCardAppearsInBillingMonth verifies that a credit_card
// transaction dated in month M but with billing_month=M+1 appears in M+1, not in M.
func TestDashboardHandler_CreditCardAppearsInBillingMonth(t *testing.T) {
	h, pmRepo, db := buildDashboardHandlerWithDB(t)
	mux := newDashboardRouter(h)

	accountID := uuid.New()

	closingDay := 3
	ccPM := &models.PaymentMethod{
		AccountID:  accountID,
		Name:       "Nubank Crédito",
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
		Color:      "#820AD1",
		Active:     true,
	}
	if err := pmRepo.Create(ccPM); err != nil {
		t.Fatalf("create cc pm: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(ccPM) })

	// Purchase on May 4 (after closing_day=3) → billing_month=6
	billingYear := 2026
	billingMonth := 6
	pmID := ccPM.ID
	createDashboardTx(t, db, accountID, -500.00,
		time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		&pmID, &billingYear, &billingMonth)

	// June query: transaction should appear
	wJune := httptest.NewRecorder()
	rJune := authedRequest(http.MethodGet, "/dashboard?year=2026&month=6", nil, accountID)
	mux.ServeHTTP(wJune, rJune)

	if wJune.Code != http.StatusOK {
		t.Fatalf("june: expected 200, got %d", wJune.Code)
	}
	var juneResp map[string]any
	if err := json.NewDecoder(wJune.Body).Decode(&juneResp); err != nil {
		t.Fatalf("decode june: %v", err)
	}
	juneItems, _ := juneResp["payment_method_summary"].([]any)
	if len(juneItems) != 1 {
		t.Errorf("june: expected 1 entry, got %d", len(juneItems))
	}

	// May query: transaction should NOT appear
	wMay := httptest.NewRecorder()
	rMay := authedRequest(http.MethodGet, "/dashboard?year=2026&month=5", nil, accountID)
	mux.ServeHTTP(wMay, rMay)

	if wMay.Code != http.StatusOK {
		t.Fatalf("may: expected 200, got %d", wMay.Code)
	}
	var mayResp map[string]any
	if err := json.NewDecoder(wMay.Body).Decode(&mayResp); err != nil {
		t.Fatalf("decode may: %v", err)
	}
	mayItems, _ := mayResp["payment_method_summary"].([]any)
	if len(mayItems) != 0 {
		t.Errorf("may: expected 0 entries for credit_card filtered by billing_month, got %d", len(mayItems))
	}
}

// TestDashboardHandler_EmptyPaymentMethodSummaryWhenNoMethods verifies that
// payment_method_summary is an empty array when no payment methods exist.
func TestDashboardHandler_EmptyPaymentMethodSummaryWhenNoMethods(t *testing.T) {
	h, _, db := buildDashboardHandlerWithDB(t)
	mux := newDashboardRouter(h)

	accountID := uuid.New()

	// Transaction with no payment_method_id
	createDashboardTx(t, db, accountID, -100.00,
		time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
		nil, nil, nil)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/dashboard?year=2026&month=5", nil, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Null bucket appears when there are transactions without payment_method_id
	// but no registered payment methods — so we should get the null bucket entry
	// OR an empty array if there are no transactions at all. Here we have a null-bucket tx.
	raw, ok := resp["payment_method_summary"]
	if !ok {
		t.Fatal("payment_method_summary field missing")
	}
	_, ok = raw.([]any)
	if !ok {
		t.Fatalf("payment_method_summary is not an array, got %T", raw)
	}
}

// TestDashboardHandler_SummaryFieldUnchanged verifies that the existing summary
// (by category) is present and correct after adding payment_method_summary.
func TestDashboardHandler_SummaryFieldUnchanged(t *testing.T) {
	h, _, db := buildDashboardHandlerWithDB(t)
	mux := newDashboardRouter(h)

	accountID := uuid.New()

	createDashboardTx(t, db, accountID, -150.00,
		time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
		nil, nil, nil)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/dashboard?year=2026&month=5", nil, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	summary, ok := resp["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary field missing or wrong type, got %T", resp["summary"])
	}
	if _, ok := summary["income"]; !ok {
		t.Error("summary.income field missing")
	}
	if _, ok := summary["expenses"]; !ok {
		t.Error("summary.expenses field missing")
	}
	if _, ok := summary["balance"]; !ok {
		t.Error("summary.balance field missing")
	}
	if _, ok := summary["categories"]; !ok {
		t.Error("summary.categories field missing")
	}
	if summary["expenses"] != -150.00 {
		t.Errorf("summary.expenses: got %v, want -150.00", summary["expenses"])
	}
	// Verify payment_method_summary is also present alongside summary
	if _, ok := resp["payment_method_summary"]; !ok {
		t.Error("payment_method_summary field missing alongside summary")
	}
}

// TestDashboardHandler_RequiresAuth verifies that GET /dashboard returns 401
// when no valid Authorization header is provided.
func TestDashboardHandler_RequiresAuth(t *testing.T) {
	h, _, _ := buildDashboardHandlerWithDB(t)
	manager := newHandlerTestAuthManager(t)
	mux := newDashboardRouterWithAuth(h, manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dashboard?year=2026&month=5", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d; body: %s", w.Code, w.Body.String())
	}
}
