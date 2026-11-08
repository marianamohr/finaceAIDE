package handlers

import (
	"testing"
	"time"
)

// TestResolveStart verifies that a recurring expense without an explicit start
// month defaults to the current month, so it never backfills retroactive
// transactions into past (already-closed) invoices.
func TestResolveStart(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	year2025, month3 := 2025, 3

	tests := []struct {
		name          string
		startYear     *int
		startMonth    *int
		expectedYear  int
		expectedMonth int
	}{
		{
			name:          "início explícito é preservado",
			startYear:     &year2025,
			startMonth:    &month3,
			expectedYear:  2025,
			expectedMonth: 3,
		},
		{
			name:          "início ausente usa o mês atual",
			startYear:     nil,
			startMonth:    nil,
			expectedYear:  2026,
			expectedMonth: 6,
		},
		{
			name:          "apenas o ano ausente usa o mês atual",
			startYear:     nil,
			startMonth:    &month3,
			expectedYear:  2026,
			expectedMonth: 6,
		},
		{
			name:          "apenas o mês ausente usa o mês atual",
			startYear:     &year2025,
			startMonth:    nil,
			expectedYear:  2026,
			expectedMonth: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotYear, gotMonth := resolveStart(tt.startYear, tt.startMonth, now)
			if gotYear == nil || gotMonth == nil {
				t.Fatalf("resolveStart() retornou nil: year=%v, month=%v", gotYear, gotMonth)
			}
			if *gotYear != tt.expectedYear || *gotMonth != tt.expectedMonth {
				t.Errorf("resolveStart() = (%d, %d), want (%d, %d)",
					*gotYear, *gotMonth, tt.expectedYear, tt.expectedMonth)
			}
		})
	}
}
