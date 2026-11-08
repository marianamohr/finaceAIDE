package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/handlers"
)

// injectAuth injects a test account ID into the request context,
// bypassing the JWT middleware.
func injectAuth(r *http.Request, accountID uuid.UUID) *http.Request {
	ctx := auth.ContextWithValues(r.Context(), uuid.New(), accountID, "user", "active")
	return r.WithContext(ctx)
}

func newCreateReq(t *testing.T, body map[string]any, accountID uuid.UUID) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/payment-methods", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	return injectAuth(r, accountID)
}

// TestPaymentMethodHandler_Create_Validation tests all 400 validation cases.
// Uses a nil repo because validation happens before any repository call.
func TestPaymentMethodHandler_Create_Validation(t *testing.T) {
	h := handlers.NewPaymentMethodHandler(nil, nil, nil)
	accountID := uuid.New()

	tests := []struct {
		name string
		body map[string]any
	}{
		{
			name: "credit_card without closing_day",
			body: map[string]any{"name": "Nubank", "kind": "credit_card"},
		},
		{
			name: "closing_day zero",
			body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 0},
		},
		{
			name: "closing_day 32",
			body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 32},
		},
		{
			name: "pix with closing_day",
			body: map[string]any{"name": "PIX BB", "kind": "pix", "closing_day": 5},
		},
		{
			name: "name of 101 chars",
			body: map[string]any{"name": strings.Repeat("x", 101), "kind": "pix"},
		},
		{
			name: "last_four_digits com 3 digitos",
			body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 10, "last_four_digits": "123"},
		},
		{
			name: "last_four_digits com letra",
			body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 10, "last_four_digits": "12ab"},
		},
		{
			name: "last_four_digits com 5 digitos",
			body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 10, "last_four_digits": "12345"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := newCreateReq(t, tt.body, accountID)
			h.Create(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("got %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
	}
}

// TestPaymentMethodHandler_Create_EmptyName verifies that an empty name returns 400.
func TestPaymentMethodHandler_Create_EmptyName(t *testing.T) {
	h := handlers.NewPaymentMethodHandler(nil, nil, nil)
	accountID := uuid.New()

	w := httptest.NewRecorder()
	r := newCreateReq(t, map[string]any{"name": "", "kind": "pix"}, accountID)
	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestPaymentMethodHandler_Create_InvalidBody verifies that a malformed body returns 400.
func TestPaymentMethodHandler_Create_InvalidBody(t *testing.T) {
	h := handlers.NewPaymentMethodHandler(nil, nil, nil)
	accountID := uuid.New()

	r := httptest.NewRequest(http.MethodPost, "/payment-methods", strings.NewReader("{invalid json"))
	r.Header.Set("Content-Type", "application/json")
	r = injectAuth(r, accountID)
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
