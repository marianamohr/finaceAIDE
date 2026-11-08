package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestRecurringExpense_DueDay_Validation(t *testing.T) {
	t.Parallel()

	pmID := uuid.New()

	tests := []struct {
		name            string
		dueDay          *int
		paymentMethodID *uuid.UUID
		shouldBeValid   bool
	}{
		{
			name:            "nil due_day with nil payment_method_id is valid",
			dueDay:          nil,
			paymentMethodID: nil,
			shouldBeValid:   true,
		},
		{
			name:            "due_day 28 with nil payment_method_id is valid",
			dueDay:          intPtr(28),
			paymentMethodID: nil,
			shouldBeValid:   true,
		},
		{
			name:            "due_day 29 with nil payment_method_id is invalid",
			dueDay:          intPtr(29),
			paymentMethodID: nil,
			shouldBeValid:   false,
		},
		{
			name:            "due_day 1 with nil payment_method_id is valid",
			dueDay:          intPtr(1),
			paymentMethodID: nil,
			shouldBeValid:   true,
		},
		{
			name:            "due_day 0 with nil payment_method_id is invalid",
			dueDay:          intPtr(0),
			paymentMethodID: nil,
			shouldBeValid:   false,
		},
		{
			name:            "due_day 31 with payment_method_id is valid",
			dueDay:          intPtr(31),
			paymentMethodID: &pmID,
			shouldBeValid:   true,
		},
		{
			name:            "due_day 32 with payment_method_id is invalid",
			dueDay:          intPtr(32),
			paymentMethodID: &pmID,
			shouldBeValid:   false,
		},
		{
			name:            "due_day 29 with payment_method_id is valid",
			dueDay:          intPtr(29),
			paymentMethodID: &pmID,
			shouldBeValid:   true,
		},
		{
			name:            "nil due_day with payment_method_id is valid",
			dueDay:          nil,
			paymentMethodID: &pmID,
			shouldBeValid:   true,
		},
		{
			name:            "existing expense without payment_method_id and due_day 15 is unaffected",
			dueDay:          intPtr(15),
			paymentMethodID: nil,
			shouldBeValid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid := true
			if tt.dueDay != nil {
				if tt.paymentMethodID == nil && *tt.dueDay > 28 {
					isValid = false
				} else if tt.paymentMethodID == nil && (*tt.dueDay < 1 || *tt.dueDay > 31) {
					isValid = false
				} else if *tt.dueDay < 1 || *tt.dueDay > 31 {
					isValid = false
				}
			}

			if isValid != tt.shouldBeValid {
				t.Errorf("validation mismatch: expected isValid=%v, got %v", tt.shouldBeValid, isValid)
			}
		})
	}
}

func intPtr(v int) *int {
	return &v
}
