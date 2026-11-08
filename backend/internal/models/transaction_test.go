package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestTransaction_JSONFieldsPresent(t *testing.T) {
	t.Parallel()

	year := 2026
	month := 5
	carryover := 150.50
	pmID := uuid.New()

	tx := Transaction{
		ID:              uuid.New(),
		PaymentMethodID: &pmID,
		BillingYear:     &year,
		BillingMonth:    &month,
		CarryoverAmount: &carryover,
	}

	b, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"payment_method_id", "billing_year", "billing_month", "carryover_amount"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON missing field %q", key)
		}
	}
}

func TestTransaction_JSONOmitsNilOptionalFields(t *testing.T) {
	t.Parallel()

	tx := Transaction{
		ID: uuid.New(),
	}

	b, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"payment_method_id", "billing_year", "billing_month", "carryover_amount"} {
		if _, ok := m[key]; ok {
			t.Errorf("JSON should omit nil field %q", key)
		}
	}
}

func TestTransaction_JSONNoLegacyPaymentModeID(t *testing.T) {
	t.Parallel()

	tx := Transaction{ID: uuid.New()}

	b, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := m["payment_mode_id"]; ok {
		t.Error("JSON must not contain legacy field payment_mode_id")
	}
}
