//go:build integration

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
	"gorm.io/gorm"
)

func newRecurringExpenseRouter(h *handlers.RecurringExpenseHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/recurring-expenses", h.Create)
	r.Post("/recurring-expenses/{id}/pay", h.MarkPaid)
	return r
}

func buildRecurringHandlerWithDB(t *testing.T) (*handlers.RecurringExpenseHandler, *repository.PaymentMethodRepository, *gorm.DB) {
	t.Helper()
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.RecurringExpense{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate recurring: %v", err)
	}
	recurringRepo := repository.NewRecurringExpenseRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	validator := transactions.NewValidator(catRepo, pmRepo)
	h := handlers.NewRecurringExpenseHandler(recurringRepo, txRepo, catRepo, pmRepo, validator)
	return h, pmRepo, db
}

func createRecurringExpense(t *testing.T, db *gorm.DB, accountID uuid.UUID, pmID *uuid.UUID, dueDay *int) *models.RecurringExpense {
	t.Helper()
	cat := &models.Category{Slug: "alimentacao", Label: "alimentação", Type: "expense", Mode: "standard"}
	if err := db.Where("slug = ?", cat.Slug).FirstOrCreate(cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	expense := &models.RecurringExpense{
		AccountID:       accountID,
		Name:            "Netflix",
		Amount:          39.90,
		CategoryID:      &cat.ID,
		PaymentMethodID: pmID,
		DueDay:          dueDay,
		Active:          true,
	}
	if err := db.Create(expense).Error; err != nil {
		t.Fatalf("create recurring expense: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(expense) })
	return expense
}

func callMarkPaid(t *testing.T, mux *chi.Mux, expenseID uuid.UUID, accountID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	now := time.Now().UTC()
	body := map[string]any{
		"year":  now.Year(),
		"month": int(now.Month()),
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/recurring-expenses/"+expenseID.String()+"/pay", body, accountID)
	mux.ServeHTTP(w, r)
	return w
}

func callMarkPaidWithPM(t *testing.T, mux *chi.Mux, expenseID uuid.UUID, accountID uuid.UUID, pmID *uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	now := time.Now().UTC()
	body := map[string]any{
		"year":              now.Year(),
		"month":             int(now.Month()),
		"payment_method_id": pmID,
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/recurring-expenses/"+expenseID.String()+"/pay", body, accountID)
	mux.ServeHTTP(w, r)
	return w
}

// TestMarkPaid_CreditCard_BillingMonthComputed verifies that billing_year/billing_month
// are set on the transaction when the recurring expense uses a credit_card method.
func TestMarkPaid_CreditCard_BillingMonthComputed(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	// closing_day=31 guarantees billing_month == current month (any day <= 31).
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)

	dueDay := 15
	expense := createRecurringExpense(t, db, accountID, &pm.ID, &dueDay)

	w := callMarkPaid(t, mux, expense.ID, accountID)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	now := time.Now().UTC()
	expectedYear, expectedMonth := models.ComputeBillingMonth(now, closingDay)

	var tx models.Transaction
	if err := db.Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).First(&tx).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.PaymentMethodID == nil || *tx.PaymentMethodID != pm.ID {
		t.Errorf("payment_method_id: got %v, want %v", tx.PaymentMethodID, pm.ID)
	}
	if tx.BillingYear == nil || *tx.BillingYear != expectedYear {
		t.Errorf("billing_year: got %v, want %d", tx.BillingYear, expectedYear)
	}
	if tx.BillingMonth == nil || *tx.BillingMonth != int(expectedMonth) {
		t.Errorf("billing_month: got %v, want %d", tx.BillingMonth, expectedMonth)
	}
}

// TestMarkPaid_CreditCard_SmallClosingDay verifies billing rolls to next month
// when the card's closing_day is smaller than most days of the month.
func TestMarkPaid_CreditCard_SmallClosingDay(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	closingDay := 1
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)

	dueDay := 4
	expense := createRecurringExpense(t, db, accountID, &pm.ID, &dueDay)

	w := callMarkPaid(t, mux, expense.ID, accountID)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	now := time.Now().UTC()
	expectedYear, expectedMonth := models.ComputeBillingMonth(now, closingDay)

	var tx models.Transaction
	if err := db.Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).First(&tx).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.BillingYear == nil || *tx.BillingYear != expectedYear {
		t.Errorf("billing_year: got %v, want %d", tx.BillingYear, expectedYear)
	}
	if tx.BillingMonth == nil || *tx.BillingMonth != int(expectedMonth) {
		t.Errorf("billing_month: got %v, want %d", tx.BillingMonth, expectedMonth)
	}
}

// TestMarkPaid_NoBillingMonthWhenNoPaymentMethod verifies backward compatibility:
// expenses without payment_method_id create transactions without billing_year/billing_month.
func TestMarkPaid_NoBillingMonthWhenNoPaymentMethod(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	pm := &models.PaymentMethod{AccountID: accountID, Name: "Itaú Débito", Kind: models.KindDebit, Active: true}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}
	t.Cleanup(func() { _ = pmRepo.HardDelete(pm.ID, accountID) })
	expense := createRecurringExpense(t, db, accountID, nil, nil)

	w := callMarkPaidWithPM(t, mux, expense.ID, accountID, &pm.ID)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var tx models.Transaction
	if err := db.Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).First(&tx).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.PaymentMethodID == nil || *tx.PaymentMethodID != pm.ID {
		t.Errorf("payment_method_id: expected %v, got %v", pm.ID, tx.PaymentMethodID)
	}
	if tx.BillingYear != nil {
		t.Errorf("billing_year: expected nil, got %d", *tx.BillingYear)
	}
	if tx.BillingMonth != nil {
		t.Errorf("billing_month: expected nil, got %d", *tx.BillingMonth)
	}
}

