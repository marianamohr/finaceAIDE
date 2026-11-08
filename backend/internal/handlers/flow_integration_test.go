//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
	"gorm.io/gorm"
)

// buildFlowMux creates a unified chi router with /payment-methods, /transactions,
// and /dashboard routes backed by a real test database.
func buildFlowMux(t *testing.T) (*chi.Mux, *gorm.DB, string) {
	t.Helper()
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.RecurringExpense{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	catSlug := seedTxTestCategory(t, db)

	txRepo := repository.NewTransactionRepository(db)
	recurringRepo := repository.NewRecurringExpenseRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)

	pmH := handlers.NewPaymentMethodHandler(pmRepo, nil, nil)
	txH := handlers.NewTransactionHandler(txRepo, catRepo, pmRepo, transactions.NewValidator(catRepo, pmRepo))
	dashH := handlers.NewDashboardHandler(txRepo, recurringRepo, catRepo, pmRepo)

	r := chi.NewRouter()
	r.Get("/payment-methods", pmH.List)
	r.Post("/payment-methods", pmH.Create)
	r.Put("/payment-methods/{id}", pmH.Update)
	r.Delete("/payment-methods/{id}", pmH.Delete)
	r.Post("/transactions", txH.Create)
	r.Get("/dashboard", dashH.Dashboard)

	return r, db, catSlug
}

// postPM sends POST /payment-methods and returns the decoded response.
func postPM(t *testing.T, mux *chi.Mux, accountID uuid.UUID, body map[string]any) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/payment-methods", body, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /payment-methods: expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("POST /payment-methods decode: %v", err)
	}
	return resp
}

// postTx sends POST /transactions and returns the decoded response.
func postTx(t *testing.T, mux *chi.Mux, accountID uuid.UUID, body map[string]any) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/transactions", body, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /transactions: expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("POST /transactions decode: %v", err)
	}
	return resp
}

// getDashboard sends GET /dashboard?year=Y&month=M and returns payment_method_summary.
func getDashboardPMSummary(t *testing.T, mux *chi.Mux, accountID uuid.UUID, year, month int) []any {
	t.Helper()
	w := httptest.NewRecorder()
	target := fmt.Sprintf("/dashboard?year=%d&month=%d", year, month)
	r := authedRequest(http.MethodGet, target, nil, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: expected 200, got %d; body: %s", target, w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("GET %s decode: %v", target, err)
	}
	items, _ := resp["payment_method_summary"].([]any)
	return items
}

// TestFlow_CreditCard_DayAfterClosing_AppearsInNextBillingMonth tests the full HTTP chain:
// POST /payment-methods (credit_card, closing_day=3) →
// POST /transactions (date=May 4, day after closing) →
// GET /dashboard?month=6 → transaction appears in payment_method_summary (billing_month=6)
// GET /dashboard?month=5 → transaction does NOT appear (billing_month routes to June, not May)
func TestFlow_CreditCard_DayAfterClosing_AppearsInNextBillingMonth(t *testing.T) {
	mux, _, catSlug := buildFlowMux(t)
	accountID := uuid.New()

	pmResp := postPM(t, mux, accountID, map[string]any{
		"name":        "Nubank Crédito",
		"kind":        "credit_card",
		"closing_day": 3,
		"color":       "#820AD1",
	})
	pmID := pmResp["id"].(string)

	txResp := postTx(t, mux, accountID, map[string]any{
		"amount":            -100.0,
		"category":          catSlug,
		"date":              "2026-05-04",
		"payment_method_id": pmID,
	})

	// Server must have computed billing_month=6 (day 4 > closing_day 3 → next month)
	if got := txResp["billing_month"]; got != float64(6) {
		t.Errorf("billing_month: got %v, want 6", got)
	}
	if got := txResp["billing_year"]; got != float64(2026) {
		t.Errorf("billing_year: got %v, want 2026", got)
	}

	// June dashboard: transaction must appear
	juneItems := getDashboardPMSummary(t, mux, accountID, 2026, 6)
	if len(juneItems) != 1 {
		t.Fatalf("dashboard month=6: expected 1 entry in payment_method_summary, got %d", len(juneItems))
	}
	entry, _ := juneItems[0].(map[string]any)
	if entry["name"] != "Nubank Crédito" {
		t.Errorf("name: got %v, want Nubank Crédito", entry["name"])
	}
	if entry["total"] != -100.0 {
		t.Errorf("total: got %v, want -100.0", entry["total"])
	}

	// May dashboard: transaction must NOT appear (billing_month=6, not 5)
	mayItems := getDashboardPMSummary(t, mux, accountID, 2026, 5)
	if len(mayItems) != 0 {
		t.Errorf("dashboard month=5: expected 0 entries (cc routed to June), got %d", len(mayItems))
	}
}

