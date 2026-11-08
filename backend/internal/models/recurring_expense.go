package models

import (
	"time"

	"github.com/google/uuid"
)

// Source value "recurring" is used for transactions created from recurring expenses.
type RecurringExpense struct {
	ID              uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID       uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"`
	Name            string     `gorm:"type:text;not null"                             json:"name"`
	Amount          float64    `gorm:"type:numeric(12,2);not null"                    json:"amount"`
	CategoryID      *uuid.UUID `gorm:"type:uuid"                                      json:"category_id,omitempty"`
	PaymentMethodID *uuid.UUID `gorm:"type:uuid"                                      json:"payment_method_id,omitempty"`
	DueDay          *int       `gorm:"type:int"                                       json:"due_day,omitempty"`
	StartMonth      *int       `gorm:"type:int"                                       json:"start_month,omitempty"`
	StartYear       *int       `gorm:"type:int"                                       json:"start_year,omitempty"`
	EndMonth        *int       `gorm:"type:int"                                       json:"end_month,omitempty"`
	EndYear         *int       `gorm:"type:int"                                       json:"end_year,omitempty"`
	IsVariable      bool       `gorm:"not null;default:false"                         json:"is_variable"`
	Active          bool       `gorm:"not null;default:true"                          json:"active"`
	CreatedAt       time.Time  `                                                      json:"created_at"`
	UpdatedAt       time.Time  `                                                      json:"updated_at"`
}

func (RecurringExpense) TableName() string { return "recurring_expenses" }

type RecurringExpenseWithStatus struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Amount          float64    `json:"amount"`
	CategoryID      *uuid.UUID `json:"category_id,omitempty"`
	CategorySlug    string     `json:"category_slug,omitempty"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
	DueDay          *int       `json:"due_day,omitempty"`
	StartMonth      *int       `json:"start_month,omitempty"`
	StartYear       *int       `json:"start_year,omitempty"`
	EndMonth        *int       `json:"end_month,omitempty"`
	EndYear         *int       `json:"end_year,omitempty"`
	IsVariable      bool       `json:"is_variable"`
	Paid            bool       `json:"paid"`
	TransactionID   *uuid.UUID `json:"transaction_id,omitempty"`
	// PaidAmount is the absolute value actually paid (from the linked transaction),
	// set only when Paid is true. For variable bills it differs from Amount.
	PaidAmount *float64 `json:"paid_amount,omitempty"`
}
