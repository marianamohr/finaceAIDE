//go:build integration

package handlers_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.Transaction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func newHandlerTestAuthManager(t *testing.T) *auth.Manager {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	privDER := x509.MarshalPKCS1PrivateKey(key)
	pubDER, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER})
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	m, err := auth.NewManager(string(privPEM), string(pubPEM))
	if err != nil {
		t.Fatalf("new auth manager: %v", err)
	}
	return m
}

func newHandlerRouter(h *handlers.PaymentMethodHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/payment-methods", h.List)
	r.Post("/payment-methods", h.Create)
	r.Put("/payment-methods/{id}", h.Update)
	r.Delete("/payment-methods/{id}", h.Delete)
	return r
}

func newHandlerRouterWithAuth(h *handlers.PaymentMethodHandler, manager *auth.Manager) *chi.Mux {
	r := chi.NewRouter()
	r.Use(manager.Middleware)
	r.Get("/payment-methods", h.List)
	r.Post("/payment-methods", h.Create)
	r.Put("/payment-methods/{id}", h.Update)
	r.Delete("/payment-methods/{id}", h.Delete)
	return r
}

func authedRequest(method, target string, body any, accountID uuid.UUID) *http.Request {
	var buf *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(method, target, buf)
	r.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithValues(r.Context(), uuid.New(), accountID, "user", "active")
	return r.WithContext(ctx)
}

func createHandlerPM(t *testing.T, db *gorm.DB, accountID uuid.UUID) *models.PaymentMethod {
	t.Helper()
	m := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "Nubank Crédito",
		Kind:      models.KindCreditCard,
		Color:     "#820AD1",
		Active:    true,
	}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(m) })
	return m
}

func createHandlerTx(t *testing.T, db *gorm.DB, accountID uuid.UUID, pmID *uuid.UUID) *models.Transaction {
	t.Helper()
	tx := &models.Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          -100.00,
		Category:        "food",
		Date:            time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		Source:          "manual",
		Type:            "regular",
		Locale:          "BR",
		Currency:        "BRL",
		PaymentMethodID: pmID,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(tx) })
	return tx
}

// TestPaymentMethodHandler_List_RequiresAuth verifies that the endpoint
// returns 401 when no Authorization header is provided.
func TestPaymentMethodHandler_List_RequiresAuth(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	manager := newHandlerTestAuthManager(t)
	mux := newHandlerRouterWithAuth(h, manager)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/payment-methods", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestPaymentMethodHandler_List_AccountIsolation verifies that List returns
// only methods belonging to the authenticated account.
func TestPaymentMethodHandler_List_AccountIsolation(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	otherAccountID := uuid.New()

	createHandlerPM(t, db, accountID)
	createHandlerPM(t, db, otherAccountID)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/payment-methods", nil, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var methods []models.PaymentMethod
	if err := json.NewDecoder(w.Body).Decode(&methods); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(methods) != 1 {
		t.Errorf("expected 1 method, got %d", len(methods))
	}
	if methods[0].AccountID != accountID {
		t.Errorf("got method from wrong account: %v", methods[0].AccountID)
	}
}

// TestPaymentMethodHandler_Create_ValidCreditCard verifies that a valid
// credit_card POST returns 201 with the created object.
func TestPaymentMethodHandler_Create_ValidCreditCard(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	closingDay := 3
	body := map[string]any{
		"name":        "Nubank Crédito",
		"kind":        "credit_card",
		"closing_day": closingDay,
		"color":       "#820AD1",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/payment-methods", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var created models.PaymentMethod
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&created) })

	if created.ID == uuid.Nil {
		t.Error("expected non-nil id")
	}
	if created.ClosingDay == nil || *created.ClosingDay != closingDay {
		t.Errorf("closing_day: got %v, want %d", created.ClosingDay, closingDay)
	}
}

// TestPaymentMethodHandler_Create_ValidPIX verifies that a valid pix POST returns 201.
func TestPaymentMethodHandler_Create_ValidPIX(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	body := map[string]any{
		"name":  "PIX BB",
		"kind":  "pix",
		"color": "#0052CC",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/payment-methods", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var created models.PaymentMethod
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&created) })

	if created.Kind != models.KindPIX {
		t.Errorf("kind: got %q, want %q", created.Kind, models.KindPIX)
	}
}