// TestFlow_CreditCard_DayAtClosing_AppearsInSameBillingMonth tests the inclusive boundary:
// POST /payment-methods (credit_card, closing_day=3) →
// POST /transactions (date=May 3, exactly at closing day) →
// GET /dashboard?month=5 → transaction appears (closing day is inclusive → billing_month=5)
func TestFlow_CreditCard_DayAtClosing_AppearsInSameBillingMonth(t *testing.T) {
	mux, _, catSlug := buildFlowMux(t)
	accountID := uuid.New()

	pmResp := postPM(t, mux, accountID, map[string]any{
		"name":        "Nubank Crédito",
		"kind":        "credit_card",
		"closing_day": 3,
		"color":       "#820AD1",
	})
	pmID := pmResp["id"].(string)

	txResp := postTx(t, mux, accountID, map[string]any{
		"amount":            -200.0,
		"category":          catSlug,
		"date":              "2026-05-03",
		"payment_method_id": pmID,
	})

	// Day 3 == closing_day 3 → billing_month=5 (same month)
	if got := txResp["billing_month"]; got != float64(5) {
		t.Errorf("billing_month: got %v, want 5", got)
	}
	if got := txResp["billing_year"]; got != float64(2026) {
		t.Errorf("billing_year: got %v, want 2026", got)
	}

	// May dashboard: transaction must appear
	mayItems := getDashboardPMSummary(t, mux, accountID, 2026, 5)
	if len(mayItems) != 1 {
		t.Fatalf("dashboard month=5: expected 1 entry for same-month billing, got %d", len(mayItems))
	}
	entry, _ := mayItems[0].(map[string]any)
	if entry["total"] != -200.0 {
		t.Errorf("total: got %v, want -200.0", entry["total"])
	}

	// June dashboard: transaction must NOT appear (billing_month=5)
	juneItems := getDashboardPMSummary(t, mux, accountID, 2026, 6)
	if len(juneItems) != 0 {
		t.Errorf("dashboard month=6: expected 0 entries (cc billing_month=5), got %d", len(juneItems))
	}
}

// TestFlow_Pix_AppearsInCalendarMonthDashboard tests that a PIX transaction
// appears in the dashboard for the calendar month of its date:
// POST /payment-methods (pix) →
// POST /transactions (date=May 15) →
// GET /dashboard?month=5 → appears in payment_method_summary
func TestFlow_Pix_AppearsInCalendarMonthDashboard(t *testing.T) {
	mux, _, catSlug := buildFlowMux(t)
	accountID := uuid.New()

	pmResp := postPM(t, mux, accountID, map[string]any{
		"name":  "PIX BB",
		"kind":  "pix",
		"color": "#0052CC",
	})
	pmID := pmResp["id"].(string)

	postTx(t, mux, accountID, map[string]any{
		"amount":            -50.0,
		"category":          catSlug,
		"date":              "2026-05-15",
		"payment_method_id": pmID,
	})

	// May dashboard: PIX transaction appears (matched by calendar month)
	mayItems := getDashboardPMSummary(t, mux, accountID, 2026, 5)
	if len(mayItems) != 1 {
		t.Fatalf("dashboard month=5: expected 1 PIX entry, got %d", len(mayItems))
	}
	entry, _ := mayItems[0].(map[string]any)
	if entry["name"] != "PIX BB" {
		t.Errorf("name: got %v, want PIX BB", entry["name"])
	}
	if entry["kind"] != "pix" {
		t.Errorf("kind: got %v, want pix", entry["kind"])
	}
	if entry["total"] != -50.0 {
		t.Errorf("total: got %v, want -50.0", entry["total"])
	}

	// June dashboard: PIX tx from May must NOT appear
	juneItems := getDashboardPMSummary(t, mux, accountID, 2026, 6)
	if len(juneItems) != 0 {
		t.Errorf("dashboard month=6: expected 0 entries for pix in May, got %d", len(juneItems))
	}
}

