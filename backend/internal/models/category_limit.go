package models

import (
	"time"

	"github.com/google/uuid"
)

type CategoryLimit struct {
	ID           uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID    uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"`
	CategoryID   uuid.UUID  `gorm:"not null;type:uuid"                             json:"category_id"`
	GroupID      *uuid.UUID `gorm:"type:uuid;index"                                json:"group_id"`
	MonthlyLimit float64    `gorm:"type:numeric(12,2);not null"                    json:"monthly_limit"`
	CreatedAt    time.Time  `                                                      json:"created_at"`
	UpdatedAt    time.Time  `                                                      json:"updated_at"`
}

func (CategoryLimit) TableName() string { return "account_category_limits" }

type LimitWithProgress struct {
	ID           uuid.UUID `json:"id"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategorySlug string    `json:"category_slug"`
	MonthlyLimit float64   `json:"monthly_limit"`
	Spent        float64   `json:"spent"`
	Pct          float64   `json:"pct"`
}