// TestMarkPaid_NoBillingMonthForPIX verifies that a pix payment method does not
// produce billing_year/billing_month on the created transaction.
func TestMarkPaid_NoBillingMonthForPIX(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	pm := createPixPM(t, pmRepo, accountID)

	expense := createRecurringExpense(t, db, accountID, &pm.ID, nil)

	w := callMarkPaid(t, mux, expense.ID, accountID)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var tx models.Transaction
	if err := db.Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).First(&tx).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.PaymentMethodID == nil || *tx.PaymentMethodID != pm.ID {
		t.Errorf("payment_method_id: got %v, want %v", tx.PaymentMethodID, pm.ID)
	}
	if tx.BillingYear != nil {
		t.Errorf("billing_year: expected nil, got %d", *tx.BillingYear)
	}
	if tx.BillingMonth != nil {
		t.Errorf("billing_month: expected nil, got %d", *tx.BillingMonth)
	}
}

// TestMarkPaid_InvalidID verifies that MarkPaid returns 400 for a non-UUID path segment.
func TestMarkPaid_InvalidID(t *testing.T) {
	h, _, _ := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	now := time.Now().UTC()
	body := map[string]any{"year": now.Year(), "month": int(now.Month())}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/recurring-expenses/not-a-uuid/pay", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestMarkPaid_CreditCard_NoClosingDay verifies that a credit_card without
// closing_day does NOT produce billing_year/billing_month on the transaction.
func TestMarkPaid_CreditCard_NoClosingDay(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	pm := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "Cartão Sem Fechamento",
		Kind:      models.KindCreditCard,
		Color:     "#000000",
		Active:    true,
		// ClosingDay intentionally nil
	}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}

	expense := createRecurringExpense(t, db, accountID, &pm.ID, nil)

	w := callMarkPaid(t, mux, expense.ID, accountID)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var tx models.Transaction
	if err := db.Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).First(&tx).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.BillingYear != nil {
		t.Errorf("billing_year: expected nil (no closing_day), got %d", *tx.BillingYear)
	}
	if tx.BillingMonth != nil {
		t.Errorf("billing_month: expected nil (no closing_day), got %d", *tx.BillingMonth)
	}
}