// TestPaymentMethodHandler_Update_WrongAccount verifies that PUT with an ID
// from another account returns 404.
func TestPaymentMethodHandler_Update_WrongAccount(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	ownerAccountID := uuid.New()
	otherAccountID := uuid.New()

	m := createHandlerPM(t, db, ownerAccountID)

	body := map[string]any{
		"name":        "Updated Name",
		"kind":        "credit_card",
		"closing_day": 5,
		"color":       "#000",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/payment-methods/"+m.ID.String(), body, otherAccountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestPaymentMethodHandler_Delete_WithTransactions verifies that DELETE
// soft-deletes (active=false) when the method has associated transactions.
func TestPaymentMethodHandler_Delete_WithTransactions(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	m := createHandlerPM(t, db, accountID)
	createHandlerTx(t, db, accountID, &m.ID)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, "/payment-methods/"+m.ID.String(), nil, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var after models.PaymentMethod
	if err := db.First(&after, "id = ?", m.ID).Error; err != nil {
		t.Fatalf("fetch after soft delete: %v", err)
	}
	if after.Active {
		t.Error("expected active=false after soft delete")
	}
}

// TestPaymentMethodHandler_Delete_NoTransactions verifies that DELETE
// hard-deletes (removes the record) when no transactions exist.
func TestPaymentMethodHandler_Delete_NoTransactions(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	m := createHandlerPM(t, db, accountID)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, "/payment-methods/"+m.ID.String(), nil, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var after models.PaymentMethod
	err := db.First(&after, "id = ?", m.ID).Error
	if err == nil {
		t.Error("expected record to be removed, but it still exists")
	}
}

// TestPaymentMethodHandler_Update_SameKind verifies that Update succeeds when
// the kind does not change.
func TestPaymentMethodHandler_Update_SameKind(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	m := createHandlerPM(t, db, accountID)

	closingDay := 10
	body := map[string]any{
		"name":        "Updated Nubank",
		"kind":        "credit_card",
		"closing_day": closingDay,
		"color":       "#000",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/payment-methods/"+m.ID.String(), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var updated models.PaymentMethod
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.Name != "Updated Nubank" {
		t.Errorf("name: got %q, want %q", updated.Name, "Updated Nubank")
	}
}

// TestPaymentMethodHandler_Update_KindChangeNoTransactions verifies that
// changing the kind is allowed when no transactions exist.
func TestPaymentMethodHandler_Update_KindChangeNoTransactions(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	m := createHandlerPM(t, db, accountID) // created as credit_card

	body := map[string]any{
		"name":  "Nubank Débito",
		"kind":  "debit",
		"color": "#820AD1",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/payment-methods/"+m.ID.String(), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestPaymentMethodHandler_Update_KindChangeWithTransactions verifies that
// changing the kind is rejected when transactions are associated.
func TestPaymentMethodHandler_Update_KindChangeWithTransactions(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	m := createHandlerPM(t, db, accountID) // credit_card
	createHandlerTx(t, db, accountID, &m.ID)

	body := map[string]any{
		"name":  "Nubank Débito",
		"kind":  "debit",
		"color": "#820AD1",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/payment-methods/"+m.ID.String(), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestPaymentMethodHandler_Update_InvalidBody verifies that a malformed body returns 400.
func TestPaymentMethodHandler_Update_InvalidBody(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	accountID := uuid.New()
	m := createHandlerPM(t, db, accountID)

	r := httptest.NewRequest(http.MethodPut, "/payment-methods/"+m.ID.String(), strings.NewReader("{invalid"))
	r.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithValues(r.Context(), uuid.New(), accountID, "user", "active")
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestPaymentMethodHandler_Delete_WrongAccountWithTransactions verifies that
// DELETE returns 404 when the method has transactions but belongs to another account.
func TestPaymentMethodHandler_Delete_WrongAccountWithTransactions(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	ownerAccountID := uuid.New()
	otherAccountID := uuid.New()
	m := createHandlerPM(t, db, ownerAccountID)
	createHandlerTx(t, db, ownerAccountID, &m.ID)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, "/payment-methods/"+m.ID.String(), nil, otherAccountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestPaymentMethodHandler_Update_InvalidUUID verifies that an invalid URL param returns 400.
func TestPaymentMethodHandler_Update_InvalidUUID(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	body := map[string]any{"name": "Test", "kind": "pix", "color": ""}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPut, "/payment-methods/not-a-uuid", body, uuid.New())
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestPaymentMethodHandler_Delete_InvalidUUID verifies that an invalid URL param returns 400.
func TestPaymentMethodHandler_Delete_InvalidUUID(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, "/payment-methods/not-a-uuid", nil, uuid.New())
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestPaymentMethodHandler_Delete_WrongAccount verifies that DELETE with an ID
// from another account returns 404 (hard delete path).
func TestPaymentMethodHandler_Delete_WrongAccount(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	ownerAccountID := uuid.New()
	otherAccountID := uuid.New()
	m := createHandlerPM(t, db, ownerAccountID)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, "/payment-methods/"+m.ID.String(), nil, otherAccountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestPaymentMethodHandler_Create_Validation_NameTooLong complements the unit
// tests by verifying the 101-char name validation with a full router.
func TestPaymentMethodHandler_Create_Validation_NameTooLong(t *testing.T) {
	db := newHandlerTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	h := handlers.NewPaymentMethodHandler(repo, nil, nil)
	mux := newHandlerRouter(h)

	body := map[string]any{
		"name": strings.Repeat("x", 101),
		"kind": "pix",
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/payment-methods", body, uuid.New())
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
