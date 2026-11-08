package models

import (
	"time"

	"github.com/google/uuid"
)

// Source values: "manual" | "chat" | "photo" | "recurring" | "installment" | "balance_transfer" | "voucher_deposit"
type Transaction struct {
	ID          uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID  `gorm:"not null;index;type:uuid"                      json:"user_id"`    // audit: who created it
	AccountID   uuid.UUID  `gorm:"index;type:uuid"                               json:"account_id"` // shared account owner
	Amount      float64    `gorm:"not null"                                      json:"amount"`
	Category    string     `gorm:"not null"                                      json:"category"`
	CategoryID  *uuid.UUID `gorm:"index;type:uuid"                               json:"category_id,omitempty"`
	CategoryRef *Category  `gorm:"foreignKey:CategoryID;references:ID"           json:"-"`
	Merchant    string     `gorm:"default:''"                                    json:"merchant"`
	Description string     `                                                     json:"description"`
	Date        time.Time  `gorm:"not null"                                      json:"date"`
	Source      string     `gorm:"not null;default:'manual'"                     json:"source"`
	Type        string     `gorm:"not null;default:'regular'"                    json:"type"`
	ImageURL    string     `                                                     json:"image_url"`
	CreatedAt   time.Time  `                                                     json:"created_at"`

	RecurringExpenseID    *uuid.UUID `gorm:"type:uuid;index" json:"recurring_expense_id,omitempty"`
	InstallmentPurchaseID *uuid.UUID `gorm:"type:uuid;index" json:"installment_purchase_id,omitempty"`
	InstallmentNumber     *int       `                       json:"installment_number,omitempty"`

	PaymentMethodID        *uuid.UUID     `gorm:"type:uuid;index"                         json:"payment_method_id,omitempty"`
	PaymentMethod          *PaymentMethod `gorm:"foreignKey:PaymentMethodID;references:ID" json:"-"`
	VoucherPaymentMethodID *uuid.UUID     `gorm:"type:uuid;index" json:"voucher_payment_method_id,omitempty"`
	BillingYear            *int           `                                                json:"billing_year,omitempty"`
	BillingMonth           *int           `                                                json:"billing_month,omitempty"`
	CarryoverAmount        *float64       `gorm:"type:numeric(12,2)"                       json:"carryover_amount,omitempty"`

	// Multi-currency fields
	Locale          string     `gorm:"not null;default:'BR'"        json:"locale"`
	Currency        string     `gorm:"not null;default:'BRL'"       json:"currency"`
	OriginalAmount  float64    `gorm:"type:numeric(12,2);default:0" json:"original_amount"`
	ExchangeRate    *float64   `gorm:"type:numeric(10,6)"           json:"exchange_rate,omitempty"`
	ConvertedAmount *float64   `gorm:"type:numeric(12,2)"           json:"converted_amount,omitempty"`
	ConvertedAt     *time.Time `                                    json:"converted_at,omitempty"`
}
