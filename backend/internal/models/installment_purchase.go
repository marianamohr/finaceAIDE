package models

import (
	"time"

	"github.com/google/uuid"
)

// InstallmentPurchase é a "compra pai" de uma compra avulsa parcelada no cartão
// de crédito. Cada parcela é uma Transaction com source="installment" apontando
// para esta compra via InstallmentPurchaseID.
type InstallmentPurchase struct {
	ID              uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID       uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"`
	PaymentMethodID uuid.UUID  `gorm:"not null;type:uuid"                             json:"payment_method_id"` // deve ser credit_card
	CategoryID      *uuid.UUID `gorm:"type:uuid"                                      json:"category_id,omitempty"`
	Description     string     `gorm:"type:text;not null"                            json:"description"`
	Merchant        string     `gorm:"type:text;default:''"                          json:"merchant"`
	TotalAmount     float64    `gorm:"type:numeric(12,2);not null"                   json:"total_amount"` // positivo
	Installments    int        `gorm:"not null"                                      json:"installments"` // N >= 2
	PurchaseDate    time.Time  `gorm:"not null"                                      json:"purchase_date"`
	CreatedAt       time.Time  `                                                     json:"created_at"`
	UpdatedAt       time.Time  `                                                     json:"updated_at"`
}

func (InstallmentPurchase) TableName() string { return "installment_purchases" }