// TestFlow_SoftDelete_PreservesTransactionPaymentMethodID verifies the full lifecycle:
// 1. Create payment method
// 2. Create transaction linked to that method
// 3. DELETE /payment-methods/{id} → 200 (soft delete because transactions exist)
// 4. Verify payment method is inactive
// 5. Verify transaction still references the payment method (historical association preserved)
func TestFlow_SoftDelete_PreservesTransactionPaymentMethodID(t *testing.T) {
	mux, db, catSlug := buildFlowMux(t)
	accountID := uuid.New()

	pmResp := postPM(t, mux, accountID, map[string]any{
		"name":        "Nubank Crédito",
		"kind":        "credit_card",
		"closing_day": 5,
		"color":       "#820AD1",
	})
	pmID := pmResp["id"].(string)
	pmUUID, _ := uuid.Parse(pmID)

	txResp := postTx(t, mux, accountID, map[string]any{
		"amount":            -300.0,
		"category":          catSlug,
		"date":              "2026-05-10",
		"payment_method_id": pmID,
	})
	txID := txResp["id"].(string)
	txUUID, _ := uuid.Parse(txID)

	// DELETE should soft-delete (has transactions)
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, "/payment-methods/"+pmID, nil, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE /payment-methods: expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	// Verify payment method is inactive
	var pm models.PaymentMethod
	if err := db.First(&pm, "id = ?", pmUUID).Error; err != nil {
		t.Fatalf("fetch payment method after soft delete: %v", err)
	}
	if pm.Active {
		t.Error("expected payment method to be inactive (active=false) after soft delete")
	}

	// Verify transaction still has payment_method_id (historical association preserved)
	var tx models.Transaction
	if err := db.First(&tx, "id = ?", txUUID).Error; err != nil {
		t.Fatalf("fetch transaction after soft delete: %v", err)
	}
	if tx.PaymentMethodID == nil {
		t.Error("expected transaction to retain payment_method_id after soft delete, but it is nil")
	}
	if *tx.PaymentMethodID != pmUUID {
		t.Errorf("transaction payment_method_id: got %v, want %v", *tx.PaymentMethodID, pmUUID)
	}
}

// TestFlow_Transaction_RequiresAuth verifies that POST /transactions returns 401
// when no valid Authorization header is provided.
func TestFlow_Transaction_RequiresAuth(t *testing.T) {
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.Category{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	manager := newHandlerTestAuthManager(t)

	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	txH := handlers.NewTransactionHandler(txRepo, catRepo, pmRepo, transactions.NewValidator(catRepo, pmRepo))

	r := chi.NewRouter()
	r.Use(manager.Middleware)
	r.Post("/transactions", txH.Create)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/transactions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestFlow_AccountIsolation_TransactionWithOtherAccountPM verifies that creating a
// transaction using a payment_method_id from another account returns 400.
func TestFlow_AccountIsolation_TransactionWithOtherAccountPM(t *testing.T) {
	mux, _, catSlug := buildFlowMux(t)

	ownerAccountID := uuid.New()
	callerAccountID := uuid.New()

	// Create payment method under ownerAccountID
	pmResp := postPM(t, mux, ownerAccountID, map[string]any{
		"name":        "Cartão do Dono",
		"kind":        "credit_card",
		"closing_day": 10,
		"color":       "#000000",
	})
	pmID := pmResp["id"].(string)

	// Attempt to use that payment method from callerAccountID
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/transactions", map[string]any{
		"amount":            -100.0,
		"category":          catSlug,
		"date":              "2026-05-10",
		"payment_method_id": pmID,
	}, callerAccountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when using another account's payment method, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestFlow_AccountIsolation_DashboardShowsOnlyOwnMethods verifies that
// GET /dashboard returns only payment method data for the authenticated account.
func TestFlow_AccountIsolation_DashboardShowsOnlyOwnMethods(t *testing.T) {
	mux, _, catSlug := buildFlowMux(t)

	accountA := uuid.New()
	accountB := uuid.New()

	// Account A: credit_card with a May 4 transaction (billing_month=6)
	pmRespA := postPM(t, mux, accountA, map[string]any{
		"name":        "Cartão A",
		"kind":        "credit_card",
		"closing_day": 3,
		"color":       "#AAAAAA",
	})
	postTx(t, mux, accountA, map[string]any{
		"amount":            -500.0,
		"category":          catSlug,
		"date":              "2026-05-04",
		"payment_method_id": pmRespA["id"].(string),
	})

	// Account B: pix with a May transaction
	pmRespB := postPM(t, mux, accountB, map[string]any{
		"name":  "PIX B",
		"kind":  "pix",
		"color": "#BBBBBB",
	})
	postTx(t, mux, accountB, map[string]any{
		"amount":            -200.0,
		"category":          catSlug,
		"date":              "2026-05-10",
		"payment_method_id": pmRespB["id"].(string),
	})

	// Account A dashboard for June: must see only Account A's credit_card entry
	itemsA := getDashboardPMSummary(t, mux, accountA, 2026, 6)
	if len(itemsA) != 1 {
		t.Fatalf("account A dashboard June: expected 1 entry, got %d", len(itemsA))
	}
	entryA, _ := itemsA[0].(map[string]any)
	if entryA["name"] != "Cartão A" {
		t.Errorf("account A entry name: got %v, want Cartão A", entryA["name"])
	}

	// Account B dashboard for May: must see only Account B's PIX entry
	itemsB := getDashboardPMSummary(t, mux, accountB, 2026, 5)
	if len(itemsB) != 1 {
		t.Fatalf("account B dashboard May: expected 1 entry, got %d", len(itemsB))
	}
	entryB, _ := itemsB[0].(map[string]any)
	if entryB["name"] != "PIX B" {
		t.Errorf("account B entry name: got %v, want PIX B", entryB["name"])
	}
}
