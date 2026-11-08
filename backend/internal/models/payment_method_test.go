package models

import (
	"testing"
	"time"
)

func TestComputeBillingMonth(t *testing.T) {
	t.Parallel()

	// billing_month = month in which the invoice closes (not when it's due).
	//
	// Nubank (closes 26): purchase 19/May → closes 26/May → billing May
	// Cartão Bia (closes 1): purchase 19/May → closes 01/Jun → billing June
	tests := []struct {
		name          string
		txDate        time.Time
		closingDay    int
		expectedYear  int
		expectedMonth time.Month
	}{
		{
			// Day 3 <= closing 3 → closes March → billing March.
			name:          "purchase on exact closing day: billing is current month",
			txDate:        time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC),
			closingDay:    3,
			expectedYear:  2026,
			expectedMonth: time.March,
		},
		{
			// Day 4 > closing 3 → closes April → billing April.
			name:          "purchase day after closing day: billing is next month",
			txDate:        time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC),
			closingDay:    3,
			expectedYear:  2026,
			expectedMonth: time.April,
		},
		{
			// Feb 28 <= closing 30 → closes February → billing February.
			name:          "february with closing_day=30: billing is february",
			txDate:        time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
			closingDay:    30,
			expectedYear:  2026,
			expectedMonth: time.February,
		},
		{
			// Dec 31 > closing 30 → closes January next year → billing January.
			name:          "dec 31 with closing_day=30: billing is january next year",
			txDate:        time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
			closingDay:    30,
			expectedYear:  2026,
			expectedMonth: time.January,
		},
		{
			// May 1 <= closing 15 → closes May → billing May.
			name:          "first day of month with closing_day greater than 1: billing is current month",
			txDate:        time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
			closingDay:    15,
			expectedYear:  2026,
			expectedMonth: time.May,
		},
		{
			// Mar 31 <= closing 31 → closes March → billing March.
			name:          "last day of month with closing_day equal to last day: billing is current month",
			txDate:        time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
			closingDay:    31,
			expectedYear:  2026,
			expectedMonth: time.March,
		},
		{
			// Nubank: day 19 <= closing 26 → closes May → billing May.
			name:          "nubank: purchase before closing day returns may",
			txDate:        time.Date(2026, time.May, 19, 0, 0, 0, 0, time.UTC),
			closingDay:    26,
			expectedYear:  2026,
			expectedMonth: time.May,
		},
		{
			// Nubank: day 27 > closing 26 → closes June → billing June.
			name:          "nubank: purchase after closing day returns june",
			txDate:        time.Date(2026, time.May, 27, 0, 0, 0, 0, time.UTC),
			closingDay:    26,
			expectedYear:  2026,
			expectedMonth: time.June,
		},
		{
			// Cartão Bia: day 19 > closing 1 → closes June → billing June.
			name:          "card bia: purchase after closing day 1 returns june",
			txDate:        time.Date(2026, time.May, 19, 0, 0, 0, 0, time.UTC),
			closingDay:    1,
			expectedYear:  2026,
			expectedMonth: time.June,
		},
		{
			// Cartão Bia: day 1 <= closing 1 → closes June → billing June.
			name:          "card bia: purchase on closing day 1 of june returns june",
			txDate:        time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			closingDay:    1,
			expectedYear:  2026,
			expectedMonth: time.June,
		},
		{
			// Cartão Bia: day 2 > closing 1 → closes July → billing July.
			name:          "card bia: purchase after closing day 1 in june returns july",
			txDate:        time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC),
			closingDay:    1,
			expectedYear:  2026,
			expectedMonth: time.July,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotYear, gotMonth := ComputeBillingMonth(tt.txDate, tt.closingDay)

			if gotYear != tt.expectedYear {
				t.Errorf("year: got %d, want %d", gotYear, tt.expectedYear)
			}

			if gotMonth != tt.expectedMonth {
				t.Errorf("month: got %v, want %v", gotMonth, tt.expectedMonth)
			}
		})
	}
}
