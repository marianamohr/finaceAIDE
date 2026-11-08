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
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
	"gorm.io/gorm"
)

func newTxHandlerRouter(h *handlers.TransactionHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/transactions", h.Create)
	r.Put("/transactions/{id}", h.Update)
	r.Delete("/transactions/{id}", h.Delete)
	r.Get("/transactions/{id}", h.Get)
	return r
}

// seedTxTestCategory inserts a minimal expense category into db and returns its slug.
// Uses ON CONFLICT DO NOTHING so it is safe to call multiple times.
func seedTxTestCategory(t *testing.T, db *gorm.DB) string {
	t.Helper()
	slug := "alimentacao"
	cat := &models.Category{
		Slug:  slug,
		Label: "alimentação",
		Type:  "expense",
		Mode:  "standard",
	}
	if err := db.
		Where("slug = ?", slug).
		FirstOrCreate(cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return slug
}

func buildTxHandlerWithDB(t *testing.T) (*handlers.TransactionHandler, *repository.PaymentMethodRepository, *gorm.DB, string) {
	t.Helper()
	db := newHandlerTestDB(t)
	// Ensure Category table exists for the handler's category lookup.
	if err := db.AutoMigrate(&models.Category{}); err != nil {
		t.Fatalf("automigrate category: %v", err)
	}
	catSlug := seedTxTestCategory(t, db)
	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	validator := transactions.NewValidator(catRepo, pmRepo)
	h := handlers.NewTransactionHandler(txRepo, catRepo, pmRepo, validator)
	return h, pmRepo, db, catSlug
}

func createCreditCardPM(t *testing.T, repo *repository.PaymentMethodRepository, accountID uuid.UUID, closingDay int) *models.PaymentMethod {
	t.Helper()
	m := &models.PaymentMethod{
		AccountID:  accountID,
		Name:       "Nubank Crédito",
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
		Color:      "#820AD1",
		Active:     true,
	}
	if err := repo.Create(m); err != nil {
		t.Fatalf("create credit card pm: %v", err)
	}
	return m
}

func createPixPM(t *testing.T, repo *repository.PaymentMethodRepository, accountID uuid.UUID) *models.PaymentMethod {
	t.Helper()
	m := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "PIX BB",
		Kind:      models.KindPIX,
		Color:     "#0052CC",
		Active:    true,
	}
	if err := repo.Create(m); err != nil {
		t.Fatalf("create pix pm: %v", err)
	}
	return m
}

// TestTransactionHandler_Create_CreditCardBillingMonthNextMonth verifies that
// a purchase on day 4 with closing_day=3 maps to the next billing month.
func TestTransactionHandler_Create_CreditCardBillingMonthNextMonth(t *testing.T) {
	h, pmRepo, _, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	accountID := uuid.New()
	pm := createCreditCardPM(t, pmRepo, accountID, 3)

	// Day 4 of month 5 → closing_day=3, day > closing_day → billing_month=6 (next month)
	body := map[string]any{
		"amount":            -100.00,
		"category":          catSlug,
		"date":              "2026-05-04",
		"payment_method_id": pm.ID,
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/transactions", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := resp["billing_month"]; got != float64(6) {
		t.Errorf("billing_month: got %v, want 6", got)
	}
	if got := resp["billing_year"]; got != float64(2026) {
		t.Errorf("billing_year: got %v, want 2026", got)
	}
}

// TestTransactionHandler_Create_CreditCardBillingMonthSameMonth verifies that
// a purchase on the closing day maps to the current billing month (inclusive).
func TestTransactionHandler_Create_CreditCardBillingMonthSameMonth(t *testing.T) {
	h, pmRepo, _, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	accountID := uuid.New()
	pm := createCreditCardPM(t, pmRepo, accountID, 3)

	// Day 3 of month 5 → closing_day=3, day == closing_day → billing_month=5 (same month)
	body := map[string]any{
		"amount":            -100.00,
		"category":          catSlug,
		"date":              "2026-05-03",
		"payment_method_id": pm.ID,
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/transactions", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := resp["billing_month"]; got != float64(5) {
		t.Errorf("billing_month: got %v, want 5", got)
	}
	if got := resp["billing_year"]; got != float64(2026) {
		t.Errorf("billing_year: got %v, want 2026", got)
	}
}

// TestTransactionHandler_Create_PixNoBillingMonth verifies that a PIX transaction
// has nil billing_year and billing_month.
func TestTransactionHandler_Create_PixNoBillingMonth(t *testing.T) {
	h, pmRepo, _, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	accountID := uuid.New()
	pm := createPixPM(t, pmRepo, accountID)

	body := map[string]any{
		"amount":            -50.00,
		"category":          catSlug,
		"date":              "2026-05-10",
		"payment_method_id": pm.ID,
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/transactions", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, present := resp["billing_month"]; present {
		t.Errorf("billing_month: expected absent (nil), but present: %v", resp["billing_month"])
	}
	if _, present := resp["billing_year"]; present {
		t.Errorf("billing_year: expected absent (nil), but present: %v", resp["billing_year"])
	}
}