// TestMarkPaid_InvalidBody verifies that MarkPaid returns 400 when year is missing.
func TestMarkPaid_InvalidBody(t *testing.T) {
	h, _, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	expense := createRecurringExpense(t, db, accountID, nil, nil)

	body := map[string]any{"month": 5} // year is 0
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/recurring-expenses/"+expense.ID.String()+"/pay", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestMarkPaid_MalformedJSON verifies that MarkPaid returns 400 for non-JSON bodies.
func TestMarkPaid_MalformedJSON(t *testing.T) {
	h, _, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	expense := createRecurringExpense(t, db, accountID, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/recurring-expenses/"+expense.ID.String()+"/pay",
		strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithValues(req.Context(), uuid.New(), accountID, "user", "active")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestMarkPaid_ExpenseNotFound verifies MarkPaid returns 404 for unknown expense ID.
func TestMarkPaid_ExpenseNotFound(t *testing.T) {
	h, _, _ := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	now := time.Now().UTC()
	body := map[string]any{"year": now.Year(), "month": int(now.Month())}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/recurring-expenses/"+uuid.New().String()+"/pay", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestMarkPaid_Idempotency verifies that calling MarkPaid twice returns the existing
// transaction on the second call (HTTP 200) rather than creating a duplicate.
func TestMarkPaid_Idempotency(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	pm := &models.PaymentMethod{AccountID: accountID, Name: "Itaú Débito", Kind: models.KindDebit, Active: true}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}
	t.Cleanup(func() { _ = pmRepo.HardDelete(pm.ID, accountID) })
	expense := createRecurringExpense(t, db, accountID, nil, nil)

	w1 := callMarkPaidWithPM(t, mux, expense.ID, accountID, &pm.ID)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first pay: expected 201, got %d; body: %s", w1.Code, w1.Body.String())
	}

	// Second call — must return 200 with the existing transaction.
	w2 := callMarkPaidWithPM(t, mux, expense.ID, accountID, &pm.ID)
	if w2.Code != http.StatusOK {
		t.Errorf("second pay: expected 200, got %d; body: %s", w2.Code, w2.Body.String())
	}

	// Only one transaction should exist.
	var count int64
	db.Model(&models.Transaction{}).Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 transaction, got %d", count)
	}
}

// TestMarkPaid_CreditCard_WithCategory verifies billing month is set even when
// the expense has a category, exercising the catRepo lookup branch.
func TestMarkPaid_CreditCard_WithCategory(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	// Ensure the category exists.
	catSlug := seedTxTestCategory(t, db)
	var cat models.Category
	if err := db.Where("slug = ?", catSlug).First(&cat).Error; err != nil {
		t.Fatalf("find category: %v", err)
	}

	accountID := uuid.New()
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)

	expense := &models.RecurringExpense{
		AccountID:       accountID,
		Name:            "Spotify",
		Amount:          21.90,
		PaymentMethodID: &pm.ID,
		CategoryID:      &cat.ID,
		Active:          true,
	}
	if err := db.Create(expense).Error; err != nil {
		t.Fatalf("create expense: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(expense) })

	w := callMarkPaid(t, mux, expense.ID, accountID)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	now := time.Now().UTC()
	expectedYear, expectedMonth := models.ComputeBillingMonth(now, closingDay)

	var tx models.Transaction
	if err := db.Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).First(&tx).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.BillingYear == nil || *tx.BillingYear != expectedYear {
		t.Errorf("billing_year: got %v, want %d", tx.BillingYear, expectedYear)
	}
	if tx.BillingMonth == nil || *tx.BillingMonth != int(expectedMonth) {
		t.Errorf("billing_month: got %v, want %d", tx.BillingMonth, expectedMonth)
	}
	if tx.Category != catSlug {
		t.Errorf("category: got %q, want %q", tx.Category, catSlug)
	}
}

func TestListByAccount_ReturnsExpiredSubscriptions(t *testing.T) {
	_, _, db := buildRecurringHandlerWithDB(t)
	accountID := uuid.New()
	recurringRepo := repository.NewRecurringExpenseRepository(db)

	endMonth := 3
	endYear := 2026
	expired := &models.RecurringExpense{
		AccountID: accountID,
		Name:      "Seguro cancelado",
		Amount:    120.00,
		EndMonth:  &endMonth,
		EndYear:   &endYear,
		Active:    true,
	}
	if err := db.Create(expired).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(expired) })

	items, err := recurringRepo.ListByAccount(accountID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == expired.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected expired subscription to appear in ListByAccount, but it didn't")
	}
}

