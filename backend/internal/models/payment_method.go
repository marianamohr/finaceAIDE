package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentMethodKind string

const (
	KindCreditCard PaymentMethodKind = "credit_card"
	KindDebit      PaymentMethodKind = "debit"
	KindPIX        PaymentMethodKind = "pix"
	KindVoucher    PaymentMethodKind = "voucher"
	KindCash       PaymentMethodKind = "cash"
)

type PaymentMethod struct {
	ID          uuid.UUID         `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID   uuid.UUID         `gorm:"not null;index;type:uuid"                      json:"account_id"`
	Name        string            `gorm:"not null"                                      json:"name"`
	Kind        PaymentMethodKind `gorm:"not null"                                      json:"kind"`
	ClosingDay  *int              `gorm:"type:int"                                      json:"closing_day,omitempty"`
	DueDay      *int              `gorm:"type:int"                                      json:"due_day,omitempty"`
	CreditLimit    *float64          `gorm:"type:numeric(12,2)"                            json:"credit_limit,omitempty"`
	LastFourDigits *string           `gorm:"type:varchar(4)"                               json:"last_four_digits,omitempty"`
	Color          string            `gorm:"not null;default:''"                           json:"color"`
	Active      bool              `gorm:"not null;default:true"                         json:"active"`
	CreatedAt   time.Time         `                                                     json:"created_at"`
}

// ComputeBillingMonth returns the (year, month) of the invoice that captures a
// purchase made on txDate, given the card's closing_day.
//
// billing_month = month in which the invoice closes (not when it's due).
//
// Examples (closing day 26):
//   - purchase 19/May → closes 26/May → billing May
//   - purchase 27/May → closes 26/Jun → billing June
//
// Examples (closing day 1):
//   - purchase 19/May → closes 01/Jun → billing June
//   - purchase 02/Jun → closes 01/Jul → billing July
//
// We anchor to day=1 before AddDate to avoid Go's day-overflow behaviour
// (e.g. March 31 + 1 month = May 1 instead of April 1).
func ComputeBillingMonth(txDate time.Time, closingDay int) (year int, month time.Month) {
	anchor := time.Date(txDate.Year(), txDate.Month(), 1, 0, 0, 0, 0, time.UTC)

	if txDate.Day() <= closingDay {
		// Invoice closes this month.
		return anchor.Year(), anchor.Month()
	}

	// Invoice closes next month.
	next := anchor.AddDate(0, 1, 0)
	return next.Year(), next.Month()
}