// TestTransactionHandler_Create_NoPaymentMethod verifies that an expense without
// payment_method_id and no active PMs in the account returns 422 needs_clarification.
// The validator requires a payment method for expenses — it cannot silently omit one.
func TestTransactionHandler_Create_NoPaymentMethod(t *testing.T) {
	h, _, _, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	// Fresh account with zero PMs registered.
	accountID := uuid.New()

	body := map[string]any{
		"amount":   -75.00,
		"category": catSlug,
		"date":     "2026-05-10",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/transactions", body, accountID)
	mux.ServeHTTP(w, r)

	// Account has no active PMs → validator cannot resolve PM → 422 with error message.
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestTransactionHandler_Create_WrongAccountPaymentMethod verifies that a
// payment_method_id from another account returns 422 (validator hard error).
func TestTransactionHandler_Create_WrongAccountPaymentMethod(t *testing.T) {
	h, pmRepo, _, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	ownerAccountID := uuid.New()
	otherAccountID := uuid.New()

	pm := createCreditCardPM(t, pmRepo, ownerAccountID, 3)

	body := map[string]any{
		"amount":            -100.00,
		"category":          catSlug,
		"date":              "2026-05-10",
		"payment_method_id": pm.ID,
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/transactions", body, otherAccountID)
	mux.ServeHTTP(w, r)

	// The validator returns a hard error when the PM is not found for the account → 422.
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestTransactionHandler_Update_CreditCardToPixClearsBillingMonth verifies that
// changing payment method from credit_card to pix clears billing_year/billing_month.
func TestTransactionHandler_Update_CreditCardToPixClearsBillingMonth(t *testing.T) {
	h, pmRepo, db, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	accountID := uuid.New()
	ccPM := createCreditCardPM(t, pmRepo, accountID, 3)
	pixPM := createPixPM(t, pmRepo, accountID)

	billingYear := 2026
	billingMonth := 6
	tx := &models.Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          -100.00,
		Category:        catSlug,
		Date:            time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Source:          "manual",
		Type:            "regular",
		Locale:          "BR",
		Currency:        "BRL",
		PaymentMethodID: &ccPM.ID,
		BillingYear:     &billingYear,
		BillingMonth:    &billingMonth,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create tx: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(tx) })

	body := map[string]any{
		"amount":            -100.00,
		"category":          catSlug,
		"date":              "2026-05-04",
		"payment_method_id": pixPM.ID,
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/transactions/"+tx.ID.String(), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, present := resp["billing_month"]; present {
		t.Errorf("billing_month: expected absent after changing to pix, got: %v", resp["billing_month"])
	}
	if _, present := resp["billing_year"]; present {
		t.Errorf("billing_year: expected absent after changing to pix, got: %v", resp["billing_year"])
	}
}

// TestTransactionHandler_Update_NullPaymentMethodClearsBillingMonth verifies that
// setting payment_method_id=null on an existing credit_card transaction clears billing month.
func TestTransactionHandler_Update_NullPaymentMethodClearsBillingMonth(t *testing.T) {
	h, pmRepo, db, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	accountID := uuid.New()
	ccPM := createCreditCardPM(t, pmRepo, accountID, 3)

	billingYear := 2026
	billingMonth := 6
	tx := &models.Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          -100.00,
		Category:        catSlug,
		Date:            time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Source:          "manual",
		Type:            "regular",
		Locale:          "BR",
		Currency:        "BRL",
		PaymentMethodID: &ccPM.ID,
		BillingYear:     &billingYear,
		BillingMonth:    &billingMonth,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create tx: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(tx) })

	body := map[string]any{
		"amount":            -100.00,
		"category":          catSlug,
		"date":              "2026-05-04",
		"payment_method_id": nil,
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/transactions/"+tx.ID.String(), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, present := resp["billing_month"]; present {
		t.Errorf("billing_month: expected absent after null payment method, got: %v", resp["billing_month"])
	}
	if _, present := resp["billing_year"]; present {
		t.Errorf("billing_year: expected absent after null payment method, got: %v", resp["billing_year"])
	}
}

// TestTransactionHandler_Update_PreservesSystemFields verifies that editing a
// transaction (e.g. correcting its amount) does NOT wipe system-managed fields
// the edit form never sends. The reported bug: editing the amount of a bill
// payment dropped recurring_expense_id, so the gasto fixo reminder reappeared as
// pending. type and image_url were silently lost the same way.
func TestTransactionHandler_Update_PreservesSystemFields(t *testing.T) {
	h, _, db, catSlug := buildTxHandlerWithDB(t)
	mux := newTxHandlerRouter(h)

	accountID := uuid.New()
	recID := uuid.New()
	imageURL := "https://example.com/nota.jpg"
	tx := &models.Transaction{
		AccountID:          accountID,
		UserID:             uuid.New(),
		Amount:             -150.00,
		Category:           catSlug,
		Date:               time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		Source:             "recurring",
		Type:               "regular",
		Locale:             "BR",
		Currency:           "BRL",
		ImageURL:           imageURL,
		RecurringExpenseID: &recID,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create tx: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(tx) })

	// The real bill came in higher than the registered estimate — edit only the amount.
	body := map[string]any{
		"amount":   -187.45,
		"category": catSlug,
		"date":     "2026-06-04",
		"source":   "recurring",
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/transactions/"+tx.ID.String(), body, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var reloaded models.Transaction
	if err := db.First(&reloaded, "id = ?", tx.ID).Error; err != nil {
		t.Fatalf("reload tx: %v", err)
	}

	if reloaded.RecurringExpenseID == nil || *reloaded.RecurringExpenseID != recID {
		t.Errorf("recurring_expense_id: expected %v preserved, got %v", recID, reloaded.RecurringExpenseID)
	}
	if reloaded.Type != "regular" {
		t.Errorf("type: expected 'regular' preserved, got %q", reloaded.Type)
	}
	if reloaded.ImageURL != imageURL {
		t.Errorf("image_url: expected %q preserved, got %q", imageURL, reloaded.ImageURL)
	}
	if reloaded.Amount != -187.45 {
		t.Errorf("amount: expected -187.45 (updated), got %v", reloaded.Amount)
	}
}