// TestMarkPaid_WrongAccountPaymentMethod verifies that MarkPaid returns 400 and
// does not create a transaction when the expense references a PM from another account.
func TestMarkPaid_WrongAccountPaymentMethod(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	ownerAccountID := uuid.New()
	callerAccountID := uuid.New()

	closingDay := 10
	pm := &models.PaymentMethod{
		AccountID:  ownerAccountID,
		Name:       "Cartão Outro",
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
		Color:      "#000000",
		Active:     true,
	}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}

	// The expense belongs to callerAccountID but references a PM from ownerAccountID.
	expense := &models.RecurringExpense{
		AccountID:       callerAccountID,
		Name:            "Assinatura",
		Amount:          50.00,
		PaymentMethodID: &pm.ID,
		Active:          true,
	}
	if err := db.Create(expense).Error; err != nil {
		t.Fatalf("create expense: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(expense) })

	w := callMarkPaid(t, mux, expense.ID, callerAccountID)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}

	// No transaction should have been created.
	var count int64
	db.Model(&models.Transaction{}).Where("account_id = ?", callerAccountID).Count(&count)
	if count != 0 {
		t.Errorf("expected no transactions, got %d", count)
	}

	var errResp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] == "" {
		t.Error("expected error message in response body")
	}
}

// TestMarkPaid_AmountOverride verifies that an `amount` in the pay request is
// used for the created transaction instead of the registered estimate — the core
// of paying a variable bill (água/luz) with the real value of the month.
func TestMarkPaid_AmountOverride(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	pm := &models.PaymentMethod{AccountID: accountID, Name: "Itaú Débito", Kind: models.KindDebit, Active: true}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}
	t.Cleanup(func() { _ = pmRepo.HardDelete(pm.ID, accountID) })

	// Estimate registered as 39.90; the real bill came in at 180.50.
	expense := createRecurringExpense(t, db, accountID, nil, nil)

	now := time.Now().UTC()
	body := map[string]any{
		"year":              now.Year(),
		"month":             int(now.Month()),
		"payment_method_id": pm.ID,
		"amount":            180.50,
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/recurring-expenses/"+expense.ID.String()+"/pay", body, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var tx models.Transaction
	if err := db.Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).First(&tx).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.Amount != -180.50 {
		t.Errorf("amount: expected -180.50 (override), got %v", tx.Amount)
	}
}

// TestMarkPaid_VariableRequiresAmount verifies that paying an is_variable expense
// without an amount is rejected — a variable bill has no meaningful fixed value to
// charge automatically.
func TestMarkPaid_VariableRequiresAmount(t *testing.T) {
	h, pmRepo, db := buildRecurringHandlerWithDB(t)
	mux := newRecurringExpenseRouter(h)

	accountID := uuid.New()
	pm := &models.PaymentMethod{AccountID: accountID, Name: "Itaú Débito", Kind: models.KindDebit, Active: true}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}
	t.Cleanup(func() { _ = pmRepo.HardDelete(pm.ID, accountID) })

	cat := &models.Category{Slug: "alimentacao", Label: "alimentação", Type: "expense", Mode: "standard"}
	if err := db.Where("slug = ?", cat.Slug).FirstOrCreate(cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	expense := &models.RecurringExpense{
		AccountID:  accountID,
		Name:       "Conta de Luz",
		Amount:     150.00,
		CategoryID: &cat.ID,
		IsVariable: true,
		Active:     true,
	}
	if err := db.Create(expense).Error; err != nil {
		t.Fatalf("create expense: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(expense) })

	now := time.Now().UTC()
	body := map[string]any{
		"year":              now.Year(),
		"month":             int(now.Month()),
		"payment_method_id": pm.ID,
		// amount intentionally omitted
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/recurring-expenses/"+expense.ID.String()+"/pay", body, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (amount required for variable), got %d; body: %s", w.Code, w.Body.String())
	}

	var count int64
	db.Model(&models.Transaction{}).Where("account_id = ? AND recurring_expense_id = ?", accountID, expense.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected no transaction created, got %d", count)
	}
}
